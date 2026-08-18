use anyhow::{bail, Result};
use clap::{Args, Subcommand};
use serde_json::{json, Value};

use crate::api::{self, Client};
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::{jpath, jstr, resolve_domain_id};

#[derive(Args)]
#[command(
    long_about = "List, create, update, verify, and delete domains in your MailerSend account."
)]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// List domains
    List {
        /// maximum number of domains to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by verified status
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        verified: Option<bool>,
    },
    /// Get domain details
    #[command(arg_required_else_help = true)]
    Get {
        /// domain ID or name
        domain_id_or_name: String,
    },
    /// Add a new domain
    Add {
        /// domain name (required)
        #[arg(long, default_value = "")]
        name: String,
        /// custom return path subdomain
        #[arg(long, default_value = "")]
        return_path_subdomain: String,
        /// custom tracking subdomain
        #[arg(long, default_value = "")]
        custom_tracking_subdomain: String,
    },
    /// Delete a domain
    #[command(arg_required_else_help = true)]
    Delete {
        /// domain ID or name
        domain_id_or_name: String,
    },
    /// Update domain settings
    #[command(arg_required_else_help = true)]
    UpdateSettings {
        /// domain ID or name
        domain_id_or_name: String,
        /// pause sending
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        send_paused: Option<bool>,
        /// track clicks
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        track_clicks: Option<bool>,
        /// track opens
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        track_opens: Option<bool>,
        /// track unsubscribes
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        track_unsubscribe: Option<bool>,
        /// track content
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        track_content: Option<bool>,
        /// enable custom tracking
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        custom_tracking_enabled: Option<bool>,
        /// custom tracking subdomain
        #[arg(long)]
        custom_tracking_subdomain: Option<String>,
        /// set precedence bulk header
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        precedence_bulk: Option<bool>,
        /// ignore duplicated recipients
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        ignore_duplicated_recipients: Option<bool>,
    },
    /// Show DNS records for a domain
    #[command(arg_required_else_help = true)]
    Dns {
        /// domain ID or name
        domain_id_or_name: String,
    },
    /// Verify a domain
    #[command(arg_required_else_help = true)]
    Verify {
        /// domain ID or name
        domain_id_or_name: String,
    },
}

fn jbool(v: &Value, key: &str) -> bool {
    v.get(key).and_then(Value::as_bool).unwrap_or(false)
}

fn yes_no(b: bool) -> String {
    if b { "Yes" } else { "No" }.to_string()
}

fn check(b: bool) -> String {
    if b { "\u{2713}" } else { "\u{2717}" }.to_string()
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    let client = ctx.client()?;
    match cmd.command {
        Sub::List { limit, verified } => list(ctx, &client, limit, verified),
        Sub::Get { domain_id_or_name } => get(ctx, &client, &domain_id_or_name),
        Sub::Add {
            name,
            return_path_subdomain,
            custom_tracking_subdomain,
        } => add(
            ctx,
            &client,
            &name,
            &return_path_subdomain,
            &custom_tracking_subdomain,
        ),
        Sub::Delete { domain_id_or_name } => delete(ctx, &client, &domain_id_or_name),
        Sub::UpdateSettings {
            domain_id_or_name,
            send_paused,
            track_clicks,
            track_opens,
            track_unsubscribe,
            track_content,
            custom_tracking_enabled,
            custom_tracking_subdomain,
            precedence_bulk,
            ignore_duplicated_recipients,
        } => update_settings(
            ctx,
            &client,
            &domain_id_or_name,
            send_paused,
            track_clicks,
            track_opens,
            track_unsubscribe,
            track_content,
            custom_tracking_enabled,
            custom_tracking_subdomain,
            precedence_bulk,
            ignore_duplicated_recipients,
        ),
        Sub::Dns { domain_id_or_name } => dns(ctx, &client, &domain_id_or_name),
        Sub::Verify { domain_id_or_name } => verify(ctx, &client, &domain_id_or_name),
    }
}

fn list(ctx: &Ctx, client: &Client, limit: u64, verified: Option<bool>) -> Result<()> {
    let mut query: Vec<(&str, String)> = Vec::new();
    if let Some(v) = verified {
        query.push(("verified", v.to_string()));
    }

    let domains = api::fetch_all_paged(client, "/domains", &query, limit)?;

    if ctx.json {
        return output::json(&domains);
    }

    let rows: Vec<Vec<String>> = domains
        .iter()
        .map(|d| {
            vec![
                jstr(d, "id"),
                jstr(d, "name"),
                yes_no(jbool(d, "is_verified")),
                yes_no(jbool(d, "is_dns_active")),
                jstr(d, "created_at"),
            ]
        })
        .collect();

    output::table(&["ID", "NAME", "VERIFIED", "DNS ACTIVE", "CREATED"], &rows);
    Ok(())
}

