use anyhow::Result;
use clap::{Args, Subcommand};
use serde_json::Value;

use crate::api::{self, Client};
use crate::cli::Ctx;
use crate::output;
use crate::util::{jstr, resolve_domain_name};

#[derive(Args)]
#[command(long_about = "List, view, and delete recipients.")]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// List recipients
    List {
        /// maximum number of recipients to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
    },
    /// Get recipient details
    #[command(arg_required_else_help = true)]
    Get {
        /// recipient ID
        recipient_id: String,
    },
    /// Delete a recipient
    #[command(arg_required_else_help = true)]
    Delete {
        /// recipient ID
        recipient_id: String,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    match cmd.command {
        Sub::List { limit, domain } => list(ctx, limit, &domain),
        Sub::Get { recipient_id } => get(ctx, &recipient_id),
        Sub::Delete { recipient_id } => delete(ctx, &recipient_id),
    }
}

// The MailerSend API /recipients endpoint does not support domain_id
// filtering, so we resolve the domain to a name and filter client-side by
// email suffix. When filtering we must fetch all recipients (limit 0) and
// trim afterward.
fn fetch_recipients(client: &Client, limit: u64, domain: &str) -> Result<Vec<Value>> {
    let mut suffix = String::new();
    if !domain.is_empty() {
        let name = resolve_domain_name(client, domain)?;
        suffix = format!("@{}", name.to_lowercase());
    }

    let fetch_limit = if suffix.is_empty() { limit } else { 0 };

    let mut items = api::fetch_all_paged(client, "/recipients", &[], fetch_limit)?;

    if !suffix.is_empty() {
        items.retain(|r| jstr(r, "email").to_lowercase().ends_with(&suffix));
        if limit > 0 && items.len() as u64 > limit {
            items.truncate(limit as usize);
        }
    }

    Ok(items)
}

fn list(ctx: &Ctx, limit: u64, domain: &str) -> Result<()> {
    let client = ctx.client()?;

    let items = fetch_recipients(&client, limit, domain)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|r| vec![jstr(r, "id"), jstr(r, "email"), jstr(r, "created_at")])
        .collect();

    output::table(&["ID", "EMAIL", "CREATED AT"], &rows);
    Ok(())
}

fn get(ctx: &Ctx, recipient_id: &str) -> Result<()> {
    let client = ctx.client()?;

    let body = client.get(&format!("/recipients/{recipient_id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = &body["data"];
    let rows = vec![
        vec!["ID".to_string(), jstr(d, "id")],
        vec!["Email".to_string(), jstr(d, "email")],
        vec!["Created At".to_string(), jstr(d, "created_at")],
        vec!["Updated At".to_string(), jstr(d, "updated_at")],
    ];
    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn delete(ctx: &Ctx, recipient_id: &str) -> Result<()> {
    let client = ctx.client()?;

    client.delete(&format!("/recipients/{recipient_id}"))?;

    output::success(&format!("Recipient {recipient_id} deleted successfully."));
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::ENV_LOCK;
    use std::sync::{mpsc, Arc};

    const EMPTY_LIST: &str = r#"{
        "data": [],
        "links": {"next": ""},
        "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 0}
    }"#;

    const DOMAINS: &str = r#"{
        "data": [
            {"id": "dom-1", "name": "example.com"},
            {"id": "dom-2", "name": "test-sdk.com"}
        ],
        "links": {"next": ""},
        "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 2}
    }"#;

    const RECIPIENTS: &str = r#"{
        "data": [
            {"id": "r1", "email": "alice@example.com", "created_at": "2024-01-01T00:00:00Z", "updated_at": "2024-01-01T00:00:00Z", "deleted_at": ""},
            {"id": "r2", "email": "bob@test-sdk.com", "created_at": "2024-01-02T00:00:00Z", "updated_at": "2024-01-02T00:00:00Z", "deleted_at": ""},
            {"id": "r3", "email": "carol@test-sdk.com", "created_at": "2024-01-03T00:00:00Z", "updated_at": "2024-01-03T00:00:00Z", "deleted_at": ""},
            {"id": "r4", "email": "dave@other.org", "created_at": "2024-01-04T00:00:00Z", "updated_at": "2024-01-04T00:00:00Z", "deleted_at": ""}
        ],
        "links": {"next": ""},
        "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 4}
    }"#;

    struct Mock {
        server: Arc<tiny_http::Server>,
        rx: mpsc::Receiver<String>,
        handle: std::thread::JoinHandle<()>,
    }

    impl Mock {
        fn start(recipients_body: &'static str, domains_body: &'static str) -> (Self, String) {
            let server = Arc::new(tiny_http::Server::http("127.0.0.1:0").unwrap());
            let base = format!("http://{}", server.server_addr().to_ip().unwrap());
            let (tx, rx) = mpsc::channel();
            let srv = server.clone();
            let handle = std::thread::spawn(move || {
                for req in srv.incoming_requests() {
                    let url = req.url().to_string();
                    let _ = tx.send(url.clone());
                    let body = if url.starts_with("/domains") {
                        domains_body
                    } else {
                        recipients_body
                    };
                    let resp = tiny_http::Response::from_string(body).with_header(
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

        /// Shuts the server down and returns every request path it served.
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
    fn list_no_domain_filter() {
        let (mock, base) = Mock::start(EMPTY_LIST, DOMAINS);
        let client = client_for(&base);

        let items = fetch_recipients(&client, 0, "").unwrap();
        assert!(items.is_empty());

        let paths = mock.finish();
        assert!(
            paths.iter().any(|p| p.starts_with("/recipients")),
            "expected /recipients, got {paths:?}"
        );
        assert!(
            paths.iter().all(|p| !p.starts_with("/domains")),
            "domain resolution should not run without --domain"
        );
    }

    #[test]
    fn list_domain_filter_by_name() {
        let (mock, base) = Mock::start(RECIPIENTS, DOMAINS);
        let client = client_for(&base);

        let items = fetch_recipients(&client, 0, "test-sdk.com").unwrap();

        let paths = mock.finish();
        assert!(
            paths.iter().any(|p| p.starts_with("/recipients")),
            "expected /recipients endpoint to be called"
        );
        let emails: Vec<String> = items.iter().map(|r| jstr(r, "email")).collect();
        assert_eq!(emails, vec!["bob@test-sdk.com", "carol@test-sdk.com"]);
    }

    #[test]
    fn list_domain_filter_by_id() {
        let (mock, base) = Mock::start(RECIPIENTS, DOMAINS);
        let client = client_for(&base);

        let items = fetch_recipients(&client, 0, "dom-2").unwrap();

        let paths = mock.finish();
        assert!(
            paths.iter().any(|p| p.starts_with("/domains")),
            "expected domain ID to be resolved via /domains"
        );
        let emails: Vec<String> = items.iter().map(|r| jstr(r, "email")).collect();
        assert_eq!(emails, vec!["bob@test-sdk.com", "carol@test-sdk.com"]);
    }
}
