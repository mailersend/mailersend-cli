use anyhow::Result;
use clap::{Args, Subcommand};
use serde_json::{json, Value};

use crate::api::{self, Client};
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::{jstr, resolve_domain_id};

#[derive(Args)]
#[command(long_about = "List, view, create, update, and delete SMTP users for a domain.")]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// List SMTP users
    List {
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// maximum number of SMTP users to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
    },
    /// Get SMTP user details
    #[command(arg_required_else_help = true)]
    Get {
        /// SMTP user ID
        smtp_user_id: String,
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
    },
    /// Create an SMTP user
    Create {
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// SMTP user name (required)
        #[arg(long, default_value = "")]
        name: String,
        /// whether the SMTP user is enabled
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        enabled: Option<bool>,
    },
    /// Update an SMTP user
    #[command(arg_required_else_help = true)]
    Update {
        /// SMTP user ID
        smtp_user_id: String,
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// SMTP user name
        #[arg(long)]
        name: Option<String>,
        /// whether the SMTP user is enabled
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        enabled: Option<bool>,
    },
    /// Delete an SMTP user
    #[command(arg_required_else_help = true)]
    Delete {
        /// SMTP user ID
        smtp_user_id: String,
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
    },
}

fn yes_no(b: bool) -> String {
    if b { "Yes" } else { "No" }.to_string()
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    let client = ctx.client()?;
    match cmd.command {
        Sub::List { domain, limit } => list(ctx, &client, &domain, limit),
        Sub::Get {
            smtp_user_id,
            domain,
        } => get(ctx, &client, &domain, &smtp_user_id),
        Sub::Create {
            domain,
            name,
            enabled,
        } => create(ctx, &client, &domain, &name, enabled),
        Sub::Update {
            smtp_user_id,
            domain,
            name,
            enabled,
        } => update(ctx, &client, &domain, &smtp_user_id, name, enabled),
        Sub::Delete {
            smtp_user_id,
            domain,
        } => delete(ctx, &client, &domain, &smtp_user_id),
    }
}

fn list(ctx: &Ctx, client: &Client, domain: &str, limit: u64) -> Result<()> {
    let domain = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(client, &domain)?;

    let items = api::fetch_all_paged(
        client,
        &format!("/domains/{domain_id}/smtp-users"),
        &[],
        limit,
    )?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|s| {
            vec![
                jstr(s, "id"),
                jstr(s, "name"),
                yes_no(s.get("enabled").and_then(Value::as_bool).unwrap_or(false)),
            ]
        })
        .collect();

    output::table(&["ID", "NAME", "ENABLED"], &rows);
    Ok(())
}

fn get(ctx: &Ctx, client: &Client, domain: &str, smtp_user_id: &str) -> Result<()> {
    let domain = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(client, &domain)?;

    let body = client.get(&format!("/domains/{domain_id}/smtp-users/{smtp_user_id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let rows = vec![
        vec!["ID".to_string(), jstr(&d, "id")],
        vec!["Name".to_string(), jstr(&d, "name")],
        vec![
            "Enabled".to_string(),
            yes_no(d.get("enabled").and_then(Value::as_bool).unwrap_or(false)),
        ],
    ];

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn create(
    ctx: &Ctx,
    client: &Client,
    domain: &str,
    name: &str,
    enabled: Option<bool>,
) -> Result<()> {
    let domain = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(client, &domain)?;
    let name = prompt::require_arg(name, "name", "SMTP user name")?;

    let mut body = json!({ "name": name });
    if let Some(v) = enabled {
        body["enabled"] = json!(v);
    }

    let result = client.post(&format!("/domains/{domain_id}/smtp-users"), &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!(
        "SMTP user created successfully. ID: {}",
        jstr(&result["data"], "id")
    ));
    Ok(())
}

fn update(
    ctx: &Ctx,
    client: &Client,
    domain: &str,
    smtp_user_id: &str,
    name: Option<String>,
    enabled: Option<bool>,
) -> Result<()> {
    let domain = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(client, &domain)?;

    let mut body = json!({});

    if let Some(v) = name {
        body["name"] = json!(v);
    }
    if let Some(v) = enabled {
        body["enabled"] = json!(v);
    }

    let result = client.put(
        &format!("/domains/{domain_id}/smtp-users/{smtp_user_id}"),
        &body,
    )?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!("SMTP user {smtp_user_id} updated successfully."));
    Ok(())
}

fn delete(_ctx: &Ctx, client: &Client, domain: &str, smtp_user_id: &str) -> Result<()> {
    let domain = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(client, &domain)?;

    client.delete(&format!("/domains/{domain_id}/smtp-users/{smtp_user_id}"))?;
    output::success(&format!("SMTP user {smtp_user_id} deleted successfully."));
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
                {"id": "smtp-1", "name": "User One", "enabled": true},
                {"id": "smtp-2", "name": "User Two", "enabled": false}
            ],
            "links": {"first": "", "last": "", "prev": "", "next": ""},
            "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 2}
        })
    }

    #[test]
    fn list_hits_smtp_users_endpoint() {
        let (client, server) = mock(list_response());

        list(&test_ctx(false), &client, "dom-1", 0).expect("list");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "GET");
        assert_eq!(
            cap.url.split('?').next().unwrap(),
            "/domains/dom-1/smtp-users"
        );
        assert_eq!(cap.auth, "Bearer test-token");
    }

    #[test]
    fn get_hits_smtp_user_endpoint() {
        let (client, server) = mock(json!({
            "data": {"id": "smtp-1", "name": "User One", "enabled": true}
        }));

        get(&test_ctx(false), &client, "dom-1", "smtp-1").expect("get");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "GET");
        assert_eq!(cap.url, "/domains/dom-1/smtp-users/smtp-1");
    }

    #[test]
    fn create_posts_smtp_user_body() {
        let (client, server) = mock(json!({"data": {"id": "smtp-new"}}));

        create(&test_ctx(false), &client, "dom-1", "My User", None).expect("create");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "POST");
        assert_eq!(cap.url, "/domains/dom-1/smtp-users");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["name"], "My User");
        assert_eq!(body["enabled"], Value::Null);
    }

    #[test]
    fn create_includes_enabled_when_provided() {
        let (client, server) = mock(json!({"data": {"id": "smtp-new"}}));

        create(&test_ctx(false), &client, "dom-1", "My User", Some(false)).expect("create");

        let cap = server.join().expect("server thread");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["enabled"], false);
    }

    #[test]
    fn update_puts_changed_fields() {
        let (client, server) = mock(json!({"data": {"id": "smtp-1"}}));

        update(
            &test_ctx(false),
            &client,
            "dom-1",
            "smtp-1",
            Some("New Name".to_string()),
            None,
        )
        .expect("update");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "PUT");
        assert_eq!(cap.url, "/domains/dom-1/smtp-users/smtp-1");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["name"], "New Name");
        assert_eq!(body["enabled"], Value::Null);
    }

    #[test]
    fn delete_hits_smtp_user_endpoint() {
        let (client, server) = mock(json!({}));

        delete(&test_ctx(false), &client, "dom-1", "smtp-1").expect("delete");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "DELETE");
        assert_eq!(cap.url, "/domains/dom-1/smtp-users/smtp-1");
    }
}
