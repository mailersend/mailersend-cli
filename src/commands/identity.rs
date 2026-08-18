use anyhow::Result;
use clap::{Args, Subcommand};
use serde_json::{json, Value};

use crate::api::{self, Client};
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::{jstr, resolve_domain_id};

#[derive(Args)]
#[command(long_about = "List, view, create, update, and delete sender identities.")]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// List sender identities
    List {
        /// maximum number of identities to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
    },
    /// Get sender identity details
    #[command(arg_required_else_help = true)]
    Get {
        /// identity ID or email
        id_or_email: String,
    },
    /// Create a sender identity
    Create {
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// sender name (required)
        #[arg(long, default_value = "")]
        name: String,
        /// sender email (required)
        #[arg(long, default_value = "")]
        email: String,
        /// reply-to email
        #[arg(long, default_value = "")]
        reply_to_email: String,
        /// reply-to name
        #[arg(long, default_value = "")]
        reply_to_name: String,
        /// add personal note
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        add_note: Option<bool>,
        /// personal note text
        #[arg(long, default_value = "")]
        personal_note: String,
    },
    /// Update a sender identity
    #[command(arg_required_else_help = true)]
    Update {
        /// identity ID or email
        id_or_email: String,
        /// sender name
        #[arg(long)]
        name: Option<String>,
        /// reply-to email
        #[arg(long)]
        reply_to_email: Option<String>,
        /// reply-to name
        #[arg(long)]
        reply_to_name: Option<String>,
        /// add personal note
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        add_note: Option<bool>,
        /// personal note text
        #[arg(long)]
        personal_note: Option<String>,
    },
    /// Delete a sender identity
    #[command(arg_required_else_help = true)]
    Delete {
        /// identity ID or email
        id_or_email: String,
    },
}

/// Emails are addressed via the /identities/email/{email} subpath.
fn id_path(id_or_email: &str) -> String {
    if id_or_email.contains('@') {
        format!("/identities/email/{id_or_email}")
    } else {
        format!("/identities/{id_or_email}")
    }
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    let client = ctx.client()?;
    match cmd.command {
        Sub::List { limit, domain } => list(ctx, &client, limit, &domain),
        Sub::Get { id_or_email } => get(ctx, &client, &id_or_email),
        Sub::Create {
            domain,
            name,
            email,
            reply_to_email,
            reply_to_name,
            add_note,
            personal_note,
        } => create(
            ctx,
            &client,
            &domain,
            &name,
            &email,
            &reply_to_email,
            &reply_to_name,
            add_note,
            &personal_note,
        ),
        Sub::Update {
            id_or_email,
            name,
            reply_to_email,
            reply_to_name,
            add_note,
            personal_note,
        } => update(
            ctx,
            &client,
            &id_or_email,
            IdentityUpdate {
                name,
                reply_to_email,
                reply_to_name,
                add_note,
                personal_note,
            },
        ),
        Sub::Delete { id_or_email } => delete(ctx, &client, &id_or_email),
    }
}

fn list(ctx: &Ctx, client: &Client, limit: u64, domain: &str) -> Result<()> {
    let mut domain_id = String::new();
    if !domain.is_empty() {
        domain_id = resolve_domain_id(client, domain)?;
    }

    let items = api::fetch_all_paged(client, "/identities", &[("domain_id", domain_id)], limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|i| vec![jstr(i, "id"), jstr(i, "name"), jstr(i, "email")])
        .collect();

    output::table(&["ID", "NAME", "EMAIL"], &rows);
    Ok(())
}

