use anyhow::Result;
use clap::{Args, Subcommand};
use serde_json::{json, Map, Value};

use crate::api::{self, Client};
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::{jpath, jstr, resolve_domain_id};

#[derive(Args)]
#[command(long_about = "List, view, create, update, and delete API tokens.")]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// List API tokens
    List {
        /// maximum number of tokens to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
    },
    /// Get API token details
    #[command(arg_required_else_help = true)]
    Get {
        /// token ID
        id: String,
    },
    /// Create an API token
    Create {
        /// token name (required)
        #[arg(long, default_value = "")]
        name: String,
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// token scopes (required)
        #[arg(long, value_delimiter = ',')]
        scopes: Option<Vec<String>>,
    },
    /// Update an API token
    #[command(arg_required_else_help = true)]
    Update {
        /// token ID
        id: String,
        /// token name
        #[arg(long)]
        name: Option<String>,
    },
    /// Update API token status (pause/unpause)
    #[command(arg_required_else_help = true)]
    UpdateStatus {
        /// token ID
        id: String,
        /// token status: pause or unpause (required)
        #[arg(long, default_value = "")]
        status: String,
    },
    /// Delete an API token
    #[command(arg_required_else_help = true)]
    Delete {
        /// token ID
        id: String,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    match cmd.command {
        Sub::List { limit } => list(ctx, limit),
        Sub::Get { id } => get(ctx, &id),
        Sub::Create {
            name,
            domain,
            scopes,
        } => create(ctx, &name, &domain, scopes),
        Sub::Update { id, name } => update(ctx, &id, name),
        Sub::UpdateStatus { id, status } => update_status(ctx, &id, &status),
        Sub::Delete { id } => delete(ctx, &id),
    }
}

fn fetch_tokens(client: &Client, limit: u64) -> Result<Vec<Value>> {
    api::fetch_all_paged(client, "/token", &[], limit)
}

fn list(ctx: &Ctx, limit: u64) -> Result<()> {
    let client = ctx.client()?;

    let items = fetch_tokens(&client, limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|t| {
            vec![
                jstr(t, "id"),
                jstr(t, "name"),
                jstr(t, "status"),
                jstr(t, "created_at"),
            ]
        })
        .collect();

    output::table(&["ID", "NAME", "STATUS", "CREATED AT"], &rows);
    Ok(())
}

fn get(ctx: &Ctx, id: &str) -> Result<()> {
    let client = ctx.client()?;

    let body = client.get(&format!("/token/{id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = &body["data"];
    let rows = vec![
        vec!["ID".to_string(), jstr(d, "id")],
        vec!["Name".to_string(), jstr(d, "name")],
        vec!["Status".to_string(), jstr(d, "status")],
        vec!["Created At".to_string(), jstr(d, "created_at")],
    ];
    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn create_token(client: &Client, name: &str, domain_id: &str, scopes: &[String]) -> Result<Value> {
    client.post(
        "/token",
        &json!({
            "name": name,
            "domain_id": domain_id,
            "scopes": scopes,
        }),
    )
}

fn create(ctx: &Ctx, name: &str, domain: &str, scopes: Option<Vec<String>>) -> Result<()> {
    let client = ctx.client()?;

    let name = prompt::require_arg(name, "name", "Token name")?;
    let domain_id = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(&client, &domain_id)?;
    let scopes = prompt::require_slice_arg(&scopes.unwrap_or_default(), "scopes", "Token scopes")?;

    let result = create_token(&client, &name, &domain_id, &scopes)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!(
        "Token created successfully. ID: {}",
        jpath(&result, "data.id")
    ));
    let access_token = jpath(&result, "data.accessToken");
    if !access_token.is_empty() {
        println!("Access Token: {access_token}");
    }
    Ok(())
}

fn update_token(client: &Client, id: &str, name: Option<&str>) -> Result<Value> {
    let mut payload = Map::new();
    if let Some(name) = name {
        payload.insert("name".to_string(), json!(name));
    }
    client.put(&format!("/token/{id}"), &Value::Object(payload))
}

fn update(ctx: &Ctx, id: &str, name: Option<String>) -> Result<()> {
    let client = ctx.client()?;

    let result = update_token(&client, id, name.as_deref())?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!("Token {id} updated successfully."));
    Ok(())
}

fn update_token_status(client: &Client, id: &str, status: &str) -> Result<Value> {
    client.put(
        &format!("/token/{id}/settings"),
        &json!({ "status": status }),
    )
}

fn update_status(ctx: &Ctx, id: &str, status: &str) -> Result<()> {
    let client = ctx.client()?;

    let status = prompt::require_arg(status, "status", "Token status (pause or unpause)")?;

    let result = update_token_status(&client, id, &status)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!("Token {id} status updated to {status}."));
    Ok(())
}

