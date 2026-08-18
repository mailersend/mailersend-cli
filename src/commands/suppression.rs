use anyhow::{bail, Result};
use clap::{Args, Subcommand};
use serde_json::{json, Value};

use crate::api;
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::{jpath, jstr, resolve_domain_id};

#[derive(Args)]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// Manage blocklist suppressions
    Blocklist {
        #[command(subcommand)]
        command: BlocklistSub,
    },
    /// Manage hard bounce suppressions
    HardBounces {
        #[command(subcommand)]
        command: HardBouncesSub,
    },
    /// Manage spam complaint suppressions
    SpamComplaints {
        #[command(subcommand)]
        command: SpamComplaintsSub,
    },
    /// Manage unsubscribe suppressions
    Unsubscribes {
        #[command(subcommand)]
        command: UnsubscribesSub,
    },
    /// Manage on-hold list
    OnHold {
        #[command(subcommand)]
        command: OnHoldSub,
    },
}

#[derive(Subcommand)]
enum BlocklistSub {
    /// List blocklist entries
    List {
        /// maximum number of items to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
    },
    /// Add entries to the blocklist
    Add {
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// recipient emails to block
        #[arg(long, value_delimiter = ',')]
        recipients: Option<Vec<String>>,
        /// patterns to block
        #[arg(long, value_delimiter = ',')]
        patterns: Option<Vec<String>>,
    },
    /// Delete blocklist entries
    Delete {
        /// IDs to delete
        #[arg(long, value_delimiter = ',')]
        ids: Option<Vec<String>>,
        /// delete all entries
        #[arg(long)]
        all: bool,
        /// domain name or ID
        #[arg(long)]
        domain: Option<String>,
    },
}

#[derive(Subcommand)]
enum HardBouncesSub {
    /// List hard bounce entries
    List {
        /// maximum number of items to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
    },
    /// Add hard bounce entries
    Add {
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// recipient emails
        #[arg(long, value_delimiter = ',')]
        recipients: Option<Vec<String>>,
    },
    /// Delete hard bounce entries
    Delete {
        /// IDs to delete
        #[arg(long, value_delimiter = ',')]
        ids: Option<Vec<String>>,
        /// delete all entries
        #[arg(long)]
        all: bool,
        /// domain name or ID
        #[arg(long)]
        domain: Option<String>,
    },
}

#[derive(Subcommand)]
enum SpamComplaintsSub {
    /// List spam complaint entries
    List {
        /// maximum number of items to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
    },
    /// Add spam complaint entries
    Add {
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// recipient emails
        #[arg(long, value_delimiter = ',')]
        recipients: Option<Vec<String>>,
    },
    /// Delete spam complaint entries
    Delete {
        /// IDs to delete
        #[arg(long, value_delimiter = ',')]
        ids: Option<Vec<String>>,
        /// delete all entries
        #[arg(long)]
        all: bool,
        /// domain name or ID
        #[arg(long)]
        domain: Option<String>,
    },
}

#[derive(Subcommand)]
enum UnsubscribesSub {
    /// List unsubscribe entries
    List {
        /// maximum number of items to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
    },
    /// Add unsubscribe entries
    Add {
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// recipient emails
        #[arg(long, value_delimiter = ',')]
        recipients: Option<Vec<String>>,
    },
    /// Delete unsubscribe entries
    Delete {
        /// IDs to delete
        #[arg(long, value_delimiter = ',')]
        ids: Option<Vec<String>>,
        /// delete all entries
        #[arg(long)]
        all: bool,
        /// domain name or ID
        #[arg(long)]
        domain: Option<String>,
    },
}