fn get(ctx: &Ctx, client: &Client, id_or_email: &str) -> Result<()> {
    let body = client.get(&id_path(id_or_email))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let rows = vec![
        vec!["ID".to_string(), jstr(&d, "id")],
        vec!["Name".to_string(), jstr(&d, "name")],
        vec!["Email".to_string(), jstr(&d, "email")],
        vec!["Reply-To Email".to_string(), jstr(&d, "reply_to_email")],
        vec!["Reply-To Name".to_string(), jstr(&d, "reply_to_name")],
        vec!["Personal Note".to_string(), jstr(&d, "personal_note")],
    ];

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

#[allow(clippy::too_many_arguments)]
fn create(
    ctx: &Ctx,
    client: &Client,
    domain: &str,
    name: &str,
    email: &str,
    reply_to_email: &str,
    reply_to_name: &str,
    add_note: Option<bool>,
    personal_note: &str,
) -> Result<()> {
    let domain = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(client, &domain)?;
    let name = prompt::require_arg(name, "name", "Sender name")?;
    let email = prompt::require_arg(email, "email", "Sender email")?;

    let mut body = json!({ "domain_id": domain_id, "name": name, "email": email });
    if !reply_to_email.is_empty() {
        body["reply_to_email"] = json!(reply_to_email);
    }
    if !reply_to_name.is_empty() {
        body["reply_to_name"] = json!(reply_to_name);
    }
    if let Some(v) = add_note {
        body["add_note"] = json!(v);
    }
    if !personal_note.is_empty() {
        body["personal_note"] = json!(personal_note);
    }

    let result = client.post("/identities", &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!(
        "Identity created successfully. ID: {}",
        jstr(&result["data"], "id")
    ));
    Ok(())
}

struct IdentityUpdate {
    name: Option<String>,
    reply_to_email: Option<String>,
    reply_to_name: Option<String>,
    add_note: Option<bool>,
    personal_note: Option<String>,
}

fn update(ctx: &Ctx, client: &Client, id_or_email: &str, update: IdentityUpdate) -> Result<()> {
    let IdentityUpdate {
        name,
        reply_to_email,
        reply_to_name,
        add_note,
        personal_note,
    } = update;
    let mut body = json!({});

    if let Some(v) = name {
        body["name"] = json!(v);
    }
    if let Some(v) = reply_to_email {
        body["reply_to_email"] = json!(v);
    }
    if let Some(v) = reply_to_name {
        body["reply_to_name"] = json!(v);
    }
    if let Some(v) = add_note {
        body["add_note"] = json!(v);
    }
    if let Some(v) = personal_note {
        body["personal_note"] = json!(v);
    }

    let result = client.put(&id_path(id_or_email), &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!("Identity {id_or_email} updated successfully."));
    Ok(())
}

fn delete(_ctx: &Ctx, client: &Client, id_or_email: &str) -> Result<()> {
    client.delete(&id_path(id_or_email))?;
    output::success(&format!("Identity {id_or_email} deleted successfully."));
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
                {"id": "identity-1", "name": "Alice", "email": "alice@example.com"},
                {"id": "identity-2", "name": "Bob", "email": "bob@example.com"}
            ],
            "links": {"first": "", "last": "", "prev": "", "next": ""},
            "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 2}
        })
    }

    #[test]
    fn list_hits_identities_endpoint() {
        let (client, server) = mock(list_response());

        list(&test_ctx(false), &client, 0, "").expect("list");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "GET");
        assert_eq!(cap.url.split('?').next().unwrap(), "/identities");
        assert_eq!(cap.auth, "Bearer test-token");
    }

    #[test]
    fn list_sends_domain_id_query() {
        let (client, server) = mock(list_response());

        list(&test_ctx(false), &client, 0, "dom-1").expect("list");

        let cap = server.join().expect("server thread");
        assert!(cap.url.contains("domain_id=dom-1"));
    }

    #[test]
    fn get_by_email_uses_email_subpath() {
        let (client, server) = mock(json!({
            "data": {
                "id": "identity-1",
                "name": "Alice",
                "email": "alice@example.com",
                "reply_to_email": null,
                "reply_to_name": null,
                "personal_note": null
            }
        }));

        get(&test_ctx(false), &client, "alice@example.com").expect("get");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "GET");
        assert_eq!(cap.url, "/identities/email/alice@example.com");
    }

    #[test]
    fn create_posts_identity_body() {
        let (client, server) = mock(json!({
            "data": {"id": "new-identity", "name": "Alice", "email": "alice@example.com"}
        }));

        create(
            &test_ctx(false),
            &client,
            "dom-1",
            "Alice",
            "alice@example.com",
            "noreply@example.com",
            "No Reply",
            Some(true),
            "hi there",
        )
        .expect("create");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "POST");
        assert_eq!(cap.url, "/identities");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["domain_id"], "dom-1");
        assert_eq!(body["name"], "Alice");
        assert_eq!(body["email"], "alice@example.com");
        assert_eq!(body["reply_to_email"], "noreply@example.com");
        assert_eq!(body["reply_to_name"], "No Reply");
        assert_eq!(body["add_note"], true);
        assert_eq!(body["personal_note"], "hi there");
    }

    #[test]
    fn create_omits_unset_optional_fields() {
        let (client, server) = mock(json!({
            "data": {"id": "new-identity"}
        }));

        create(
            &test_ctx(false),
            &client,
            "dom-1",
            "Alice",
            "alice@example.com",
            "",
            "",
            None,
            "",
        )
        .expect("create");

        let cap = server.join().expect("server thread");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["reply_to_email"], Value::Null);
        assert_eq!(body["reply_to_name"], Value::Null);
        assert_eq!(body["add_note"], Value::Null);
        assert_eq!(body["personal_note"], Value::Null);
    }

    #[test]
    fn update_puts_only_changed_fields() {
        let (client, server) = mock(json!({"data": {"id": "identity-1"}}));

        update(
            &test_ctx(false),
            &client,
            "identity-1",
            IdentityUpdate {
                name: Some("New Name".to_string()),
                reply_to_email: None,
                reply_to_name: None,
                add_note: None,
                personal_note: None,
            },
        )
        .expect("update");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "PUT");
        assert_eq!(cap.url, "/identities/identity-1");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["name"], "New Name");
        assert_eq!(body["reply_to_email"], Value::Null);
        assert_eq!(body["add_note"], Value::Null);
    }

    #[test]
    fn update_by_email_uses_email_subpath() {
        let (client, server) = mock(json!({"data": {"id": "identity-1"}}));

        update(
            &test_ctx(false),
            &client,
            "alice@example.com",
            IdentityUpdate {
                name: Some("New Name".to_string()),
                reply_to_email: None,
                reply_to_name: None,
                add_note: None,
                personal_note: None,
            },
        )
        .expect("update");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "PUT");
        assert_eq!(cap.url, "/identities/email/alice@example.com");
    }

    #[test]
    fn delete_by_email_uses_email_subpath() {
        let (client, server) = mock(json!({}));

        delete(&test_ctx(false), &client, "alice@example.com").expect("delete");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "DELETE");
        assert_eq!(cap.url, "/identities/email/alice@example.com");
    }
}