fn get(ctx: &Ctx, client: &Client, id_or_name: &str) -> Result<()> {
    let domain_id = resolve_domain_id(client, id_or_name)?;
    let body = client.get(&format!("/domains/{domain_id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let rows = vec![
        vec!["ID".to_string(), jstr(&d, "id")],
        vec!["Name".to_string(), jstr(&d, "name")],
        vec!["Verified".to_string(), yes_no(jbool(&d, "is_verified"))],
        vec!["SPF".to_string(), yes_no(jbool(&d, "spf"))],
        vec!["DKIM".to_string(), yes_no(jbool(&d, "dkim"))],
        vec!["Tracking".to_string(), yes_no(jbool(&d, "tracking"))],
        vec!["DNS Active".to_string(), yes_no(jbool(&d, "is_dns_active"))],
        vec!["Created".to_string(), jstr(&d, "created_at")],
        vec!["Updated".to_string(), jstr(&d, "updated_at")],
    ];

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn add(
    ctx: &Ctx,
    client: &Client,
    name: &str,
    return_path_subdomain: &str,
    custom_tracking_subdomain: &str,
) -> Result<()> {
    let name = prompt::require_arg(name, "name", "Domain name")?;

    let mut body = json!({ "name": name });
    if !return_path_subdomain.is_empty() {
        body["return_path_subdomain"] = json!(return_path_subdomain);
    }
    if !custom_tracking_subdomain.is_empty() {
        body["custom_tracking_subdomain"] = json!(custom_tracking_subdomain);
    }

    let result = client.post("/domains", &body)?;

    if ctx.json {
        return output::json(&result);
    }

    let d = result.get("data").cloned().unwrap_or(Value::Null);
    output::success(&format!(
        "Domain created successfully: {} (ID: {})",
        jstr(&d, "name"),
        jstr(&d, "id")
    ));
    Ok(())
}

fn delete(_ctx: &Ctx, client: &Client, id_or_name: &str) -> Result<()> {
    let domain_id = resolve_domain_id(client, id_or_name)?;
    client.delete(&format!("/domains/{domain_id}"))?;
    output::success(&format!("Domain {id_or_name} deleted successfully."));
    Ok(())
}

#[allow(clippy::too_many_arguments)]
fn update_settings(
    ctx: &Ctx,
    client: &Client,
    id_or_name: &str,
    send_paused: Option<bool>,
    track_clicks: Option<bool>,
    track_opens: Option<bool>,
    track_unsubscribe: Option<bool>,
    track_content: Option<bool>,
    custom_tracking_enabled: Option<bool>,
    custom_tracking_subdomain: Option<String>,
    precedence_bulk: Option<bool>,
    ignore_duplicated_recipients: Option<bool>,
) -> Result<()> {
    let domain_id = resolve_domain_id(client, id_or_name)?;

    let mut body = json!({});
    let mut changed = false;

    if let Some(v) = send_paused {
        body["send_paused"] = json!(v);
        changed = true;
    }
    if let Some(v) = track_clicks {
        body["track_clicks"] = json!(v);
        changed = true;
    }
    if let Some(v) = track_opens {
        body["track_opens"] = json!(v);
        changed = true;
    }
    if let Some(v) = track_unsubscribe {
        body["track_unsubscribe"] = json!(v);
        changed = true;
    }
    if let Some(v) = track_content {
        body["track_content"] = json!(v);
        changed = true;
    }
    if let Some(v) = custom_tracking_enabled {
        body["custom_tracking_enabled"] = json!(v);
        changed = true;
    }
    if let Some(v) = custom_tracking_subdomain {
        if !v.is_empty() {
            body["custom_tracking_subdomain"] = json!(v);
        }
        changed = true;
    }
    if let Some(v) = precedence_bulk {
        body["precedence_bulk"] = json!(v);
        changed = true;
    }
    if let Some(v) = ignore_duplicated_recipients {
        body["ignore_duplicated_recipients"] = json!(v);
        changed = true;
    }

    if !changed {
        bail!("no settings flags provided; use --help to see available options");
    }

    let result = client.put(&format!("/domains/{domain_id}/settings"), &body)?;

    if ctx.json {
        return output::json(&result);
    }

    let d = result.get("data").cloned().unwrap_or(Value::Null);
    output::success(&format!(
        "Domain settings updated for {} (ID: {}).",
        jstr(&d, "name"),
        jstr(&d, "id")
    ));
    Ok(())
}

fn dns(ctx: &Ctx, client: &Client, id_or_name: &str) -> Result<()> {
    let domain_id = resolve_domain_id(client, id_or_name)?;
    let body = client.get(&format!("/domains/{domain_id}/dns-records"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let rows = vec![
        vec![
            "SPF".to_string(),
            jpath(&d, "spf.hostname"),
            jpath(&d, "spf.type"),
            jpath(&d, "spf.value"),
        ],
        vec![
            "DKIM".to_string(),
            jpath(&d, "dkim.hostname"),
            jpath(&d, "dkim.type"),
            jpath(&d, "dkim.value"),
        ],
        vec![
            "Return Path".to_string(),
            jpath(&d, "return_path.hostname"),
            jpath(&d, "return_path.type"),
            jpath(&d, "return_path.value"),
        ],
        vec![
            "Custom Tracking".to_string(),
            jpath(&d, "custom_tracking.hostname"),
            jpath(&d, "custom_tracking.type"),
            jpath(&d, "custom_tracking.value"),
        ],
    ];

    output::table(&["RECORD", "HOSTNAME", "TYPE", "VALUE"], &rows);
    Ok(())
}

fn verify(ctx: &Ctx, client: &Client, id_or_name: &str) -> Result<()> {
    let domain_id = resolve_domain_id(client, id_or_name)?;
    let body = client.get(&format!("/domains/{domain_id}/verify"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let rows = vec![
        vec!["DKIM".to_string(), check(jbool(&d, "dkim"))],
        vec!["SPF".to_string(), check(jbool(&d, "spf"))],
        vec!["MX".to_string(), check(jbool(&d, "mx"))],
        vec!["Tracking".to_string(), check(jbool(&d, "tracking"))],
        vec!["CNAME".to_string(), check(jbool(&d, "cname"))],
        vec![
            "Return Path CNAME".to_string(),
            check(jbool(&d, "rp_cname")),
        ],
    ];

    output::table(&["RECORD", "STATUS"], &rows);
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::ENV_LOCK;
    use std::thread::JoinHandle;

    struct Captured {
        method: String,
        url: String,
        auth: String,
        body: String,
    }

    fn mock(response: Value) -> (Client, JoinHandle<Captured>) {
        let server = tiny_http::Server::http("127.0.0.1:0").expect("bind mock server");
        let addr = format!("http://{}", server.server_addr());
        let handle = std::thread::spawn(move || {
            let mut req = server.recv().expect("receive request");
            let mut body = String::new();
            req.as_reader()
                .read_to_string(&mut body)
                .expect("read body");
            let cap = Captured {
                method: req.method().to_string(),
                url: req.url().to_string(),
                auth: req
                    .headers()
                    .iter()
                    .find(|h| h.field.equiv("Authorization"))
                    .map(|h| h.value.to_string())
                    .unwrap_or_default(),
                body,
            };
            let resp = tiny_http::Response::from_string(response.to_string()).with_header(
                tiny_http::Header::from_bytes(&b"Content-Type"[..], &b"application/json"[..])
                    .expect("header"),
            );
            req.respond(resp).expect("respond");
            cap
        });
        let client = {
            let _guard = ENV_LOCK.lock().unwrap();
            std::env::set_var("MAILERSEND_API_BASE_URL", &addr);
            let client = Client::new("test-token".into(), false);
            std::env::remove_var("MAILERSEND_API_BASE_URL");
            client
        };
        (client, handle)
    }

    fn test_ctx(json: bool) -> Ctx {
        Ctx {
            profile: None,
            verbose: false,
            json,
        }
    }

    fn list_response() -> Value {
        json!({
            "data": [
                {
                    "id": "domain-id-1",
                    "name": "example.com",
                    "is_verified": true,
                    "is_dns_active": true,
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-02T00:00:00Z"
                },
                {
                    "id": "domain-id-2",
                    "name": "test.com",
                    "is_verified": false,
                    "is_dns_active": false,
                    "created_at": "2024-02-01T00:00:00Z",
                    "updated_at": "2024-02-02T00:00:00Z"
                }
            ],
            "links": {"first": "", "last": "", "prev": "", "next": ""},
            "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 2}
        })
    }

    #[test]
    fn list_hits_domains_endpoint() {
        let (client, server) = mock(list_response());

        list(&test_ctx(false), &client, 0, None).expect("list");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "GET");
        assert_eq!(cap.url.split('?').next().unwrap(), "/domains");
        assert_eq!(cap.auth, "Bearer test-token");
    }

    #[test]
    fn get_hits_domain_endpoint() {
        let (client, server) = mock(json!({
            "data": {
                "id": "domain-id-1",
                "name": "example.com",
                "is_verified": true,
                "is_dns_active": true,
                "dkim": true,
                "spf": true,
                "tracking": false,
                "created_at": "2024-01-01T00:00:00Z",
                "updated_at": "2024-01-02T00:00:00Z"
            }
        }));

        // An ID (no dot) skips domain resolution.
        get(&test_ctx(false), &client, "domain-id-1").expect("get");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "GET");
        assert_eq!(cap.url, "/domains/domain-id-1");
    }

    #[test]
    fn add_posts_domain_body() {
        let (client, server) = mock(json!({
            "data": {"id": "new-domain-id", "name": "newdomain.com"}
        }));

        add(&test_ctx(false), &client, "newdomain.com", "rp", "track").expect("add");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "POST");
        assert_eq!(cap.url, "/domains");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["name"], "newdomain.com");
        assert_eq!(body["return_path_subdomain"], "rp");
        assert_eq!(body["custom_tracking_subdomain"], "track");
    }

    #[test]
    fn list_json_output_succeeds() {
        let (client, server) = mock(json!({
            "data": [{"id": "d1", "name": "example.com"}],
            "links": {"first": "", "last": "", "prev": "", "next": ""},
            "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 1}
        }));

        list(&test_ctx(true), &client, 0, None).expect("list --json");
        server.join().expect("server thread");
    }
}
