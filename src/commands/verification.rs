use anyhow::{bail, Context, Result};
use clap::{Args, Subcommand};
use serde_json::{json, Value};

use crate::api;
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::{jint, jpath, jstr};

#[derive(Args)]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// Verify a single email address
    #[command(arg_required_else_help = true)]
    Verify {
        /// email address to verify
        email: String,
    },
    /// Verify a single email address asynchronously
    #[command(arg_required_else_help = true)]
    VerifyAsync {
        /// email address to verify
        email: String,
    },
    /// Get async email verification status
    #[command(arg_required_else_help = true)]
    Status {
        /// async verification ID
        id: String,
    },
    /// Manage verification lists
    #[command(long_about = "List, create, verify, and inspect email verification lists.")]
    List {
        #[command(subcommand)]
        command: ListSub,
    },
}

#[derive(Subcommand)]
enum ListSub {
    /// List all verification lists
    List {
        /// maximum number of lists to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
    },
    /// Get verification list details
    #[command(arg_required_else_help = true)]
    Get {
        /// verification list ID
        id: String,
    },
    /// Create a verification list
    Create {
        /// name for the verification list (required)
        #[arg(long, default_value = "")]
        name: String,
        /// comma-separated list of email addresses
        #[arg(long, value_delimiter = ',')]
        emails: Option<Vec<String>>,
        /// path to file with one email per line
        #[arg(long, default_value = "")]
        emails_file: String,
    },
    /// Start verification of a list
    #[command(arg_required_else_help = true)]
    Verify {
        /// verification list ID
        id: String,
        /// poll until verification completes
        #[arg(long)]
        wait: bool,
    },
    /// Get verification results for a list
    #[command(arg_required_else_help = true)]
    Results {
        /// verification list ID
        id: String,
        /// maximum number of results to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by status (valid, invalid, catch_all, mailbox_full, role, unknown)
        #[arg(long, default_value = "")]
        status: String,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    match cmd.command {
        Sub::Verify { email } => verify(ctx, &email),
        Sub::VerifyAsync { email } => verify_async(ctx, &email),
        Sub::Status { id } => status(ctx, &id),
        Sub::List { command } => match command {
            ListSub::List { limit } => list_list(ctx, limit),
            ListSub::Get { id } => list_get(ctx, &id),
            ListSub::Create {
                name,
                emails,
                emails_file,
            } => list_create(ctx, &name, emails, &emails_file),
            ListSub::Verify { id, wait } => list_verify(ctx, &id, wait),
            ListSub::Results {
                id,
                limit,
                status: _,
            } => list_results(ctx, &id, limit),
        },
    }
}

fn fmt_datetime(s: &str) -> String {
    chrono::DateTime::parse_from_rfc3339(s)
        .map(|d| d.format("%Y-%m-%d %H:%M:%S").to_string())
        .unwrap_or_default()
}

fn fmt_value(v: &Value) -> String {
    match v {
        Value::String(s) => s.clone(),
        other => other.to_string(),
    }
}

fn verify(ctx: &Ctx, email: &str) -> Result<()> {
    let client = ctx.client()?;

    let body = client.post("/email-verification/verify", &json!({ "email": email }))?;

    if ctx.json {
        return output::json(&body);
    }

    let data = body.get("data").cloned().unwrap_or(Value::Null);

    let mut rows = vec![
        vec!["Email".to_string(), email.to_string()],
        vec!["Status".to_string(), jstr(&data, "status")],
    ];

    if let Some(info) = data.get("email").and_then(Value::as_object) {
        for key in ["local_part", "domain", "mx_found", "mx_record"] {
            if let Some(v) = info.get(key) {
                if !v.is_null() {
                    rows.push(vec![key.to_string(), fmt_value(v)]);
                }
            }
        }
    }

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn verify_async(ctx: &Ctx, email: &str) -> Result<()> {
    let client = ctx.client()?;

    let body = client.post(
        "/email-verification/verify-async",
        &json!({ "email": email }),
    )?;

    if ctx.json {
        return output::json(&body);
    }

    let data = body.get("data").cloned().unwrap_or(Value::Null);

    let rows = vec![
        vec!["ID".to_string(), jstr(&data, "id")],
        vec!["Address".to_string(), jstr(&data, "address")],
        vec!["Status".to_string(), jstr(&data, "status")],
    ];

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn status(ctx: &Ctx, id: &str) -> Result<()> {
    let client = ctx.client()?;

    let body = client.get(&format!("/email-verification/verify-async/{id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let data = body.get("data").cloned().unwrap_or(Value::Null);

    let mut rows = vec![
        vec!["ID".to_string(), jstr(&data, "id")],
        vec!["Address".to_string(), jstr(&data, "address")],
        vec!["Status".to_string(), jstr(&data, "status")],
    ];

    if let Some(result) = data.get("result") {
        if !result.is_null() {
            rows.push(vec!["Result".to_string(), result.to_string()]);
        }
    }
    if let Some(error) = data.get("error") {
        if !error.is_null() {
            rows.push(vec!["Error".to_string(), error.to_string()]);
        }
    }

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn list_list(ctx: &Ctx, limit: u64) -> Result<()> {
    let client = ctx.client()?;

    let items = api::fetch_all_paged(&client, "/email-verification", &[], limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|item| {
            vec![
                jstr(item, "id"),
                jstr(item, "name"),
                jint(item, "total").to_string(),
                jpath(item, "status.name"),
                fmt_datetime(&jstr(item, "created_at")),
            ]
        })
        .collect();

    output::table(&["ID", "NAME", "TOTAL", "STATUS", "CREATED AT"], &rows);
    Ok(())
}

fn list_get(ctx: &Ctx, id: &str) -> Result<()> {
    let client = ctx.client()?;

    let body = client.get(&format!("/email-verification/{id}"))?;

    if ctx.json {
        return output::json(&body);
    }

    let d = body.get("data").cloned().unwrap_or(Value::Null);

    let opt_value = |key: &str| -> String {
        match d.get(key) {
            None | Some(Value::Null) => String::new(),
            Some(v) => fmt_value(v),
        }
    };

    let stats = d.get("statistics").cloned().unwrap_or(Value::Null);

    let rows = vec![
        vec!["ID".to_string(), jstr(&d, "id")],
        vec!["Name".to_string(), jstr(&d, "name")],
        vec!["Total".to_string(), jint(&d, "total").to_string()],
        vec!["Status".to_string(), jpath(&d, "status.name")],
        vec!["Source".to_string(), jstr(&d, "source")],
        vec![
            "Verification Started".to_string(),
            opt_value("verification_started"),
        ],
        vec![
            "Verification Ended".to_string(),
            opt_value("verification_ended"),
        ],
        vec![
            "Created At".to_string(),
            fmt_datetime(&jstr(&d, "created_at")),
        ],
        vec![
            "Updated At".to_string(),
            fmt_datetime(&jstr(&d, "updated_at")),
        ],
        vec![String::new(), String::new()],
        vec!["--- Statistics ---".to_string(), String::new()],
        vec!["Valid".to_string(), jint(&stats, "valid").to_string()],
        vec![
            "Catch All".to_string(),
            jint(&stats, "catch_all").to_string(),
        ],
        vec![
            "Mailbox Full".to_string(),
            jint(&stats, "mailbox_full").to_string(),
        ],
        vec![
            "Role Based".to_string(),
            jint(&stats, "role_based").to_string(),
        ],
        vec!["Unknown".to_string(), jint(&stats, "unknown").to_string()],
        vec![
            "Syntax Error".to_string(),
            jint(&stats, "syntax_error").to_string(),
        ],
        vec!["Typo".to_string(), jint(&stats, "typo").to_string()],
        vec![
            "Mailbox Not Found".to_string(),
            jint(&stats, "mailbox_not_found").to_string(),
        ],
        vec![
            "Disposable".to_string(),
            jint(&stats, "disposable").to_string(),
        ],
        vec![
            "Mailbox Blocked".to_string(),
            jint(&stats, "mailbox_blocked").to_string(),
        ],
        vec!["Failed".to_string(), jint(&stats, "failed").to_string()],
    ];

    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}

fn list_create(
    ctx: &Ctx,
    name: &str,
    emails: Option<Vec<String>>,
    emails_file: &str,
) -> Result<()> {
    let client = ctx.client()?;

    let name = prompt::require_arg(name, "name", "Verification list name")?;

    let mut emails = emails.unwrap_or_default();
    if !emails_file.is_empty() {
        let data = std::fs::read_to_string(emails_file).context("failed to read emails file")?;
        for line in data.lines() {
            let trimmed = line.trim();
            if !trimmed.is_empty() {
                emails.push(trimmed.to_string());
            }
        }
    }

    if emails.is_empty() {
        bail!("provide emails via --emails or --emails-file");
    }

    let result = client.post(
        "/email-verification",
        &json!({ "name": name, "emails": emails }),
    )?;

    if ctx.json {
        return output::json(&result);
    }

    let data = result.get("data").cloned().unwrap_or(Value::Null);
    output::success(&format!(
        "Verification list created: {} (ID: {})",
        jstr(&data, "name"),
        jstr(&data, "id")
    ));
    Ok(())
}

fn list_verify(ctx: &Ctx, id: &str, wait: bool) -> Result<()> {
    let client = ctx.client()?;

    let result = client.get(&format!("/email-verification/{id}/verify"))?;

    if !wait {
        if ctx.json {
            return output::json(&result);
        }

        output::success(&format!("Verification started for list {id}."));
        return Ok(());
    }

    loop {
        std::thread::sleep(std::time::Duration::from_secs(5));

        let poll = client.get(&format!("/email-verification/{id}"))?;
        let status_name = jpath(&poll, "data.status.name");

        println!("Waiting... (status: {status_name})");

        if status_name == "verified" || status_name == "failed" {
            if ctx.json {
                return output::json(&poll);
            }

            if status_name == "verified" {
                output::success(&format!("Verification completed for list {id}."));
            } else {
                output::error(&format!("Verification failed for list {id}."));
            }
            return Ok(());
        }
    }
}

fn list_results(ctx: &Ctx, id: &str, limit: u64) -> Result<()> {
    let client = ctx.client()?;

    let items = api::fetch_all_paged(
        &client,
        &format!("/email-verification/{id}/results"),
        &[],
        limit,
    )?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|item| vec![jstr(item, "address"), jstr(item, "result"), String::new()])
        .collect();

    output::table(&["EMAIL", "RESULT", "REASON"], &rows);
    Ok(())
}
