use anyhow::Result;
use clap::{Args, Subcommand};
use serde_json::{json, Value};

use crate::api::{self, Client};
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::{jint, jstr, resolve_domain_id};

#[derive(Args)]
#[command(long_about = "List, view, create, update, and delete inbound routes.")]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// List inbound routes
    List {
        /// maximum number of routes to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
    },
    /// Get inbound route details
    #[command(arg_required_else_help = true)]
    Get {
        /// inbound route ID
        inbound_id: String,
    },
    /// Create an inbound route
    Create {
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// route name (required)
        #[arg(long, default_value = "")]
        name: String,
        /// whether the domain is enabled
        #[arg(long, default_value_t = true)]
        domain_enabled: bool,
        /// inbound domain (required when domain-enabled is true)
        #[arg(long, default_value = "")]
        inbound_domain: String,
        /// inbound priority (required when domain-enabled is true)
        #[arg(long)]
        inbound_priority: Option<i64>,
        /// catch type (catch_recipient, catch_all)
        #[arg(long = "catch-type", default_value = "")]
        _catch_type: String,
        /// catch filter type (required when domain-enabled, e.g. catch_all, catch_recipient)
        #[arg(long, default_value = "")]
        catch_filter_type: String,
        /// match filter type (required, e.g. match_all, match_recipient)
        #[arg(long, default_value = "")]
        match_filter_type: String,
        /// forward URLs as type:value pairs, e.g. 'webhook:https://example.com' (required)
        #[arg(long, value_delimiter = ',')]
        forwards: Option<Vec<String>>,
    },
    /// Update an inbound route
    #[command(arg_required_else_help = true)]
    Update {
        /// inbound route ID
        inbound_id: String,
        /// route name
        #[arg(long)]
        name: Option<String>,
        /// whether the domain is enabled
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        domain_enabled: Option<bool>,
        /// inbound domain
        #[arg(long)]
        inbound_domain: Option<String>,
        /// inbound priority
        #[arg(long)]
        inbound_priority: Option<i64>,
        /// catch type
        #[arg(long = "catch-type", default_value = "")]
        _catch_type: String,
        /// catch filter type
        #[arg(long)]
        catch_filter_type: Option<String>,
        /// match filter type
        #[arg(long)]
        match_filter_type: Option<String>,
        /// forward URLs as type:value pairs, e.g. 'webhook:https://example.com'
        #[arg(long, value_delimiter = ',')]
        forwards: Option<Vec<String>>,
    },
    /// Delete an inbound route
    #[command(arg_required_else_help = true)]
    Delete {
        /// inbound route ID
        inbound_id: String,
    },
}

/// Converts "type:value" strings into forward filter objects. Without a
/// colon (or for http(s) URLs) the type defaults to "webhook".
fn parse_forwards(raw: &[String]) -> Vec<Value> {
    raw.iter()
        .map(|s| match s.find(':') {
            Some(idx) if idx > 0 && !s.starts_with("http") => {
                json!({"type": &s[..idx], "value": &s[idx + 1..]})
            }
            _ => json!({"type": "webhook", "value": s}),
        })
        .collect()
}

fn jbool(v: &Value, key: &str) -> bool {
    v.get(key).and_then(Value::as_bool).unwrap_or(false)
}

fn yes_no(b: bool) -> String {
    if b { "Yes" } else { "No" }.to_string()
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    let client = ctx.client()?;
    match cmd.command {
        Sub::List { limit, domain } => list(ctx, &client, limit, &domain),
        Sub::Get { inbound_id } => get(ctx, &client, &inbound_id),
        Sub::Create {
            domain,
            name,
            domain_enabled,
            inbound_domain,
            inbound_priority,
            _catch_type: _,
            catch_filter_type,
            match_filter_type,
            forwards,
        } => create(
            ctx,
            &client,
            &domain,
            &name,
            domain_enabled,
            &inbound_domain,
            inbound_priority,
            &catch_filter_type,
            &match_filter_type,
            forwards.unwrap_or_default(),
        ),
        Sub::Update {
            inbound_id,
            name,
            domain_enabled,
            inbound_domain,
            inbound_priority,
            _catch_type: _,
            catch_filter_type,
            match_filter_type,
            forwards,
        } => update(
            ctx,
            &client,
            &inbound_id,
            name,
            domain_enabled,
            inbound_domain,
            inbound_priority,
            catch_filter_type,
            match_filter_type,
            forwards,
        ),
        Sub::Delete { inbound_id } => delete(ctx, &client, &inbound_id),
    }
}

