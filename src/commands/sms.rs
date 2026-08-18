use anyhow::Result;
use clap::{Args, Subcommand};
use serde_json::{json, Value};

use crate::api::{self, Client};
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::{jstr, parse_date};

#[derive(Args)]
#[command(
    long_about = "Send SMS, manage messages, activity, phone numbers, recipients, inbound routes, and webhooks."
)]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// Send an SMS
    Send {
        /// sender phone number (required)
        #[arg(long, default_value = "")]
        from: String,
        /// recipient phone numbers (required)
        #[arg(long, value_delimiter = ',')]
        to: Option<Vec<String>>,
        /// message text (required)
        #[arg(long, default_value = "")]
        text: String,
    },
    /// Manage SMS messages
    Message {
        #[command(subcommand)]
        command: MessageSub,
    },
    /// View SMS activity
    Activity {
        #[command(subcommand)]
        command: ActivitySub,
    },
    /// Manage SMS phone numbers
    Number {
        #[command(subcommand)]
        command: NumberSub,
    },
    /// Manage SMS recipients
    Recipient {
        #[command(subcommand)]
        command: RecipientSub,
    },
    /// Manage SMS inbound routes
    Inbound {
        #[command(subcommand)]
        command: InboundSub,
    },
    /// Manage SMS webhooks
    Webhook {
        #[command(subcommand)]
        command: WebhookSub,
    },
}

#[derive(Subcommand)]
enum MessageSub {
    /// List SMS messages
    List {
        /// maximum number of messages to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
    },
    /// Get SMS message details
    #[command(arg_required_else_help = true)]
    Get {
        /// message ID
        id: String,
    },
}

#[derive(Subcommand)]
enum ActivitySub {
    /// List SMS activity
    List {
        /// maximum number of items to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by SMS number ID
        #[arg(long, default_value = "")]
        sms_number_id: String,
        /// start date (YYYY-MM-DD or unix timestamp)
        #[arg(long, default_value = "")]
        date_from: String,
        /// end date (YYYY-MM-DD or unix timestamp)
        #[arg(long, default_value = "")]
        date_to: String,
        /// filter by status
        #[arg(long, value_delimiter = ',')]
        status: Option<Vec<String>>,
    },
}

#[derive(Subcommand)]
enum NumberSub {
    /// List SMS phone numbers
    List {
        /// maximum number of numbers to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by paused status
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        paused: Option<bool>,
    },
    /// Get SMS phone number details
    #[command(arg_required_else_help = true)]
    Get {
        /// number ID
        id: String,
    },
    /// Update an SMS phone number
    #[command(arg_required_else_help = true)]
    Update {
        /// number ID
        id: String,
        /// whether the number is paused
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        paused: Option<bool>,
    },
    /// Delete an SMS phone number
    #[command(arg_required_else_help = true)]
    Delete {
        /// number ID
        id: String,
    },
}

#[derive(Subcommand)]
enum RecipientSub {
    /// List SMS recipients
    List {
        /// maximum number of recipients to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by status
        #[arg(long, default_value = "")]
        status: String,
        /// filter by SMS number ID
        #[arg(long, default_value = "")]
        sms_number_id: String,
    },
    /// Get SMS recipient details
    #[command(arg_required_else_help = true)]
    Get {
        /// recipient ID
        id: String,
    },
    /// Update an SMS recipient
    #[command(arg_required_else_help = true)]
    Update {
        /// recipient ID
        id: String,
        /// recipient status (required)
        #[arg(long, default_value = "")]
        status: String,
    },
}

#[derive(Subcommand)]
enum InboundSub {
    /// List SMS inbound routes
    List {
        /// maximum number of routes to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by SMS number ID
        #[arg(long, default_value = "")]
        sms_number_id: String,
        /// filter by enabled status
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        enabled: Option<bool>,
    },
    /// Get SMS inbound route details
    #[command(arg_required_else_help = true)]
    Get {
        /// route ID
        id: String,
    },
    /// Create an SMS inbound route
    Create {
        /// SMS number ID (required)
        #[arg(long, default_value = "")]
        sms_number_id: String,
        /// route name (required)
        #[arg(long, default_value = "")]
        name: String,
        /// forward URL (required)
        #[arg(long, default_value = "")]
        forward_url: String,
        /// filter comparer
        #[arg(long, default_value = "")]
        filter_comparer: String,
        /// filter value
        #[arg(long, default_value = "")]
        filter_value: String,
        /// whether the route is enabled
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        enabled: Option<bool>,
    },
    /// Update an SMS inbound route
    #[command(arg_required_else_help = true)]
    Update {
        /// route ID
        id: String,
        /// SMS number ID
        #[arg(long)]
        sms_number_id: Option<String>,
        /// route name
        #[arg(long)]
        name: Option<String>,
        /// forward URL
        #[arg(long)]
        forward_url: Option<String>,
        /// filter comparer
        #[arg(long)]
        filter_comparer: Option<String>,
        /// filter value
        #[arg(long)]
        filter_value: Option<String>,
        /// whether the route is enabled
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        enabled: Option<bool>,
    },
    /// Delete an SMS inbound route
    #[command(arg_required_else_help = true)]
    Delete {
        /// route ID
        id: String,
    },
}