#[derive(Subcommand)]
enum OnHoldSub {
    /// List on-hold entries
    List {
        /// maximum number of items to return (0 = all)
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
    },
    /// Delete on-hold entries
    Delete {
        /// IDs to delete
        #[arg(long, value_delimiter = ',')]
        ids: Option<Vec<String>>,
        /// delete all entries
        #[arg(long)]
        all: bool,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    match cmd.command {
        Sub::Blocklist { command } => match command {
            BlocklistSub::List { limit, domain } => list(ctx, "blocklist", true, limit, &domain),
            BlocklistSub::Add {
                domain,
                recipients,
                patterns,
            } => blocklist_add(ctx, &domain, recipients, patterns),
            BlocklistSub::Delete { ids, all, domain } => delete(ctx, "blocklist", ids, all, domain),
        },
        Sub::HardBounces { command } => match command {
            HardBouncesSub::List { limit, domain } => {
                list(ctx, "hard-bounces", false, limit, &domain)
            }
            HardBouncesSub::Add { domain, recipients } => add(
                ctx,
                "hard-bounces",
                &domain,
                recipients,
                "Hard bounce entries added successfully.",
            ),
            HardBouncesSub::Delete { ids, all, domain } => {
                delete(ctx, "hard-bounces", ids, all, domain)
            }
        },
        Sub::SpamComplaints { command } => match command {
            SpamComplaintsSub::List { limit, domain } => {
                list(ctx, "spam-complaints", false, limit, &domain)
            }
            SpamComplaintsSub::Add { domain, recipients } => add(
                ctx,
                "spam-complaints",
                &domain,
                recipients,
                "Spam complaint entries added successfully.",
            ),
            SpamComplaintsSub::Delete { ids, all, domain } => {
                delete(ctx, "spam-complaints", ids, all, domain)
            }
        },
        Sub::Unsubscribes { command } => match command {
            UnsubscribesSub::List { limit, domain } => {
                list(ctx, "unsubscribes", false, limit, &domain)
            }
            UnsubscribesSub::Add { domain, recipients } => add(
                ctx,
                "unsubscribes",
                &domain,
                recipients,
                "Unsubscribe entries added successfully.",
            ),
            UnsubscribesSub::Delete { ids, all, domain } => {
                delete(ctx, "unsubscribes", ids, all, domain)
            }
        },
        Sub::OnHold { command } => match command {
            OnHoldSub::List { limit, domain } => on_hold_list(ctx, limit, &domain),
            OnHoldSub::Delete { ids, all } => on_hold_delete(ctx, ids, all),
        },
    }
}

fn fmt_datetime(s: &str) -> String {
    chrono::DateTime::parse_from_rfc3339(s)
        .map(|d| d.format("%Y-%m-%d %H:%M:%S").to_string())
        .unwrap_or_default()
}

fn list(ctx: &Ctx, stype: &str, blocklist: bool, limit: u64, domain: &str) -> Result<()> {
    let client = ctx.client()?;

    let mut domain_id = domain.to_string();
    if !domain_id.is_empty() {
        domain_id = resolve_domain_id(&client, &domain_id)?;
    }

    let mut query: Vec<(&str, String)> = Vec::new();
    if !domain_id.is_empty() {
        query.push(("domain_id", domain_id));
    }

    let data = api::fetch_all_paged(&client, &format!("/suppressions/{stype}"), &query, limit)?;

    let items: Vec<Value> = data
        .iter()
        .map(|d| {
            if blocklist {
                json!({
                    "ID": jstr(d, "id"),
                    "Type": jstr(d, "type"),
                    "PatternEmail": jstr(d, "pattern"),
                    "CreatedAt": fmt_datetime(&jstr(d, "created_at")),
                })
            } else {
                json!({
                    "ID": jstr(d, "id"),
                    "Type": "",
                    "PatternEmail": jpath(d, "recipient.email"),
                    "CreatedAt": fmt_datetime(&jstr(d, "created_at")),
                })
            }
        })
        .collect();

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|i| {
            vec![
                jstr(i, "ID"),
                jstr(i, "Type"),
                jstr(i, "PatternEmail"),
                jstr(i, "CreatedAt"),
            ]
        })
        .collect();