fn list(ctx: &Ctx, client: &Client, limit: u64, domain: &str) -> Result<()> {
    let domain = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(client, &domain)?;

    let items = api::fetch_all_paged(client, "/inbound", &[("domain_id", domain_id)], limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|i| vec![jstr(i, "id"), jstr(i, "name")])
        .collect();

    output::table(&["ID", "NAME"], &rows);
    Ok(())
}

fn get(ctx: &Ctx, client: &Client, inbound_id: &str) -> Result<()> {
    let body = client.get(&format!("/inbound/{inbound_id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let rows = vec![
        vec!["ID".to_string(), jstr(&d, "id")],
        vec!["Name".to_string(), jstr(&d, "name")],
        vec!["Domain Enabled".to_string(), yes_no(jbool(&d, "enabled"))],
        vec!["Inbound Domain".to_string(), jstr(&d, "domain")],
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
    domain_enabled: bool,
    inbound_domain: &str,
    inbound_priority: Option<i64>,
    catch_filter_type: &str,
    match_filter_type: &str,
    forwards: Vec<String>,
) -> Result<()> {
    let domain = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(client, &domain)?;
    let name = prompt::require_arg(name, "name", "Route name")?;
    let match_filter_type = prompt::require_arg(
        match_filter_type,
        "match-filter-type",
        "Match filter type (e.g. match_all, match_recipient)",
    )?;
    let forwards =
        prompt::require_slice_arg(&forwards, "forwards", "Forward URLs (type:value pairs)")?;

    let mut priority = 0i64;
    if let Some(v) = inbound_priority {
        if v > 0 {
            priority = v;
        }
    }
    // The API requires inbound_priority when domain_enabled is true; the SDK
    // omits it when 0, so default to 100.
    if domain_enabled && priority == 0 {
        priority = 100;
    }

    let mut body = json!({
        "domain_id": domain_id,
        "name": name,
        "domain_enabled": domain_enabled,
        "match_filter": {"type": match_filter_type},
        "forwards": parse_forwards(&forwards),
    });
    if !inbound_domain.is_empty() {
        body["inbound_domain"] = json!(inbound_domain);
    }
    if priority != 0 {
        body["inbound_priority"] = json!(priority);
    }
    if !catch_filter_type.is_empty() {
        body["catch_filter"] = json!({ "type": catch_filter_type });
    }

    let result = client.post("/inbound", &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!(
        "Inbound route created successfully. ID: {}",
        jstr(&result["data"], "id")
    ));
    Ok(())
}

#[allow(clippy::too_many_arguments)]
fn update(
    ctx: &Ctx,
    client: &Client,
    inbound_id: &str,
    name: Option<String>,
    domain_enabled: Option<bool>,
    inbound_domain: Option<String>,
    inbound_priority: Option<i64>,
    catch_filter_type: Option<String>,
    match_filter_type: Option<String>,
    forwards: Option<Vec<String>>,
) -> Result<()> {
    // Fetch the current route first -- the API requires all fields on PUT.
    let current = client
        .get(&format!("/inbound/{inbound_id}"))
        .map_err(|e| anyhow::anyhow!("failed to fetch current route: {e}"))?;
    let d = current.get("data").cloned().unwrap_or(Value::Null);

    let mut name_v = jstr(&d, "name");
    let mut enabled_v = jbool(&d, "enabled");
    let mut inbound_domain_v = jstr(&d, "domain");
    let mut priority_v = jint(&d, "priority");

    let mut match_type: Option<String> = None;
    let mut catch_type: Option<String> = None;
    if let Some(Value::Array(filters)) = d.get("filters") {
        for f in filters {
            match jstr(f, "type").as_str() {
                "match_all" | "match_sender" | "match_domain" | "match_recipient" => {
                    match_type = Some(jstr(f, "type"))
                }
                "catch_all" | "catch_recipient" => catch_type = Some(jstr(f, "type")),
                _ => {}
            }
        }
    }
    let mut match_type = match_type.unwrap_or_else(|| "match_all".to_string());
    let mut catch_type = catch_type.unwrap_or_else(|| "catch_all".to_string());

    let mut fwds: Vec<Value> = Vec::new();
    if let Some(Value::Array(list)) = d.get("forwards") {
        for fw in list {
            fwds.push(json!({"type": jstr(fw, "type"), "value": jstr(fw, "value")}));
        }
    }

    if let Some(v) = name {
        name_v = v;
    }
    if let Some(v) = domain_enabled {
        enabled_v = v;
    }
    if let Some(v) = inbound_domain {
        inbound_domain_v = v;
    }
    if let Some(v) = inbound_priority {
        priority_v = v;
    }
    if let Some(v) = catch_filter_type {
        catch_type = v;
    }
    if let Some(v) = match_filter_type {
        match_type = v;
    }
    if let Some(v) = forwards {
        fwds = parse_forwards(&v);
    }

    let mut body = json!({
        "name": name_v,
        "domain_enabled": enabled_v,
        "match_filter": {"type": match_type},
        "catch_filter": {"type": catch_type},
        "forwards": fwds,
    });
    if !inbound_domain_v.is_empty() {
        body["inbound_domain"] = json!(inbound_domain_v);
    }
    if priority_v != 0 {
        body["inbound_priority"] = json!(priority_v);
    }

    let result = client.put(&format!("/inbound/{inbound_id}"), &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!("Inbound route {inbound_id} updated successfully."));
    Ok(())
}

fn delete(_ctx: &Ctx, client: &Client, inbound_id: &str) -> Result<()> {
    client.delete(&format!("/inbound/{inbound_id}"))?;
    output::success(&format!("Inbound route {inbound_id} deleted successfully."));
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

    /// Mock answering two sequential requests (e.g. GET current + PUT).
    fn mock2(first: Value, second: Value) -> (Client, JoinHandle<(Captured, Captured)>) {
        let server = tiny_http::Server::http("127.0.0.1:0").expect("bind mock server");
        let addr = format!("http://{}", server.server_addr());
        let handle = std::thread::spawn(move || {
            let mut caps = Vec::new();
            for response in [first, second] {
                let mut req = server.recv().expect("receive request");
                let mut body = String::new();
                req.as_reader()
                    .read_to_string(&mut body)
                    .expect("read body");
                caps.push(Captured {
                    method: req.method().to_string(),
                    url: req.url().to_string(),
                    auth: req
                        .headers()
                        .iter()
                        .find(|h| h.field.equiv("Authorization"))
                        .map(|h| h.value.to_string())
                        .unwrap_or_default(),
                    body,
                });
                let resp = tiny_http::Response::from_string(response.to_string()).with_header(
                    tiny_http::Header::from_bytes(&b"Content-Type"[..], &b"application/json"[..])
                        .expect("header"),
                );
                req.respond(resp).expect("respond");
            }
            let mut iter = caps.into_iter();
            (
                iter.next().expect("first cap"),
                iter.next().expect("second cap"),
            )
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
                {"id": "inbound-1", "name": "Route One"},
                {"id": "inbound-2", "name": "Route Two"}
            ],
            "links": {"first": "", "last": "", "prev": "", "next": ""},
            "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 2}
        })
    }

    #[test]
    fn parse_forwards_splits_type_value() {
        let out = parse_forwards(&[
            "webhook:https://example.com/hook".to_string(),
            "https://plain.example.com".to_string(),
            "email:user@example.com".to_string(),
        ]);
        assert_eq!(out[0]["type"], "webhook");
        assert_eq!(out[0]["value"], "https://example.com/hook");
        assert_eq!(out[1]["type"], "webhook");
        assert_eq!(out[1]["value"], "https://plain.example.com");
        assert_eq!(out[2]["type"], "email");
        assert_eq!(out[2]["value"], "user@example.com");
    }

    #[test]
    fn list_hits_inbound_endpoint() {
        let (client, server) = mock(list_response());

        list(&test_ctx(false), &client, 0, "dom-1").expect("list");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "GET");
        assert_eq!(cap.url.split('?').next().unwrap(), "/inbound");
        assert!(cap.url.contains("domain_id=dom-1"));
        assert_eq!(cap.auth, "Bearer test-token");
    }

    #[test]
    fn get_hits_inbound_endpoint() {
        let (client, server) = mock(json!({
            "data": {
                "id": "inbound-1",
                "name": "Route One",
                "enabled": true,
                "domain": "inbound.example.com"
            }
        }));

        get(&test_ctx(false), &client, "inbound-1").expect("get");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "GET");
        assert_eq!(cap.url, "/inbound/inbound-1");
    }

    #[test]
    fn create_posts_full_body_with_default_priority() {
        let (client, server) = mock(json!({"data": {"id": "inbound-new"}}));

        create(
            &test_ctx(false),
            &client,
            "dom-1",
            "My Route",
            true,
            "inbound.example.com",
            None,
            "catch_all",
            "match_all",
            vec![
                "webhook:https://example.com/hook".to_string(),
                "https://plain.example.com".to_string(),
            ],
        )
        .expect("create");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "POST");
        assert_eq!(cap.url, "/inbound");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["domain_id"], "dom-1");
        assert_eq!(body["name"], "My Route");
        assert_eq!(body["domain_enabled"], true);
        assert_eq!(body["inbound_domain"], "inbound.example.com");
        // domain_enabled defaults the omitted priority to 100.
        assert_eq!(body["inbound_priority"], 100);
        assert_eq!(body["match_filter"]["type"], "match_all");
        assert_eq!(body["catch_filter"]["type"], "catch_all");
        assert_eq!(body["forwards"][0]["type"], "webhook");
        assert_eq!(body["forwards"][0]["value"], "https://example.com/hook");
        assert_eq!(body["forwards"][1]["type"], "webhook");
        assert_eq!(body["forwards"][1]["value"], "https://plain.example.com");
    }

    #[test]
    fn create_without_domain_enabled_omits_priority() {
        let (client, server) = mock(json!({"data": {"id": "inbound-new"}}));

        create(
            &test_ctx(false),
            &client,
            "dom-1",
            "My Route",
            false,
            "",
            None,
            "",
            "match_all",
            vec!["webhook:https://example.com".to_string()],
        )
        .expect("create");

        let cap = server.join().expect("server thread");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["domain_enabled"], false);
        assert_eq!(body["inbound_priority"], Value::Null);
        assert_eq!(body["inbound_domain"], Value::Null);
        assert_eq!(body["catch_filter"], Value::Null);
    }

    #[test]
    fn update_merges_current_route_with_overrides() {
        let current = json!({
            "data": {
                "id": "inbound-1",
                "name": "Old Name",
                "domain": "inbound.example.com",
                "priority": 50,
                "enabled": true,
                "filters": [{"type": "match_recipient"}, {"type": "catch_all"}],
                "forwards": [{"type": "webhook", "value": "https://old.example.com"}]
            }
        });
        let (client, server) = mock2(current, json!({"data": {"id": "inbound-1"}}));

        update(
            &test_ctx(false),
            &client,
            "inbound-1",
            Some("New Name".to_string()),
            None,
            None,
            None,
            None,
            None,
            Some(vec!["webhook:https://new.example.com".to_string()]),
        )
        .expect("update");

        let (get_cap, put_cap) = server.join().expect("server thread");
        assert_eq!(get_cap.method, "GET");
        assert_eq!(get_cap.url, "/inbound/inbound-1");
        assert_eq!(put_cap.method, "PUT");
        assert_eq!(put_cap.url, "/inbound/inbound-1");
        let body: Value = serde_json::from_str(&put_cap.body).expect("json body");
        assert_eq!(body["name"], "New Name");
        assert_eq!(body["domain_enabled"], true);
        assert_eq!(body["inbound_domain"], "inbound.example.com");
        assert_eq!(body["inbound_priority"], 50);
        assert_eq!(body["match_filter"]["type"], "match_recipient");
        assert_eq!(body["catch_filter"]["type"], "catch_all");
        assert_eq!(body["forwards"][0]["value"], "https://new.example.com");
    }

    #[test]
    fn update_defaults_missing_filters() {
        let current = json!({
            "data": {
                "id": "inbound-1",
                "name": "Old Name",
                "domain": "",
                "priority": 0,
                "enabled": false,
                "filters": [],
                "forwards": []
            }
        });
        let (client, server) = mock2(current, json!({"data": {"id": "inbound-1"}}));

        update(
            &test_ctx(false),
            &client,
            "inbound-1",
            None,
            None,
            None,
            None,
            None,
            None,
            None,
        )
        .expect("update");

        let (_, put_cap) = server.join().expect("server thread");
        let body: Value = serde_json::from_str(&put_cap.body).expect("json body");
        assert_eq!(body["match_filter"]["type"], "match_all");
        assert_eq!(body["catch_filter"]["type"], "catch_all");
        assert_eq!(body["inbound_priority"], Value::Null);
        assert_eq!(body["inbound_domain"], Value::Null);
        assert_eq!(body["domain_enabled"], false);
    }

    #[test]
    fn delete_hits_inbound_endpoint() {
        let (client, server) = mock(json!({}));

        delete(&test_ctx(false), &client, "inbound-1").expect("delete");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "DELETE");
        assert_eq!(cap.url, "/inbound/inbound-1");
    }
}
