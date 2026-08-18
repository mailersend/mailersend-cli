use anyhow::Result;
use clap::{Args, Subcommand};
use serde_json::{json, Value};

use crate::api::Client;
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::{jstr, resolve_domain_id};

#[derive(Args)]
#[command(long_about = "List, view, create, update, and delete webhooks.")]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// List webhooks for a domain
    List {
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// maximum number of webhooks to return
        #[arg(long, default_value_t = 0)]
        limit: u64,
    },
    /// Get webhook details
    #[command(arg_required_else_help = true)]
    Get {
        /// webhook ID
        webhook_id: String,
    },
    /// Create a webhook
    #[command(
        long_about = "Create a new webhook.\n\nValid events: activity.sent, activity.delivered, activity.soft_bounced, activity.hard_bounced, activity.opened, activity.opened_unique, activity.clicked, activity.clicked_unique, activity.unsubscribed, activity.spam_complaint, activity.survey_opened, activity.survey_submitted, maintenance.start, maintenance.end, email_single.verified, email_list.verified, bulk_email.completed"
    )]
    Create {
        /// webhook name (required)
        #[arg(long, default_value = "")]
        name: String,
        /// webhook URL (required)
        #[arg(long, default_value = "")]
        url: String,
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// webhook events (required)
        #[arg(long, value_delimiter = ',')]
        events: Option<Vec<String>>,
        /// whether the webhook is enabled
        #[arg(long, default_value_t = true)]
        enabled: bool,
        /// webhook payload version (1=legacy, 2=recommended)
        #[arg(long, default_value_t = 2)]
        version: i64,
    },
    /// Update an existing webhook
    #[command(
        arg_required_else_help = true,
        long_about = "Update an existing webhook.\n\nValid events: activity.sent, activity.delivered, activity.soft_bounced, activity.hard_bounced, activity.opened, activity.opened_unique, activity.clicked, activity.clicked_unique, activity.unsubscribed, activity.spam_complaint, activity.survey_opened, activity.survey_submitted, maintenance.start, maintenance.end, email_single.verified, email_list.verified, bulk_email.completed"
    )]
    Update {
        /// webhook ID
        webhook_id: String,
        /// webhook name
        #[arg(long)]
        name: Option<String>,
        /// webhook URL
        #[arg(long)]
        url: Option<String>,
        /// webhook events
        #[arg(long, value_delimiter = ',')]
        events: Option<Vec<String>>,
        /// whether the webhook is enabled
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        enabled: Option<bool>,
        /// webhook payload version (1 or 2)
        #[arg(long)]
        version: Option<i64>,
    },
    /// Delete a webhook
    #[command(arg_required_else_help = true)]
    Delete {
        /// webhook ID
        webhook_id: String,
    },
}

fn yes_no(b: bool) -> String {
    if b { "Yes" } else { "No" }.to_string()
}

fn fmt_rfc3339(s: &str) -> String {
    chrono::DateTime::parse_from_rfc3339(s)
        .map(|d| d.to_rfc3339_opts(chrono::SecondsFormat::Secs, false))
        .unwrap_or_else(|_| s.to_string())
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    let client = ctx.client()?;
    match cmd.command {
        Sub::List { domain, limit } => list(ctx, &client, &domain, limit),
        Sub::Get { webhook_id } => get(ctx, &client, &webhook_id),
        Sub::Create {
            name,
            url,
            domain,
            events,
            enabled,
            version,
        } => create(ctx, &client, &name, &url, &domain, events, enabled, version),
        Sub::Update {
            webhook_id,
            name,
            url,
            events,
            enabled,
            version,
        } => update(
            ctx,
            &client,
            &webhook_id,
            WebhookUpdate {
                name,
                url,
                events,
                enabled,
                version,
            },
        ),
        Sub::Delete { webhook_id } => delete(ctx, &client, &webhook_id),
    }
}

