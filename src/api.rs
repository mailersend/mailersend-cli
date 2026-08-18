use std::collections::BTreeMap;
use std::fmt;
use std::io::Read;
use std::time::Duration;

use anyhow::{Context, Result};
use serde_json::Value;

pub const DEFAULT_BASE_URL: &str = "https://api.mailersend.com/v1";
const MAX_RETRIES: u32 = 3;
const PER_PAGE: u64 = 25;
// The MailerSend API requires limit >= 10.
const MIN_PER_PAGE: u64 = 10;
#[cfg(test)]
pub static ENV_LOCK: std::sync::Mutex<()> = std::sync::Mutex::new(());

/// An API error with full field-level validation details.
#[derive(Debug, Default)]
pub struct ApiError {
    pub status: u16,
    pub message: String,
    pub errors: BTreeMap<String, Vec<String>>,
    pub raw_body: Option<Value>,
}

impl fmt::Display for ApiError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "API error {}: {}", self.status, self.message)?;
        for (field, msgs) in &self.errors {
            for msg in msgs {
                write!(f, "\n  {field}: {msg}")?;
            }
        }
        Ok(())
    }
}

impl std::error::Error for ApiError {}

impl ApiError {
    fn from_body(status: u16, body: &[u8]) -> Self {
        let mut err = ApiError {
            status,
            ..ApiError::default()
        };
        if !body.is_empty() {
            if let Ok(parsed) = serde_json::from_slice::<Value>(body) {
                if let Some(msg) = parsed.get("message").and_then(Value::as_str) {
                    err.message = msg.to_string();
                }
                if let Some(fields) = parsed.get("errors").and_then(Value::as_object) {
                    for (field, msgs) in fields {
                        let msgs: Vec<String> = msgs
                            .as_array()
                            .map(|a| {
                                a.iter()
                                    .filter_map(Value::as_str)
                                    .map(str::to_string)
                                    .collect()
                            })
                            .unwrap_or_default();
                        if !msgs.is_empty() {
                            err.errors.insert(field.clone(), msgs);
                        }
                    }
                }
                err.raw_body = Some(parsed);
            }
        }
        if err.message.is_empty() {
            err.message = format!("HTTP {status}");
        }
        err
    }
}

/// A successful API response with headers and parsed JSON body.
pub struct ApiResponse {
    pub body: Value,
    headers: Vec<(String, String)>,
}

impl ApiResponse {
    /// Case-insensitive response header lookup.
    pub fn header(&self, name: &str) -> Option<&str> {
        self.headers
            .iter()
            .find(|(k, _)| k.eq_ignore_ascii_case(name))
            .map(|(_, v)| v.as_str())
    }
}

/// MailerSend API client with retry, rate-limit handling, verbose logging,
/// and base URL override.
#[derive(Clone)]
pub struct Client {
    agent: ureq::Agent,
    token: String,
    base_url: String,
    verbose: bool,
    user_agent: String,
}

impl Client {
    pub fn new(token: String, verbose: bool) -> Self {
        let base_url = std::env::var("MAILERSEND_API_BASE_URL")
            .ok()
            .filter(|s| !s.is_empty())
            .unwrap_or_else(|| DEFAULT_BASE_URL.to_string());
        Client {
            agent: ureq::AgentBuilder::new()
                .timeout(Duration::from_secs(30))
                .build(),
            token,
            base_url,
            verbose,
            user_agent: format!("mailersend-cli/{}", crate::cli::VERSION),
        }
    }