    output::table(&["ID", "TYPE", "PATTERN/EMAIL", "CREATED AT"], &rows);
    Ok(())
}

fn blocklist_add(
    ctx: &Ctx,
    domain: &str,
    recipients: Option<Vec<String>>,
    patterns: Option<Vec<String>>,
) -> Result<()> {
    let client = ctx.client()?;

    let domain_id = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(&client, &domain_id)?;

    let mut body = json!({ "domain_id": domain_id });
    if let Some(recipients) = recipients.filter(|r| !r.is_empty()) {
        body["recipients"] = json!(recipients);
    }
    if let Some(patterns) = patterns.filter(|p| !p.is_empty()) {
        body["patterns"] = json!(patterns);
    }

    let result = client.post("/suppressions/blocklist", &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success("Blocklist entries added successfully.");
    Ok(())
}

fn add(
    ctx: &Ctx,
    stype: &str,
    domain: &str,
    recipients: Option<Vec<String>>,
    ok_msg: &str,
) -> Result<()> {
    let client = ctx.client()?;

    let domain_id = prompt::require_arg(domain, "domain", "Domain name or ID")?;
    let domain_id = resolve_domain_id(&client, &domain_id)?;

    let body = json!({
        "domain_id": domain_id,
        "recipients": recipients.unwrap_or_default(),
    });

    let result = client.post(&format!("/suppressions/{stype}"), &body)?;

    if ctx.json {
        return output::json(&result);
    }

    output::success(ok_msg);
    Ok(())
}

fn delete(
    ctx: &Ctx,
    stype: &str,
    ids: Option<Vec<String>>,
    all: bool,
    domain: Option<String>,
) -> Result<()> {
    let client = ctx.client()?;

    let ids = ids.unwrap_or_default();
    if ids.is_empty() && !all {
        bail!("provide --ids or --all");
    }

    let mut domain_id = String::new();
    if let Some(domain) = domain {
        domain_id = resolve_domain_id(&client, &domain)?;
    }

    let body = if all {
        json!({ "domain_id": domain_id, "all": true })
    } else {
        json!({ "domain_id": domain_id, "ids": ids })
    };

    client.request(
        "DELETE",
        &format!("/suppressions/{stype}"),
        &[],
        Some(&body),
    )?;

    output::success("Suppression entries deleted successfully.");
    Ok(())
}

fn on_hold_list(ctx: &Ctx, limit: u64, domain: &str) -> Result<()> {
    let client = ctx.client()?;

    let mut domain_id = domain.to_string();
    if !domain_id.is_empty() {
        domain_id = resolve_domain_id(&client, &domain_id)?;
    }

    let mut query: Vec<(&str, String)> = Vec::new();
    if !domain_id.is_empty() {
        query.push(("domain_id", domain_id));
    }

    let items = api::fetch_all_paged(&client, "/suppressions/on-hold-list", &query, limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|i| {
            let mut value = jstr(i, "pattern");
            if value.is_empty() {
                value = jpath(i, "recipient.email");
            }
            vec![jstr(i, "id"), jstr(i, "type"), value, jstr(i, "created_at")]
        })
        .collect();

    output::table(&["ID", "TYPE", "PATTERN/EMAIL", "CREATED AT"], &rows);
    Ok(())
}

fn on_hold_delete(ctx: &Ctx, ids: Option<Vec<String>>, all: bool) -> Result<()> {
    let client = ctx.client()?;

    let mut payload = serde_json::Map::new();
    if let Some(ids) = ids.filter(|i| !i.is_empty()) {
        payload.insert("ids".to_string(), json!(ids));
    }
    if all {
        payload.insert("all".to_string(), json!(true));
    }

    if payload.is_empty() {
        bail!("provide --ids or --all");
    }

    client.request(
        "DELETE",
        "/suppressions/on-hold-list",
        &[],
        Some(&Value::Object(payload)),
    )?;

    output::success("On-hold entries deleted successfully.");
    Ok(())
}
