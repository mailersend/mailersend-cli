use anyhow::{anyhow, Result};
use clap::{Args, Subcommand};
use serde_json::Value;

use crate::api::Client;
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::{jint, jstr};

#[derive(Args)]
#[command(long_about = "Send bulk emails and check bulk email status.")]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// Send bulk email
    Send {
        /// path to JSON file with email array (required)
        #[arg(long, default_value = "")]
        file: String,
    },
    /// Get bulk email status
    #[command(arg_required_else_help = true)]
    Status {
        /// bulk email ID
        bulk_email_id: String,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    let client = ctx.client()?;
    match cmd.command {
        Sub::Send { file } => send(ctx, &client, &file),
        Sub::Status { bulk_email_id } => status(ctx, &client, &bulk_email_id),
    }
}

fn send(ctx: &Ctx, client: &Client, file: &str) -> Result<()> {
    let file = prompt::require_arg(file, "file", "Path to JSON file")?;

    let data = std::fs::read_to_string(&file).map_err(|e| anyhow!("failed to read file: {e}"))?;

    let messages: Vec<Value> =
        serde_json::from_str(&data).map_err(|e| anyhow!("invalid JSON in file: {e}"))?;

    let result = client.post("/bulk-email", &Value::Array(messages))?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(&format!(
        "Bulk email sent. ID: {}",
        jstr(&result, "bulk_email_id")
    ));
    Ok(())
}

fn fmt_time(s: &str) -> String {
    chrono::DateTime::parse_from_rfc3339(s)
        .map(|d| d.format("%Y-%m-%d %H:%M:%S").to_string())
        .unwrap_or_else(|_| s.to_string())
}

fn status(ctx: &Ctx, client: &Client, bulk_email_id: &str) -> Result<()> {
    let body = client.get(&format!("/bulk-email/{bulk_email_id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);
    let messages = d
        .get("messages_id")
        .and_then(Value::as_array)
        .map(|a| {
            a.iter()
                .filter_map(Value::as_str)
                .collect::<Vec<_>>()
                .join(", ")
        })
        .unwrap_or_default();

    let rows = vec![
        vec!["ID".to_string(), jstr(&d, "id")],
        vec!["State".to_string(), jstr(&d, "state")],
        vec![
            "Total Recipients".to_string(),
            jint(&d, "total_recipients_count").to_string(),
        ],
        vec![
            "Suppressed Recipients".to_string(),
            jint(&d, "suppressed_recipients_count").to_string(),
        ],
        vec![
            "Validation Errors".to_string(),
            jint(&d, "validation_errors_count").to_string(),
        ],
        vec!["Messages".to_string(), messages],
        vec!["Created At".to_string(), fmt_time(&jstr(&d, "created_at"))],
        vec!["Updated At".to_string(), fmt_time(&jstr(&d, "updated_at"))],
    ];

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}