fn list(ctx: &Ctx, client: &Client, domain: &str, limit: u64) -> Result<()> {
    let domain = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(client, &domain)?;

    let mut query = vec![("domain_id", domain_id)];
    if limit > 0 {
        query.push(("limit", limit.to_string()));
    }
    let body = client.get_with("/webhooks", &query)?;
    let data = list_data(&body);

    if ctx.json {
        return output::json(&data);
    }

    let rows: Vec<Vec<String>> = data
        .as_array()
        .map(|items| {
            items
                .iter()
                .map(|w| {
                    vec![
                        jstr(w, "id"),
                        output::truncate(&jstr(w, "name"), 40),
                        output::truncate(&jstr(w, "url"), 50),
                        yes_no(w.get("enabled").and_then(Value::as_bool).unwrap_or(false)),
                        fmt_rfc3339(&jstr(w, "created_at")),
                    ]
                })
                .collect()
        })
        .unwrap_or_default();

    output::table(&["ID", "NAME", "URL", "ENABLED", "CREATED AT"], &rows);
    Ok(())
}
fn list_data(body: &Value) -> Value {
    body.get("data").cloned().unwrap_or_else(|| json!([]))
}

fn get(ctx: &Ctx, client: &Client, webhook_id: &str) -> Result<()> {
    let body = client.get(&format!("/webhooks/{webhook_id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let enabled = yes_no(d.get("enabled").and_then(Value::as_bool).unwrap_or(false));

    println!("ID:           {}", jstr(&d, "id"));
    println!("Name:         {}", jstr(&d, "name"));
    println!("URL:          {}", jstr(&d, "url"));
    println!("Enabled:      {enabled}");
    println!("Created At:   {}", fmt_rfc3339(&jstr(&d, "created_at")));
    println!("Updated At:   {}", fmt_rfc3339(&jstr(&d, "updated_at")));

    println!();
    println!("Events:");
    if let Some(Value::Array(events)) = d.get("events") {
        for e in events {
            if let Value::String(s) = e {
                println!("  - {s}");
            }
        }
    }

    Ok(())
}

#[allow(clippy::too_many_arguments)]
fn create(
    ctx: &Ctx,
    client: &Client,
    name: &str,
    url: &str,
    domain: &str,
    events: Option<Vec<String>>,
    enabled: bool,
    version: i64,
) -> Result<()> {
    let name = prompt::require_arg(name, "name", "Webhook name")?;
    let url = prompt::require_arg(url, "url", "Webhook URL")?;
    let domain = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(client, &domain)?;
    let events =
        prompt::require_slice_arg(&events.unwrap_or_default(), "events", "Webhook events")?;

    let body = json!({
        "name": name,
        "domain_id": domain_id,
        "url": url,
        "enabled": enabled,
        "events": events,
        "version": version,
    });

    let result = client.post("/webhooks", &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!(
        "Webhook created successfully. ID: {}",
        jstr(&result["data"], "id")
    ));
    Ok(())
}

struct WebhookUpdate {
    name: Option<String>,
    url: Option<String>,
    events: Option<Vec<String>>,
    enabled: Option<bool>,
    version: Option<i64>,
}

fn update(ctx: &Ctx, client: &Client, webhook_id: &str, update: WebhookUpdate) -> Result<()> {
    let WebhookUpdate {
        name,
        url,
        events,
        enabled,
        version,
    } = update;
    let mut body = json!({});

    if let Some(v) = name {
        body["name"] = json!(v);
    }
    if let Some(v) = url {
        body["url"] = json!(v);
    }
    if let Some(v) = events {
        body["events"] = json!(v);
    }
    if let Some(v) = enabled {
        body["enabled"] = json!(v);
    }
    if let Some(v) = version {
        body["version"] = json!(v);
    }

    let result = client.put(&format!("/webhooks/{webhook_id}"), &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!("Webhook {webhook_id} updated successfully."));
    Ok(())
}

fn delete(_ctx: &Ctx, client: &Client, webhook_id: &str) -> Result<()> {
    client.delete(&format!("/webhooks/{webhook_id}"))?;
    output::success(&format!("Webhook {webhook_id} deleted successfully."));
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
                    "id": "wh-1",
                    "url": "https://example.com/hook",
                    "events": ["activity.sent"],
                    "name": "Test Webhook",
                    "enabled": true,
                    "editable": true,
                    "created_at": "2024-01-01T00:00:00Z",
                    "updated_at": "2024-01-01T00:00:00Z"
                }
            ],
            "links": {
                "first": "https://api.mailersend.com/v1/webhooks?page=1",
                "last": "https://api.mailersend.com/v1/webhooks?page=1",
                "prev": "",
                "next": ""
            },
            "meta": {
                "current_page": 1,
                "from": 1,
                "path": "https://api.mailersend.com/v1/webhooks",
                "per_page": 25,
                "to": 1
            }
        })
    }

    #[test]
    fn list_json_output_data_is_array() {
        let data = list_data(&list_response());
        assert!(data.is_array(), "JSON output data should be an array");
        assert_eq!(data[0]["id"], "wh-1");
    }

    // Port of TestWebhookListCmd_MockServer.
    #[test]
    fn list_hits_webhooks_endpoint_with_domain_query() {
        let (client, server) = mock(json!({
            "data": [],
            "links": {"first": "", "last": "", "prev": "", "next": ""},
            "meta": {"current_page": 1, "from": 0, "per_page": 25, "to": 0}
        }));

        list(&test_ctx(false), &client, "dom-1", 0).expect("list");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "GET");
        assert_eq!(cap.url.split('?').next().unwrap(), "/webhooks");
        assert!(
            cap.url.contains("domain_id=dom-1"),
            "expected domain_id in query string"
        );
    }

    #[test]
    fn list_passes_limit_query_when_set() {
        let (client, server) = mock(json!({
            "data": [],
            "links": {"first": "", "last": "", "prev": "", "next": ""},
            "meta": {"current_page": 1, "from": 0, "per_page": 25, "to": 0}
        }));

        list(&test_ctx(false), &client, "dom-1", 5).expect("list");

        let cap = server.join().expect("server thread");
        assert!(cap.url.contains("limit=5"));
    }

    #[test]
    fn create_posts_webhook_body() {
        let (client, server) = mock(json!({"data": {"id": "wh-new"}}));

        create(
            &test_ctx(false),
            &client,
            "My Hook",
            "https://example.com/hook",
            "dom-1",
            Some(vec![
                "activity.sent".to_string(),
                "activity.delivered".to_string(),
            ]),
            true,
            2,
        )
        .expect("create");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "POST");
        assert_eq!(cap.url, "/webhooks");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["name"], "My Hook");
        assert_eq!(body["domain_id"], "dom-1");
        assert_eq!(body["url"], "https://example.com/hook");
        assert_eq!(body["enabled"], true);
        assert_eq!(
            body["events"],
            json!(["activity.sent", "activity.delivered"])
        );
        assert_eq!(body["version"], 2);
    }

    #[test]
    fn update_puts_only_changed_fields() {
        let (client, server) = mock(json!({"data": {"id": "wh-1"}}));

        update(
            &test_ctx(false),
            &client,
            "wh-1",
            WebhookUpdate {
                name: Some("New Name".to_string()),
                url: None,
                events: None,
                enabled: Some(false),
                version: None,
            },
        )
        .expect("update");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "PUT");
        assert_eq!(cap.url, "/webhooks/wh-1");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["name"], "New Name");
        assert_eq!(body["enabled"], false);
        assert_eq!(body["url"], Value::Null);
        assert_eq!(body["version"], Value::Null);
    }

    #[test]
    fn delete_hits_webhooks_endpoint() {
        let (client, server) = mock(json!({}));

        delete(&test_ctx(false), &client, "wh-1").expect("delete");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "DELETE");
        assert_eq!(cap.url, "/webhooks/wh-1");
    }
}