#[derive(Subcommand)]
enum WebhookSub {
    /// List SMS webhooks
    List {
        /// SMS number ID (required)
        #[arg(long, default_value = "")]
        sms_number_id: String,
    },
    /// Get SMS webhook details
    #[command(arg_required_else_help = true)]
    Get {
        /// webhook ID
        id: String,
    },
    /// Create an SMS webhook
    Create {
        /// SMS number ID (required)
        #[arg(long, default_value = "")]
        sms_number_id: String,
        /// webhook name (required)
        #[arg(long, default_value = "")]
        name: String,
        /// webhook URL (required)
        #[arg(long, default_value = "")]
        url: String,
        /// webhook events (required)
        #[arg(long, value_delimiter = ',')]
        events: Option<Vec<String>>,
        /// whether the webhook is enabled
        #[arg(long, num_args = 0..=1, default_missing_value = "true")]
        enabled: Option<bool>,
    },
    /// Update an SMS webhook
    #[command(arg_required_else_help = true)]
    Update {
        /// webhook ID
        id: String,
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
    },
    /// Delete an SMS webhook
    #[command(arg_required_else_help = true)]
    Delete {
        /// webhook ID
        id: String,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    let client = ctx.client()?;
    match cmd.command {
        Sub::Send { from, to, text } => send(ctx, &client, &from, to, &text),
        Sub::Message { command } => match command {
            MessageSub::List { limit } => message_list(ctx, &client, limit),
            MessageSub::Get { id } => message_get(ctx, &client, &id),
        },
        Sub::Activity { command } => match command {
            ActivitySub::List {
                limit,
                sms_number_id,
                date_from,
                date_to,
                status,
            } => activity_list(
                ctx,
                &client,
                limit,
                &sms_number_id,
                &date_from,
                &date_to,
                status,
            ),
        },
        Sub::Number { command } => match command {
            NumberSub::List { limit, paused } => number_list(ctx, &client, limit, paused),
            NumberSub::Get { id } => number_get(ctx, &client, &id),
            NumberSub::Update { id, paused } => number_update(ctx, &client, &id, paused),
            NumberSub::Delete { id } => number_delete(ctx, &client, &id),
        },
        Sub::Recipient { command } => match command {
            RecipientSub::List {
                limit,
                status,
                sms_number_id,
            } => recipient_list(ctx, &client, limit, &status, &sms_number_id),
            RecipientSub::Get { id } => recipient_get(ctx, &client, &id),
            RecipientSub::Update { id, status } => recipient_update(ctx, &client, &id, &status),
        },
        Sub::Inbound { command } => match command {
            InboundSub::List {
                limit,
                sms_number_id,
                enabled,
            } => inbound_list(ctx, &client, limit, &sms_number_id, enabled),
            InboundSub::Get { id } => inbound_get(ctx, &client, &id),
            InboundSub::Create {
                sms_number_id,
                name,
                forward_url,
                filter_comparer,
                filter_value,
                enabled,
            } => inbound_create(
                ctx,
                &client,
                InboundCreate {
                    sms_number_id: &sms_number_id,
                    name: &name,
                    forward_url: &forward_url,
                    filter_comparer: &filter_comparer,
                    filter_value: &filter_value,
                    enabled,
                },
            ),
            InboundSub::Update {
                id,
                sms_number_id,
                name,
                forward_url,
                filter_comparer,
                filter_value,
                enabled,
            } => inbound_update(
                ctx,
                &client,
                &id,
                InboundUpdate {
                    sms_number_id,
                    name,
                    forward_url,
                    filter_comparer,
                    filter_value,
                    enabled,
                },
            ),
            InboundSub::Delete { id } => inbound_delete(ctx, &client, &id),
        },
        Sub::Webhook { command } => match command {
            WebhookSub::List { sms_number_id } => webhook_list(ctx, &client, &sms_number_id),
            WebhookSub::Get { id } => webhook_get(ctx, &client, &id),
            WebhookSub::Create {
                sms_number_id,
                name,
                url,
                events,
                enabled,
            } => webhook_create(ctx, &client, &sms_number_id, &name, &url, events, enabled),
            WebhookSub::Update {
                id,
                name,
                url,
                events,
                enabled,
            } => webhook_update(ctx, &client, &id, name, url, events, enabled),
            WebhookSub::Delete { id } => webhook_delete(ctx, &client, &id),
        },
    }
}

fn bool_yes_no(b: bool) -> String {
    if b { "Yes" } else { "No" }.to_string()
}

fn jbool(v: &Value, key: &str) -> bool {
    v.get(key).and_then(Value::as_bool).unwrap_or(false)
}

fn jlist(v: &Value, key: &str) -> String {
    v.get(key)
        .and_then(Value::as_array)
        .map(|a| {
            a.iter()
                .filter_map(Value::as_str)
                .collect::<Vec<_>>()
                .join(", ")
        })
        .unwrap_or_default()
}

fn fmt_time(s: &str) -> String {
    chrono::DateTime::parse_from_rfc3339(s)
        .map(|d| d.format("%Y-%m-%d %H:%M:%S").to_string())
        .unwrap_or_else(|_| s.to_string())
}

fn send(ctx: &Ctx, client: &Client, from: &str, to: Option<Vec<String>>, text: &str) -> Result<()> {
    let from = prompt::require_arg(from, "from", "Sender phone number")?;
    let to = prompt::require_slice_arg(&to.unwrap_or_default(), "to", "Recipient phone numbers")?;
    let text = prompt::require_arg(text, "text", "Message text")?;

    let body = json!({ "from": from, "to": to, "text": text });
    client.post("/sms", &body)?;

    if ctx.json {
        return output::json(&json!({ "status": "sent" }));
    }

    output::success("SMS sent successfully.");
    Ok(())
}

fn message_list(ctx: &Ctx, client: &Client, limit: u64) -> Result<()> {
    let items = api::fetch_all_paged(client, "/sms-messages", &[], limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|m| {
            vec![
                jstr(m, "id"),
                jstr(m, "from"),
                jlist(m, "to"),
                fmt_time(&jstr(m, "created_at")),
            ]
        })
        .collect();

    output::table(&["ID", "FROM", "TO", "CREATED AT"], &rows);
    Ok(())
}

fn message_get(ctx: &Ctx, client: &Client, id: &str) -> Result<()> {
    let body = client.get(&format!("/sms-messages/{id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let rows = vec![
        vec!["ID".to_string(), jstr(&d, "id")],
        vec!["From".to_string(), jstr(&d, "from")],
        vec!["To".to_string(), jlist(&d, "to")],
        vec!["Text".to_string(), jstr(&d, "text")],
        vec!["Created At".to_string(), fmt_time(&jstr(&d, "created_at"))],
    ];

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn activity_list(
    ctx: &Ctx,
    client: &Client,
    limit: u64,
    sms_number_id: &str,
    date_from: &str,
    date_to: &str,
    status: Option<Vec<String>>,
) -> Result<()> {
    let mut query: Vec<(&str, String)> = Vec::new();
    if !sms_number_id.is_empty() {
        query.push(("sms_number_id", sms_number_id.to_string()));
    }
    if let Some(statuses) = &status {
        for s in statuses {
            query.push(("status[]", s.clone()));
        }
    }
    let mut date_from_ts = 0i64;
    if !date_from.is_empty() {
        date_from_ts = parse_date(date_from)?;
    }
    let mut date_to_ts = 0i64;
    if !date_to.is_empty() {
        date_to_ts = parse_date(date_to)?;
    }
    if date_from_ts > 0 {
        query.push(("date_from", date_from_ts.to_string()));
    }
    if date_to_ts > 0 {
        query.push(("date_to", date_to_ts.to_string()));
    }

    let items = api::fetch_all_paged(client, "/sms-activity", &query, limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|a| {
            vec![
                jstr(a, "sms_message_id"),
                jstr(a, "from"),
                jstr(a, "to"),
                jstr(a, "status"),
                fmt_time(&jstr(a, "created_at")),
            ]
        })
        .collect();

    output::table(&["ID", "FROM", "TO", "STATUS", "CREATED AT"], &rows);
    Ok(())
}

fn number_list(ctx: &Ctx, client: &Client, limit: u64, paused: Option<bool>) -> Result<()> {
    let mut query: Vec<(&str, String)> = Vec::new();
    if paused == Some(true) {
        query.push(("paused", "true".to_string()));
    }

    let items = api::fetch_all_paged(client, "/sms-numbers", &query, limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|n| {
            vec![
                jstr(n, "id"),
                jstr(n, "telephone_number"),
                bool_yes_no(jbool(n, "paused")),
                fmt_time(&jstr(n, "created_at")),
            ]
        })
        .collect();

    output::table(&["ID", "NUMBER", "PAUSED", "CREATED AT"], &rows);
    Ok(())
}

fn number_get(ctx: &Ctx, client: &Client, id: &str) -> Result<()> {
    let body = client.get(&format!("/sms-numbers/{id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let rows = vec![
        vec!["ID".to_string(), jstr(&d, "id")],
        vec!["Number".to_string(), jstr(&d, "telephone_number")],
        vec!["Paused".to_string(), bool_yes_no(jbool(&d, "paused"))],
        vec!["Created At".to_string(), fmt_time(&jstr(&d, "created_at"))],
    ];

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn number_update(ctx: &Ctx, client: &Client, id: &str, paused: Option<bool>) -> Result<()> {
    let mut body = json!({});
    if let Some(v) = paused {
        body["paused"] = json!(v);
    }

    let result = client.put(&format!("/sms-numbers/{id}"), &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!("SMS number {id} updated successfully."));
    Ok(())
}

fn number_delete(_ctx: &Ctx, client: &Client, id: &str) -> Result<()> {
    client.delete(&format!("/sms-numbers/{id}"))?;
    output::success(&format!("SMS number {id} deleted successfully."));
    Ok(())
}

fn recipient_list(
    ctx: &Ctx,
    client: &Client,
    limit: u64,
    _status: &str,
    sms_number_id: &str,
) -> Result<()> {
    // The API expects status as a bool; the --status flag is kept for CLI
    // compatibility and the sms-number-id filter is passed through.
    let mut query: Vec<(&str, String)> = Vec::new();
    if !sms_number_id.is_empty() {
        query.push(("sms_number_id", sms_number_id.to_string()));
    }

    let items = api::fetch_all_paged(client, "/sms-recipients", &query, limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|r| {
            vec![
                jstr(r, "id"),
                jstr(r, "number"),
                jstr(r, "status"),
                fmt_time(&jstr(r, "created_at")),
            ]
        })
        .collect();

    output::table(&["ID", "NUMBER", "STATUS", "CREATED AT"], &rows);
    Ok(())
}

fn recipient_get(ctx: &Ctx, client: &Client, id: &str) -> Result<()> {
    let body = client.get(&format!("/sms-recipients/{id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let rows = vec![
        vec!["ID".to_string(), jstr(&d, "id")],
        vec!["Number".to_string(), jstr(&d, "number")],
        vec!["Status".to_string(), jstr(&d, "status")],
    ];

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn recipient_update(ctx: &Ctx, client: &Client, id: &str, status: &str) -> Result<()> {
    let status = prompt::require_arg(status, "status", "Recipient status")?;

    let result = client.put(
        &format!("/sms-recipients/{id}"),
        &json!({ "status": status }),
    )?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!("SMS recipient {id} updated successfully."));
    Ok(())
}

fn inbound_list(
    ctx: &Ctx,
    client: &Client,
    limit: u64,
    sms_number_id: &str,
    enabled: Option<bool>,
) -> Result<()> {
    let mut query: Vec<(&str, String)> = Vec::new();
    if !sms_number_id.is_empty() {
        query.push(("sms_number_id", sms_number_id.to_string()));
    }
    if let Some(v) = enabled {
        query.push(("enabled", v.to_string()));
    }

    let items = api::fetch_all_paged(client, "/sms-inbounds", &query, limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|r| {
            vec![
                jstr(r, "id"),
                jstr(r, "name"),
                bool_yes_no(jbool(r, "enabled")),
            ]
        })
        .collect();

    output::table(&["ID", "NAME", "ENABLED"], &rows);
    Ok(())
}

fn inbound_get(ctx: &Ctx, client: &Client, id: &str) -> Result<()> {
    let body = client.get(&format!("/sms-inbounds/{id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let rows = vec![
        vec!["ID".to_string(), jstr(&d, "id")],
        vec!["Name".to_string(), jstr(&d, "name")],
        vec!["Forward URL".to_string(), jstr(&d, "forward_url")],
        vec!["Enabled".to_string(), bool_yes_no(jbool(&d, "enabled"))],
    ];

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

struct InboundCreate<'a> {
    sms_number_id: &'a str,
    name: &'a str,
    forward_url: &'a str,
    filter_comparer: &'a str,
    filter_value: &'a str,
    enabled: Option<bool>,
}

struct InboundUpdate {
    sms_number_id: Option<String>,
    name: Option<String>,
    forward_url: Option<String>,
    filter_comparer: Option<String>,
    filter_value: Option<String>,
    enabled: Option<bool>,
}

fn inbound_create(ctx: &Ctx, client: &Client, inbound: InboundCreate<'_>) -> Result<()> {
    let InboundCreate {
        sms_number_id,
        name,
        forward_url,
        filter_comparer,
        filter_value,
        enabled,
    } = inbound;
    let sms_number_id = prompt::require_arg(sms_number_id, "sms-number-id", "SMS number ID")?;
    let name = prompt::require_arg(name, "name", "Route name")?;
    let forward_url = prompt::require_arg(forward_url, "forward-url", "Forward URL")?;

    let body = json!({
        "sms_number_id": sms_number_id,
        "name": name,
        "forward_url": forward_url,
        "filter": { "comparer": filter_comparer, "value": filter_value },
        "enabled": enabled.unwrap_or(true),
    });

    let result = client.post("/sms-inbounds", &body)?;

    if ctx.json {
        return output::json(&result);
    }

    let d = result.get("data").cloned().unwrap_or(Value::Null);
    output::success(&format!(
        "SMS inbound route created successfully. ID: {}",
        jstr(&d, "id")
    ));
    Ok(())
}

fn inbound_update(ctx: &Ctx, client: &Client, id: &str, update: InboundUpdate) -> Result<()> {
    let InboundUpdate {
        sms_number_id,
        name,
        forward_url,
        filter_comparer,
        filter_value,
        enabled,
    } = update;
    let mut body = json!({});
    if let Some(v) = sms_number_id {
        body["sms_number_id"] = json!(v);
    }
    if let Some(v) = name {
        body["name"] = json!(v);
    }
    if let Some(v) = forward_url {
        body["forward_url"] = json!(v);
    }
    body["filter"] = json!({
        "comparer": filter_comparer.unwrap_or_default(),
        "value": filter_value.unwrap_or_default(),
    });
    if let Some(v) = enabled {
        body["enabled"] = json!(v);
    }

    let result = client.put(&format!("/sms-inbounds/{id}"), &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!("SMS inbound route {id} updated successfully."));
    Ok(())
}

fn inbound_delete(_ctx: &Ctx, client: &Client, id: &str) -> Result<()> {
    client.delete(&format!("/sms-inbounds/{id}"))?;
    output::success(&format!("SMS inbound route {id} deleted successfully."));
    Ok(())
}

fn webhook_list(ctx: &Ctx, client: &Client, sms_number_id: &str) -> Result<()> {
    let sms_number_id = prompt::require_arg(sms_number_id, "sms-number-id", "SMS number ID")?;

    let result = client.get_with("/sms-webhooks", &[("sms_number_id", sms_number_id)])?;

    if ctx.json {
        return output::json(&result);
    }

    let items = result
        .get("data")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|w| {
            vec![
                jstr(w, "id"),
                jstr(w, "name"),
                output::truncate(&jstr(w, "url"), 50),
                bool_yes_no(jbool(w, "enabled")),
            ]
        })
        .collect();

    output::table(&["ID", "NAME", "URL", "ENABLED"], &rows);
    Ok(())
}

fn webhook_get(ctx: &Ctx, client: &Client, id: &str) -> Result<()> {
    let body = client.get(&format!("/sms-webhooks/{id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let rows = vec![
        vec!["ID".to_string(), jstr(&d, "id")],
        vec!["Name".to_string(), jstr(&d, "name")],
        vec!["URL".to_string(), jstr(&d, "url")],
        vec!["Enabled".to_string(), bool_yes_no(jbool(&d, "enabled"))],
    ];

    output::table(&["FIELD", "VALUE"], &rows);

    let events = d
        .get("events")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();
    if !events.is_empty() {
        println!("\nEvents:");
        for e in &events {
            if let Some(e) = e.as_str() {
                println!("  - {e}");
            }
        }
    }
    Ok(())
}

fn webhook_create(
    ctx: &Ctx,
    client: &Client,
    sms_number_id: &str,
    name: &str,
    url: &str,
    events: Option<Vec<String>>,
    enabled: Option<bool>,
) -> Result<()> {
    let sms_number_id = prompt::require_arg(sms_number_id, "sms-number-id", "SMS number ID")?;
    let name = prompt::require_arg(name, "name", "Webhook name")?;
    let url = prompt::require_arg(url, "url", "Webhook URL")?;
    let events =
        prompt::require_slice_arg(&events.unwrap_or_default(), "events", "Webhook events")?;

    let body = json!({
        "sms_number_id": sms_number_id,
        "name": name,
        "url": url,
        "events": events,
        "enabled": enabled.unwrap_or(true),
    });

    let result = client.post("/sms-webhooks", &body)?;

    if ctx.json {
        return output::json(&result);
    }

    let d = result.get("data").cloned().unwrap_or(Value::Null);
    output::success(&format!(
        "SMS webhook created successfully. ID: {}",
        jstr(&d, "id")
    ));
    Ok(())
}

fn webhook_update(
    ctx: &Ctx,
    client: &Client,
    id: &str,
    name: Option<String>,
    url: Option<String>,
    events: Option<Vec<String>>,
    enabled: Option<bool>,
) -> Result<()> {
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

    let result = client.put(&format!("/sms-webhooks/{id}"), &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!("SMS webhook {id} updated successfully."));
    Ok(())
}

fn webhook_delete(_ctx: &Ctx, client: &Client, id: &str) -> Result<()> {
    client.delete(&format!("/sms-webhooks/{id}"))?;
    output::success(&format!("SMS webhook {id} deleted successfully."));
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

    fn activity_response() -> Value {
        json!({
            "data": [],
            "links": {"next": ""},
            "meta": {"current_page": 1, "last_page": 1, "per_page": 25, "total": 0}
        })
    }

    #[test]
    fn activity_list_sends_sms_number_id_query_param() {
        let (client, server) = mock(activity_response());

        activity_list(
            &test_ctx(false),
            &client,
            0,
            "test-number-123",
            "",
            "",
            None,
        )
        .expect("activity list");

        let cap = server.join().expect("server thread");
        assert!(
            cap.url.contains("sms_number_id=test-number-123"),
            "expected sms_number_id in query, got: {}",
            cap.url
        );
    }

    #[test]
    fn activity_list_without_sms_number_id_hits_endpoint() {
        let (client, server) = mock(activity_response());

        activity_list(&test_ctx(false), &client, 0, "", "", "", None).expect("activity list");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.url.split('?').next().unwrap(), "/sms-activity");
    }

    #[test]
    fn inbound_create_sends_enabled_by_default() {
        let (client, server) = mock(json!({
            "data": {
                "id": "inb-1",
                "name": "QA Inbound",
                "forward_url": "https://example.com/hook",
                "enabled": true,
                "created_at": "2024-01-01T00:00:00Z"
            }
        }));

        inbound_create(
            &test_ctx(false),
            &client,
            InboundCreate {
                sms_number_id: "num-123",
                name: "QA Inbound",
                forward_url: "https://example.com/hook",
                filter_comparer: "",
                filter_value: "",
                enabled: Some(true),
            },
        )
        .expect("inbound create");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "POST");
        assert_eq!(cap.url, "/sms-inbounds");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["enabled"], true);
    }

    #[test]
    fn inbound_create_sends_enabled_explicitly_false() {
        let (client, server) = mock(json!({
            "data": {
                "id": "inb-2",
                "name": "Disabled Inbound",
                "forward_url": "https://example.com/hook",
                "enabled": false,
                "created_at": "2024-01-01T00:00:00Z"
            }
        }));

        inbound_create(
            &test_ctx(false),
            &client,
            InboundCreate {
                sms_number_id: "num-123",
                name: "Disabled Inbound",
                forward_url: "https://example.com/hook",
                filter_comparer: "",
                filter_value: "",
                enabled: Some(false),
            },
        )
        .expect("inbound create");

        let cap = server.join().expect("server thread");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["enabled"], false);
    }
}