    /// Performs a request and returns status, headers, and body. `path`
    /// starts with `/` (e.g. `/domains`). Body is `Value::Null` for 204/202
    /// empty responses. API errors surface as [`ApiError`] via `anyhow`.
    pub fn request_full(
        &self,
        method: &str,
        path: &str,
        query: &[(&str, String)],
        body: Option<&Value>,
    ) -> Result<ApiResponse> {
        let url = format!("{}{}", self.base_url, path);
        let body_bytes = body
            .map(serde_json::to_vec)
            .transpose()
            .context("failed to marshal request body")?;

        if self.verbose {
            let qs = if query.is_empty() {
                String::new()
            } else {
                let mut s = String::from("?");
                for (i, (k, v)) in query.iter().enumerate() {
                    if i > 0 {
                        s.push('&');
                    }
                    s.push_str(k);
                    s.push('=');
                    s.push_str(v);
                }
                s
            };
            println!("--> {method} {url}{qs}");
            if let Some(b) = &body_bytes {
                println!("--> body: {}", String::from_utf8_lossy(b));
            }
        }

        let mut last_transport_err: Option<ureq::Transport> = None;

        for attempt in 0..=MAX_RETRIES {
            let mut req = self
                .agent
                .request(method, &url)
                .set("Authorization", &format!("Bearer {}", self.token))
                .set("User-Agent", &self.user_agent)
                .set("Content-Type", "application/json")
                .set("Accept", "application/json");
            for (k, v) in query {
                req = req.query(k, v);
            }

            let result = match &body_bytes {
                Some(b) => req.send_bytes(b),
                None => req.call(),
            };

            let (status, resp) = match result {
                Ok(resp) => (resp.status(), resp),
                Err(ureq::Error::Status(code, resp)) => (code, resp),
                Err(ureq::Error::Transport(t)) => {
                    if self.verbose {
                        println!("<-- error: {t}");
                    }
                    last_transport_err = Some(t);
                    if attempt < MAX_RETRIES {
                        std::thread::sleep(Duration::from_secs(1 << attempt));
                        continue;
                    }
                    break;
                }
            };

            if self.verbose {
                println!("<-- {status} {}", resp.status_text());
            }

            let retry_after = resp
                .header("Retry-After")
                .and_then(|v| v.parse::<u64>().ok());
            let headers: Vec<(String, String)> = resp
                .headers_names()
                .into_iter()
                .filter_map(|name| resp.header(&name).map(|v| (name.clone(), v.to_string())))
                .collect();
            let mut resp_body = Vec::new();
            resp.into_reader()
                .take(10 << 20)
                .read_to_end(&mut resp_body)
                .context("failed to read response body")?;

            if self.verbose && !resp_body.is_empty() {
                println!("<-- body: {}", String::from_utf8_lossy(&resp_body));
            }

            if status >= 400 {
                if (status == 429 || status >= 500) && attempt < MAX_RETRIES {
                    let wait = retry_after.unwrap_or(1 << attempt);
                    if self.verbose {
                        println!("    retrying in {wait}s...");
                    }
                    std::thread::sleep(Duration::from_secs(wait));
                    continue;
                }
                return Err(ApiError::from_body(status, &resp_body).into());
            }

            let body = if resp_body.is_empty() {
                Value::Null
            } else {
                serde_json::from_slice(&resp_body).context("failed to decode response")?
            };
            return Ok(ApiResponse { body, headers });
        }

        Err(anyhow::anyhow!(
            "request failed after {MAX_RETRIES} retries: {}",
            last_transport_err.expect("transport error recorded")
        ))
    }

    /// Like [`request_full`](Self::request_full) but returns only the body.
    pub fn request(
        &self,
        method: &str,
        path: &str,
        query: &[(&str, String)],
        body: Option<&Value>,
    ) -> Result<Value> {
        Ok(self.request_full(method, path, query, body)?.body)
    }

    pub fn get(&self, path: &str) -> Result<Value> {
        self.request("GET", path, &[], None)
    }

    pub fn get_with(&self, path: &str, query: &[(&str, String)]) -> Result<Value> {
        self.request("GET", path, query, None)
    }

    pub fn post(&self, path: &str, body: &Value) -> Result<Value> {
        self.request("POST", path, &[], Some(body))
    }

    pub fn put(&self, path: &str, body: &Value) -> Result<Value> {
        self.request("PUT", path, &[], Some(body))
    }

    pub fn delete(&self, path: &str) -> Result<Value> {
        self.request("DELETE", path, &[], None)
    }
}

/// Fetches all pages of a page-numbered endpoint (`?page=N&limit=M`),
/// following `links.next`. `limit == 0` fetches everything. The MailerSend
/// API enforces a minimum page size of 10.
pub fn fetch_all_paged(
    client: &Client,
    path: &str,
    query: &[(&str, String)],
    limit: u64,
) -> Result<Vec<Value>> {
    let mut per = if limit > 0 && limit < PER_PAGE {
        limit
    } else {
        PER_PAGE
    };
    if per < MIN_PER_PAGE {
        per = MIN_PER_PAGE;
    }

    let mut all = Vec::new();
    let mut page: u64 = 1;
    loop {
        let mut q: Vec<(&str, String)> = query.to_vec();
        q.push(("page", page.to_string()));
        q.push(("limit", per.to_string()));
        let body = client.get_with(path, &q)?;
        let items: Vec<Value> = body
            .get("data")
            .and_then(Value::as_array)
            .cloned()
            .unwrap_or_default();
        let got = items.len() as u64;
        for item in items {
            all.push(item);
            if limit > 0 && all.len() as u64 >= limit {
                return Ok(all);
            }
        }
        let has_next = body
            .get("links")
            .and_then(|l| l.get("next"))
            .and_then(Value::as_str)
            .is_some_and(|s| !s.is_empty());
        if !has_next || got == 0 {
            return Ok(all);
        }
        page += 1;
    }
}
