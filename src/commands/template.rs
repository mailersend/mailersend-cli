use anyhow::Result;
use clap::{Args, Subcommand};
use serde_json::Value;

use crate::api::{self, Client};
use crate::cli::Ctx;
use crate::output;
use crate::util::{jint, jpath, jstr, resolve_domain_id};

#[derive(Args)]
#[command(long_about = "List, view, and delete email templates.")]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// List templates
    List {
        /// maximum number of templates to return
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
    },
    /// Get template details
    #[command(arg_required_else_help = true)]
    Get {
        /// template ID
        template_id: String,
    },
    /// Delete a template
    #[command(arg_required_else_help = true)]
    Delete {
        /// template ID
        template_id: String,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    match cmd.command {
        Sub::List { limit, domain } => list(ctx, limit, &domain),
        Sub::Get { template_id } => get(ctx, &template_id),
        Sub::Delete { template_id } => delete(ctx, &template_id),
    }
}

fn fetch_templates(client: &Client, limit: u64, domain_id: &str) -> Result<Vec<Value>> {
    let mut query: Vec<(&str, String)> = Vec::new();
    if !domain_id.is_empty() {
        query.push(("domain_id", domain_id.to_string()));
    }
    api::fetch_all_paged(client, "/templates", &query, limit)
}

fn list(ctx: &Ctx, limit: u64, domain: &str) -> Result<()> {
    let client = ctx.client()?;

    let mut domain_id = domain.to_string();
    if !domain_id.is_empty() {
        domain_id = resolve_domain_id(&client, &domain_id)?;
    }

    let items = fetch_templates(&client, limit, &domain_id)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|t| {
            vec![
                jstr(t, "id"),
                output::truncate(&jstr(t, "name"), 40),
                jstr(t, "type"),
                jstr(t, "created_at"),
            ]
        })
        .collect();

    output::table(&["ID", "NAME", "TYPE", "CREATED AT"], &rows);
    Ok(())
}

fn fmt_datetime(s: &str) -> String {
    chrono::DateTime::parse_from_rfc3339(s)
        .map(|d| d.format("%Y-%m-%d %H:%M:%S").to_string())
        .unwrap_or_default()
}

fn get(ctx: &Ctx, template_id: &str) -> Result<()> {
    let client = ctx.client()?;

    let result = client.get(&format!("/templates/{template_id}"))?;

    if ctx.json {
        return output::json(&result);
    }

    let d = &result["data"];

    let category = match d.get("category") {
        Some(c) if !c.is_null() => {
            if c.is_object() {
                format!(
                    "{} ({})",
                    jpath(d, "category.name"),
                    jpath(d, "category.id")
                )
            } else {
                c.to_string()
            }
        }
        _ => "—".to_string(),
    };

    let domain_id = jpath(d, "domain.id");
    let domain = if domain_id.is_empty() {
        "—".to_string()
    } else {
        format!("{} ({})", jpath(d, "domain.name"), domain_id)
    };

    let stats = &d["template_stats"];

    println!("ID:           {}", jstr(d, "id"));
    println!("Name:         {}", jstr(d, "name"));
    println!("Type:         {}", jstr(d, "type"));
    println!("Image Path:   {}", jstr(d, "image_path"));
    println!("Created At:   {}", fmt_datetime(&jstr(d, "created_at")));
    println!("Category:     {category}");
    println!("Domain:       {domain}");
    println!();
    println!("Stats:");
    println!("  Total:          {}", jint(stats, "total"));
    println!("  Queued:         {}", jint(stats, "queued"));
    println!("  Sent:           {}", jint(stats, "sent"));
    println!("  Rejected:       {}", jint(stats, "rejected"));
    println!("  Delivered:      {}", jint(stats, "delivered"));
    println!(
        "  Last Sent At:   {}",
        fmt_datetime(&jstr(stats, "last_email_sent_at"))
    );

    Ok(())
}

fn delete(ctx: &Ctx, template_id: &str) -> Result<()> {
    let client = ctx.client()?;

    client.delete(&format!("/templates/{template_id}"))?;

    output::success(&format!("Template {template_id} deleted successfully."));
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
                            {"id": "tmpl-1", "name": "Welcome Email", "type": "html",
                             "created_at": "2024-01-15T10:00:00Z"},
                            {"id": "tmpl-2", "name": "Password Reset", "type": "html",
                             "created_at": "2024-02-20T12:00:00Z"}
                        ],
                        "links": {"next": ""},
                        "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 2}
                    }"#;
                    const SINGLE_BODY: &str = r#"{
                        "data": {
                            "id": "tmpl-abc", "name": "My Template", "type": "html",
                            "image_path": "https://example.com/img.png",
                            "created_at": "2024-01-15T10:00:00Z",
                            "category": null, "domain": null,
                            "template_stats": {
                                "total": 100, "queued": 5, "sent": 90,
                                "rejected": 2, "delivered": 88,
                                "last_email_sent_at": "2024-03-01T12:00:00Z"
                            }
                        }
                    }"#;
                    let path = req.url().split('?').next().unwrap_or("");
                    let resp_body = if path == "/templates" && req.method().as_str() == "GET" {
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
    fn list_requests_templates_endpoint() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);

        let items = fetch_templates(&client, 0, "").unwrap();
        assert_eq!(items.len(), 2);
        assert_eq!(jstr(&items[0], "id"), "tmpl-1");

        let reqs = mock.finish();
        assert!(
            reqs.iter().any(|r| r.starts_with("GET /templates?")),
            "expected GET /templates, got {reqs:?}"
        );
    }

    #[test]
    fn list_passes_domain_id_filter() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);

        let _ = fetch_templates(&client, 0, "dom-1").unwrap();

        let reqs = mock.finish();
        assert!(
            reqs.iter()
                .any(|r| r.starts_with("GET /templates?") && r.contains("domain_id=dom-1")),
            "expected domain_id query param, got {reqs:?}"
        );
    }

    #[test]
    fn get_requests_template_by_id() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);

        let result = client.get("/templates/tmpl-abc").unwrap();
        assert_eq!(jpath(&result, "data.id"), "tmpl-abc");

        let reqs = mock.finish();
        assert!(
            reqs.iter().any(|r| r == "GET /templates/tmpl-abc "),
            "expected GET /templates/tmpl-abc, got {reqs:?}"
        );
    }
}
