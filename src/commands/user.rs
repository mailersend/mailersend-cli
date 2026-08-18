use anyhow::Result;
use clap::{Args, Subcommand};
use serde_json::{json, Map, Value};

use crate::api::{self, Client};
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::jstr;

#[derive(Args)]
#[command(long_about = "List, view, invite, update, and delete account users. Manage invites.")]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// List account users
    List {
        /// maximum number of users to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
    },
    /// Get user details
    #[command(arg_required_else_help = true)]
    Get {
        /// user ID
        id: String,
    },
    /// Manage user invitations
    Invite {
        #[command(subcommand)]
        command: InviteSub,
    },
    /// Update a user
    #[command(arg_required_else_help = true)]
    Update {
        /// user ID
        id: String,
        /// user role
        #[arg(long)]
        role: Option<String>,
        /// permissions
        #[arg(long, value_delimiter = ',')]
        permissions: Option<Vec<String>>,
        /// template IDs
        #[arg(long, value_delimiter = ',')]
        templates: Option<Vec<String>>,
        /// domain IDs
        #[arg(long, value_delimiter = ',')]
        domains: Option<Vec<String>>,
    },
    /// Delete a user
    #[command(arg_required_else_help = true)]
    Delete {
        /// user ID
        id: String,
    },
}

#[derive(Subcommand)]
enum InviteSub {
    /// Invite a new user
    Create {
        /// email address (required)
        #[arg(long, default_value = "")]
        email: String,
        /// user role (required)
        #[arg(long, default_value = "")]
        role: String,
        /// permissions
        #[arg(long, value_delimiter = ',')]
        permissions: Option<Vec<String>>,
        /// template IDs
        #[arg(long, value_delimiter = ',')]
        templates: Option<Vec<String>>,
        /// domain IDs
        #[arg(long, value_delimiter = ',')]
        domains: Option<Vec<String>>,
    },
    /// List pending invites
    List {
        /// maximum number of invites to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
    },
    /// Get invite details
    #[command(arg_required_else_help = true)]
    Get {
        /// invite ID
        id: String,
    },
    /// Resend an invite
    #[command(arg_required_else_help = true)]
    Resend {
        /// invite ID
        id: String,
    },
    /// Cancel an invite
    #[command(arg_required_else_help = true)]
    Cancel {
        /// invite ID
        id: String,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    match cmd.command {
        Sub::List { limit } => list(ctx, limit),
        Sub::Get { id } => get(ctx, &id),
        Sub::Invite { command } => match command {
            InviteSub::Create {
                email,
                role,
                permissions,
                templates,
                domains,
            } => invite_create(ctx, &email, &role, permissions, templates, domains),
            InviteSub::List { limit } => invite_list(ctx, limit),
            InviteSub::Get { id } => invite_get(ctx, &id),
            InviteSub::Resend { id } => invite_resend(ctx, &id),
            InviteSub::Cancel { id } => invite_cancel(ctx, &id),
        },
        Sub::Update {
            id,
            role,
            permissions,
            templates,
            domains,
        } => update(ctx, &id, role, permissions, templates, domains),
        Sub::Delete { id } => delete(ctx, &id),
    }
}

fn fetch_users(client: &Client, limit: u64) -> Result<Vec<Value>> {
    api::fetch_all_paged(client, "/users", &[], limit)
}

fn list(ctx: &Ctx, limit: u64) -> Result<()> {
    let client = ctx.client()?;

    let items = fetch_users(&client, limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|u| vec![jstr(u, "id"), jstr(u, "email"), jstr(u, "role")])
        .collect();

    output::table(&["ID", "EMAIL", "ROLE"], &rows);
    Ok(())
}

fn get(ctx: &Ctx, id: &str) -> Result<()> {
    let client = ctx.client()?;

    let body = client.get(&format!("/users/{id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = &body["data"];
    let rows = vec![
        vec!["ID".to_string(), jstr(d, "id")],
        vec!["Email".to_string(), jstr(d, "email")],
        vec!["Role".to_string(), jstr(d, "role")],
    ];
    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn invite_payload(
    email: &str,
    role: &str,
    permissions: Option<Vec<String>>,
    templates: Option<Vec<String>>,
    domains: Option<Vec<String>>,
) -> Value {
    let mut payload = json!({
        "email": email,
        "role": role,
    });
    if let Some(perms) = permissions.filter(|p| !p.is_empty()) {
        payload["permissions"] = json!(perms);
    }
    if let Some(templates) = templates.filter(|t| !t.is_empty()) {
        payload["templates"] = json!(templates);
    }
    if let Some(domains) = domains.filter(|d| !d.is_empty()) {
        payload["domains"] = json!(domains);
    }
    payload
}

fn invite_create(
    ctx: &Ctx,
    email: &str,
    role: &str,
    permissions: Option<Vec<String>>,
    templates: Option<Vec<String>>,
    domains: Option<Vec<String>>,
) -> Result<()> {
    let client = ctx.client()?;

    let email = prompt::require_arg(email, "email", "Email address")?;
    let role = prompt::require_arg(role, "role", "User role")?;

    let payload = invite_payload(&email, &role, permissions, templates, domains);

    let body = client.post("/users", &payload)?;

    if ctx.json {
        return output::json(&body);
    }

    output::success(&format!("User invitation sent to {email}."));
    Ok(())
}

fn fetch_invites(client: &Client, limit: u64) -> Result<Vec<Value>> {
    api::fetch_all_paged(client, "/invites", &[], limit)
}

fn invite_list(ctx: &Ctx, limit: u64) -> Result<()> {
    let client = ctx.client()?;

    let items = fetch_invites(&client, limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|i| vec![jstr(i, "id"), jstr(i, "email"), jstr(i, "role")])
        .collect();

    output::table(&["ID", "EMAIL", "ROLE"], &rows);
    Ok(())
}

fn invite_get(ctx: &Ctx, id: &str) -> Result<()> {
    let client = ctx.client()?;

    let body = client.get(&format!("/invites/{id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = &body["data"];
    let rows = vec![
        vec!["ID".to_string(), jstr(d, "id")],
        vec!["Email".to_string(), jstr(d, "email")],
        vec!["Role".to_string(), jstr(d, "role")],
    ];
    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn invite_resend(ctx: &Ctx, id: &str) -> Result<()> {
    let client = ctx.client()?;

    client.request("POST", &format!("/invites/{id}/resend"), &[], None)?;

    output::success(&format!("Invite {id} resent successfully."));
    Ok(())
}

fn invite_cancel(ctx: &Ctx, id: &str) -> Result<()> {
    let client = ctx.client()?;

    client.delete(&format!("/invites/{id}"))?;

    output::success(&format!("Invite {id} cancelled successfully."));
    Ok(())
}

fn update_user(
    client: &Client,
    id: &str,
    role: Option<&str>,
    permissions: Option<&[String]>,
    templates: Option<&[String]>,
    domains: Option<&[String]>,
) -> Result<Value> {
    let mut payload = Map::new();
    if let Some(role) = role {
        payload.insert("role".to_string(), json!(role));
    }
    if let Some(perms) = permissions {
        payload.insert("permissions".to_string(), json!(perms));
    }
    if let Some(templates) = templates {
        payload.insert("templates".to_string(), json!(templates));
    }
    if let Some(domains) = domains {
        payload.insert("domains".to_string(), json!(domains));
    }
    client.put(&format!("/users/{id}"), &Value::Object(payload))
}

fn update(
    ctx: &Ctx,
    id: &str,
    role: Option<String>,
    permissions: Option<Vec<String>>,
    templates: Option<Vec<String>>,
    domains: Option<Vec<String>>,
) -> Result<()> {
    let client = ctx.client()?;

    let body = update_user(
        &client,
        id,
        role.as_deref(),
        permissions.as_deref(),
        templates.as_deref(),
        domains.as_deref(),
    )?;

    if ctx.json {
        return output::json(&body);
    }

    output::success(&format!("User {id} updated successfully."));
    Ok(())
}

fn delete(ctx: &Ctx, id: &str) -> Result<()> {
    let client = ctx.client()?;

    client.delete(&format!("/users/{id}"))?;

    output::success(&format!("User {id} deleted successfully."));
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::ENV_LOCK;
    use crate::util::jpath;
    use serde_json::json;
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
                    let path = req.url().split('?').next().unwrap_or("");
                    let resp_body = if (path == "/users" || path == "/invites")
                        && req.method().as_str() == "GET"
                    {
                        r#"{
                            "data": [
                                {"id": "usr-1", "email": "dev@example.com", "role": "admin"}
                            ],
                            "links": {"next": ""},
                            "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 1}
                        }"#
                    } else {
                        r#"{
                            "data": {"id": "usr-1", "email": "dev@example.com", "role": "admin"}
                        }"#
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
    fn list_requests_users_endpoint() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);

        let items = fetch_users(&client, 0).unwrap();
        assert_eq!(items.len(), 1);
        assert_eq!(jstr(&items[0], "email"), "dev@example.com");

        let reqs = mock.finish();
        assert!(
            reqs.iter().any(|r| r.starts_with("GET /users?")),
            "expected GET /users, got {reqs:?}"
        );
    }

    #[test]
    fn invite_list_requests_invites_endpoint() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);

        let items = fetch_invites(&client, 0).unwrap();
        assert_eq!(items.len(), 1);

        let reqs = mock.finish();
        assert!(
            reqs.iter().any(|r| r.starts_with("GET /invites?")),
            "expected GET /invites, got {reqs:?}"
        );
    }

    #[test]
    fn invite_create_posts_users_with_optional_arrays() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);
        let body = client
            .post(
                "/users",
                &invite_payload(
                    "new@example.com",
                    "viewer",
                    Some(vec!["emails.read".to_string()]),
                    None,
                    Some(vec!["dom-1".to_string()]),
                ),
            )
            .unwrap();
        assert_eq!(jpath(&body, "data.id"), "usr-1");

        let reqs = mock.finish();
        assert!(
            reqs.iter().any(|r| r.starts_with("POST /users ")
                && r.contains(r#""email":"new@example.com""#)
                && r.contains(r#""role":"viewer""#)
                && r.contains(r#""permissions":["emails.read"]"#)
                && r.contains(r#""domains":["dom-1"]"#)
                && !r.contains("templates")),
            "expected POST /users with email/role/permissions/domains body, got {reqs:?}"
        );
    }

    #[test]
    fn invite_payload_omits_empty_optionals() {
        let payload = invite_payload("new@example.com", "viewer", None, None, None);
        assert_eq!(
            payload,
            json!({"email": "new@example.com", "role": "viewer"})
        );
    }

    #[test]
    fn update_puts_only_changed_fields() {
        let (mock, base) = Mock::start();
        let client = client_for(&base);

        update_user(&client, "usr-1", Some("admin"), None, None, None).unwrap();
        update_user(
            &client,
            "usr-1",
            Some("admin"),
            Some(&["emails.read".to_string()]),
            Some(&["tmpl-1".to_string(), "tmpl-2".to_string()]),
            None,
        )
        .unwrap();

        let reqs = mock.finish();
        assert!(
            reqs.iter()
                .any(|r| r == "PUT /users/usr-1 {\"role\":\"admin\"}"),
            "expected PUT /users/usr-1 with role only, got {reqs:?}"
        );
        assert!(
            reqs.iter().any(|r| r.starts_with("PUT /users/usr-1 ")
                && r.contains(r#""role":"admin""#)
                && r.contains(r#""permissions":["emails.read"]"#)
                && r.contains(r#""templates":["tmpl-1","tmpl-2"]"#)),
            "expected PUT /users/usr-1 with role/permissions/templates, got {reqs:?}"
        );
    }

    #[test]
    fn invite_resend_posts_resend_endpoint() {
        let (mock, base) = Mock::start();
        let _guard = ENV_LOCK.lock().unwrap_or_else(|e| e.into_inner());
        std::env::set_var("MAILERSEND_API_BASE_URL", &base);
        std::env::set_var("MAILERSEND_API_TOKEN", "test-token");
        let ctx = Ctx {
            profile: None,
            verbose: false,
            json: false,
        };

        invite_resend(&ctx, "inv-1").unwrap();
        std::env::remove_var("MAILERSEND_API_BASE_URL");
        std::env::remove_var("MAILERSEND_API_TOKEN");

        let reqs = mock.finish();
        assert!(
            reqs.iter().any(|r| r == "POST /invites/inv-1/resend "),
            "expected POST /invites/inv-1/resend, got {reqs:?}"
        );
    }

    #[test]
    fn invite_cancel_deletes_invite() {
        let (mock, base) = Mock::start();
        let _guard = ENV_LOCK.lock().unwrap_or_else(|e| e.into_inner());
        std::env::set_var("MAILERSEND_API_BASE_URL", &base);
        std::env::set_var("MAILERSEND_API_TOKEN", "test-token");
        let ctx = Ctx {
            profile: None,
            verbose: false,
            json: false,
        };

        invite_cancel(&ctx, "inv-1").unwrap();
        std::env::remove_var("MAILERSEND_API_BASE_URL");
        std::env::remove_var("MAILERSEND_API_TOKEN");

        let reqs = mock.finish();
        assert!(
            reqs.iter().any(|r| r == "DELETE /invites/inv-1 "),
            "expected DELETE /invites/inv-1, got {reqs:?}"
        );
    }
}