fn delete(ctx: &Ctx, id: &str) -> Result<()> {
    let client = ctx.client()?;

    client.delete(&format!("/token/{id}"))?;

    output::success(&format!("Token {id} deleted successfully."));
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::ENV_LOCK;
    use std::sync::{mpsc, Arc};

    struct Mock {
        server: Arc<tiny_http::Server>,
        rx: mpsc::Receiver<String>,
        handle: std::thread::JoinHandle<()>,
    }

    impl Mock {
        fn start() -> (Self, String) {
            let server = Arc::new(tiny_http::Server::http("127.0.0.1:0").unwrap());
            let base = format!("http://{}", server.server_addr().to_ip().unwrap());
            let (tx, rx) = mpsc::channel();
            let srv = server.clone();
            let handle = std::thread::spawn(move || {
                for mut req in srv.incoming_requests() {
                    let mut body = String::new();
                    req.as_reader()
                        .read_to_string(&mut body)
                        .expect("read request body");
                    let _ = tx.send(format!("{} {} {}", req.method().as_str(), req.url(), body));
                    const LIST_BODY: &str = r#"{
                        "data": [
                            {"id": "tok-1", "name": "ci", "status": "unpaused",
                             "created_at": "2024-01-15T10:00:00Z"}
                        ],
                        "links": {"next": ""},
                        "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 1}
                    }"#;
                    const SINGLE_BODY: &str = r#"{
                        "data": {"id": "tok-1", "name": "ci", "status": "unpaused",
                                 "accessToken": "ms-secret", "created_at": "2024-01-15T10:00:00Z"}
                    }"#;
                    let path = req.url().split('?').next().unwrap_or("");
                    let resp_body = if path == "/token" && req.method().as_str() == "GET" {
                        LIST_BODY
                    } else {
                        SINGLE_BODY
                    };
                    let resp = tiny_http::Response::from_string(resp_body).with_header(
                        tiny_http::Header::from_bytes(
                            &b"Content-Type"[..],
                            &b"application/json"[..],
                        )
                        .unwrap(),
                    );
                    let _ = req.respond(resp);
                }
            });
            (Mock { server, rx, handle }, base)
        }

        fn finish(self) -> Vec<String> {
            self.server.unblock();
            let _ = self.handle.join();
            self.rx.try_iter().collect()
        }
    }

    fn client_for(base: &str) -> Client {
        let _guard = ENV_LOCK.lock().unwrap_or_else(|e| e.into_inner());
        std::env::set_var("MAILERSEND_API_BASE_URL", base);
        let client = Client::new("test-token".into(), false);
        std::env::remove_var("MAILERSEND_API_BASE_URL");
        client
    }

    #[test]
    fn list_requests_token_endpoint() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);

        let items = fetch_tokens(&client, 0).unwrap();
        assert_eq!(items.len(), 1);
        assert_eq!(jstr(&items[0], "id"), "tok-1");

        let reqs = mock.finish();
        assert!(
            reqs.iter().any(|r| r.starts_with("GET /token?")),
            "expected GET /token, got {reqs:?}"
        );
    }

    #[test]
    fn create_posts_scopes_body() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);

        let result = create_token(
            &client,
            "ci",
            "dom-1",
            &["domains.read".to_string(), "emails.send".to_string()],
        )
        .unwrap();
        assert_eq!(jpath(&result, "data.id"), "tok-1");

        let reqs = mock.finish();
        assert!(
            reqs.iter().any(|r| r.starts_with("POST /token ")
                && r.contains(r#""name":"ci""#)
                && r.contains(r#""domain_id":"dom-1""#)
                && r.contains(r#""scopes":["domains.read","emails.send"]"#)),
            "expected POST /token with name/domain_id/scopes body, got {reqs:?}"
        );
    }

    #[test]
    fn update_puts_name_when_provided() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);

        update_token(&client, "tok-1", Some("renamed")).unwrap();
        update_token(&client, "tok-1", None).unwrap();

        let reqs = mock.finish();
        assert!(
            reqs.iter()
                .any(|r| r == "PUT /token/tok-1 {\"name\":\"renamed\"}"),
            "expected PUT /token/tok-1 with name body, got {reqs:?}"
        );
        assert!(
            reqs.iter().any(|r| r == "PUT /token/tok-1 {}"),
            "expected empty body when no flags changed, got {reqs:?}"
        );
    }

    #[test]
    fn update_status_puts_settings_endpoint() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);

        update_token_status(&client, "tok-1", "pause").unwrap();

        let reqs = mock.finish();
        assert!(
            reqs.iter()
                .any(|r| r == "PUT /token/tok-1/settings {\"status\":\"pause\"}"),
            "expected PUT /token/tok-1/settings with status body, got {reqs:?}"
        );
    }

    #[test]
    fn get_requests_token_by_id() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);

        let body = client.get("/token/tok-1").unwrap();
        assert_eq!(jpath(&body, "data.id"), "tok-1");

        let reqs = mock.finish();
        assert!(
            reqs.iter().any(|r| r == "GET /token/tok-1 "),
            "expected GET /token/tok-1, got {reqs:?}"
        );
    }
}
