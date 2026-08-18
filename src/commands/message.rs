use anyhow::Result;
use clap::{Args, Subcommand};
use serde_json::{json, Value};

use crate::api;
use crate::cli::Ctx;
use crate::output;
use crate::util::{jpath, jstr, resolve_domain_id};

#[derive(Args)]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// List messages
    List {
        /// maximum number of results to return
        #[arg(long, default_value_t = 25)]
        limit: u64,
        /// filter by status (queued|sent|delivered|failed)
        #[arg(long, default_value = "")]
        status: String,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
        /// filter from date (YYYY-MM-DD or unix timestamp)
        #[arg(long, default_value = "")]
        date_from: String,
        /// filter to date (YYYY-MM-DD or unix timestamp)
        #[arg(long, default_value = "")]
        date_to: String,
    },
    /// Get message details
    #[command(arg_required_else_help = true)]
    Get {
        /// message ID
        message_id: String,
    },
    /// Manage scheduled messages
    Scheduled {
        #[command(subcommand)]
        command: ScheduledSub,
    },
}

#[derive(Subcommand)]
enum ScheduledSub {
    /// List scheduled messages
    List {
        /// maximum number of results to return
        #[arg(long, default_value_t = 25)]
        limit: u64,
        /// filter by status (scheduled|sending|sent|error)
        #[arg(long, default_value = "")]
        status: String,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
    },
    /// Get scheduled message details
    #[command(arg_required_else_help = true)]
    Get {
        /// message ID
        message_id: String,
    },
    /// Delete a scheduled message
    #[command(arg_required_else_help = true)]
    Delete {
        /// message ID
        message_id: String,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    match cmd.command {
        Sub::List {
            limit,
            status,
            domain,
            date_from,
            date_to,
        } => list(ctx, limit, &status, &domain, &date_from, &date_to),
        Sub::Get { message_id } => get(ctx, &message_id),
        Sub::Scheduled { command } => match command {
            ScheduledSub::List {
                limit,
                status,
                domain,
            } => scheduled_list(ctx, limit, &status, &domain),
            ScheduledSub::Get { message_id } => scheduled_get(ctx, &message_id),
            ScheduledSub::Delete { message_id } => scheduled_delete(ctx, &message_id),
        },
    }
}

fn fmt_time(s: &str) -> String {
    chrono::DateTime::parse_from_rfc3339(s)
        .map(|d| d.format("%Y-%m-%d %H:%M:%S").to_string())
        .unwrap_or_else(|_| s.to_string())
}

// NOTE: The API's /messages endpoint only supports page and limit. The
// --status, --domain, --date-from, --date-to flags are kept for CLI
// compatibility but are not passed through. This is a known limitation to
// be addressed in a future API update.
fn list(
    ctx: &Ctx,
    limit: u64,
    _status: &str,
    _domain: &str,
    _date_from: &str,
    _date_to: &str,
) -> Result<()> {
    let client = ctx.client()?;

    let items = api::fetch_all_paged(&client, "/messages", &[], limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|item| {
            vec![
                jstr(item, "id"),
                fmt_time(&jstr(item, "created_at")),
                fmt_time(&jstr(item, "updated_at")),
            ]
        })
        .collect();

    output::table(&["ID", "CREATED AT", "UPDATED AT"], &rows);
    Ok(())
}

fn get(ctx: &Ctx, message_id: &str) -> Result<()> {
    let client = ctx.client()?;

    let body = client.get(&format!("/messages/{message_id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = &body["data"];
    let mut rows = vec![
        vec!["ID".to_string(), jstr(d, "id")],
        vec!["Created At".to_string(), fmt_time(&jstr(d, "created_at"))],
        vec!["Updated At".to_string(), fmt_time(&jstr(d, "updated_at"))],
        vec!["Domain".to_string(), jpath(d, "domain.name")],
    ];

    let emails: Vec<Value> = d
        .get("emails")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();

    if let Some(e) = emails.first() {
        rows.push(vec!["Subject".to_string(), jstr(e, "subject")]);
        rows.push(vec!["From".to_string(), jstr(e, "from")]);
        rows.push(vec!["Status".to_string(), jstr(e, "status")]);
    }

    if emails.len() > 1 {
        rows.push(vec!["Email Count".to_string(), emails.len().to_string()]);
    }

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn scheduled_list(ctx: &Ctx, limit: u64, status: &str, domain: &str) -> Result<()> {
    let client = ctx.client()?;

    let mut query: Vec<(&str, String)> = Vec::new();
    if !domain.is_empty() {
        let domain_id = resolve_domain_id(&client, domain)?;
        query.push(("domain_id", domain_id));
    }
    if !status.is_empty() {
        query.push(("status", status.to_string()));
    }

    let items = api::fetch_all_paged(&client, "/message-schedules", &query, limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|item| {
            vec![
                jstr(item, "message_id"),
                output::truncate(&jstr(item, "subject"), 40),
                fmt_time(&jstr(item, "send_at")),
                jstr(item, "status"),
                jstr(item, "created_at"),
            ]
        })
        .collect();

    output::table(
        &["MESSAGE ID", "SUBJECT", "SEND AT", "STATUS", "CREATED AT"],
        &rows,
    );
    Ok(())
}

fn scheduled_get(ctx: &Ctx, message_id: &str) -> Result<()> {
    let client = ctx.client()?;

    let body = client.get(&format!("/message-schedules/{message_id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = &body["data"];

    let status_msg = match d.get("status_message") {
        Some(Value::Null) | None => String::new(),
        Some(Value::String(s)) => s.clone(),
        Some(other) => other.to_string(),
    };

    let rows = vec![
        vec!["Message ID".to_string(), jstr(d, "message_id")],
        vec!["Subject".to_string(), jstr(d, "subject")],
        vec!["Send At".to_string(), fmt_time(&jstr(d, "send_at"))],
        vec!["Status".to_string(), jstr(d, "status")],
        vec!["Status Message".to_string(), status_msg],
        vec!["Created At".to_string(), fmt_time(&jstr(d, "created_at"))],
        vec!["Domain".to_string(), jpath(d, "domain.name")],
        vec!["Domain ID".to_string(), jpath(d, "domain.id")],
        vec!["Related Message ID".to_string(), jpath(d, "message.id")],
    ];

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn scheduled_delete(ctx: &Ctx, message_id: &str) -> Result<()> {
    let client = ctx.client()?;

    client.delete(&format!("/message-schedules/{message_id}"))?;

    if ctx.json {
        return output::json(&json!({"status": "deleted", "message_id": message_id}));
    }

    output::success(&format!(
        "Scheduled message {message_id} deleted successfully."
    ));
    Ok(())
}
