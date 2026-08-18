use anyhow::{bail, Result};
use clap::{Args, Subcommand};
use serde_json::Value;

use crate::cli::Ctx;
use crate::output;
use crate::util::{default_date_range, jint, jstr, resolve_domain_id};

#[derive(Args)]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// Get analytics grouped by date
    Date {
        /// start date as YYYY-MM-DD or unix timestamp (required)
        #[arg(long, default_value = "")]
        date_from: String,
        /// end date as YYYY-MM-DD or unix timestamp (required)
        #[arg(long, default_value = "")]
        date_to: String,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
        /// group by: days, weeks, months, years
        #[arg(long, default_value = "")]
        group_by: String,
        /// filter by tags
        #[arg(long, value_delimiter = ',')]
        tags: Vec<String>,
        /// event types to retrieve (required, min 1): queued, sent, delivered, soft_bounced, hard_bounced, opened, clicked, unsubscribed, spam_complaints
        #[arg(long, value_delimiter = ',')]
        event: Vec<String>,
    },
    /// Get analytics grouped by country
    Country {
        /// start date as YYYY-MM-DD or unix timestamp (required)
        #[arg(long, default_value = "")]
        date_from: String,
        /// end date as YYYY-MM-DD or unix timestamp (required)
        #[arg(long, default_value = "")]
        date_to: String,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
        /// filter by tags
        #[arg(long, value_delimiter = ',')]
        tags: Vec<String>,
    },
    /// Get analytics grouped by user agent name
    UaName {
        /// start date as YYYY-MM-DD or unix timestamp (required)
        #[arg(long, default_value = "")]
        date_from: String,
        /// end date as YYYY-MM-DD or unix timestamp (required)
        #[arg(long, default_value = "")]
        date_to: String,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
        /// filter by tags
        #[arg(long, value_delimiter = ',')]
        tags: Vec<String>,
    },
    /// Get analytics grouped by user agent type
    UaType {
        /// start date as YYYY-MM-DD or unix timestamp (required)
        #[arg(long, default_value = "")]
        date_from: String,
        /// end date as YYYY-MM-DD or unix timestamp (required)
        #[arg(long, default_value = "")]
        date_to: String,
        /// filter by domain name or ID
        #[arg(long, default_value = "")]
        domain: String,
        /// filter by tags
        #[arg(long, value_delimiter = ',')]
        tags: Vec<String>,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    match cmd.command {
        Sub::Date {
            date_from,
            date_to,
            domain,
            group_by,
            tags,
            event,
        } => date(ctx, &date_from, &date_to, &domain, &group_by, &tags, &event),
        Sub::Country {
            date_from,
            date_to,
            domain,
            tags,
        } => opens(
            ctx,
            "/analytics/country",
            "COUNTRY",
            &date_from,
            &date_to,
            &domain,
            &tags,
        ),
        Sub::UaName {
            date_from,
            date_to,
            domain,
            tags,
        } => opens(
            ctx,
            "/analytics/ua-name",
            "USER AGENT",
            &date_from,
            &date_to,
            &domain,
            &tags,
        ),
        Sub::UaType {
            date_from,
            date_to,
            domain,
            tags,
        } => opens(
            ctx,
            "/analytics/ua-type",
            "TYPE",
            &date_from,
            &date_to,
            &domain,
            &tags,
        ),
    }
}

#[allow(clippy::too_many_arguments)]
fn date(
    ctx: &Ctx,
    date_from: &str,
    date_to: &str,
    domain: &str,
    group_by: &str,
    tags: &[String],
    events: &[String],
) -> Result<()> {
    if events.is_empty() {
        bail!("missing required flag: --event\n\nExample:\n  mailersend-cli analytics date --date-from 2025-01-01 --date-to 2025-01-30 --event sent,delivered");
    }

    let (date_from, date_to) = default_date_range(date_from, date_to)?;

    let client = ctx.client()?;

    let mut query: Vec<(&str, String)> = Vec::new();
    if !domain.is_empty() {
        let domain_id = resolve_domain_id(&client, domain)?;
        query.push(("domain_id", domain_id));
    }
    query.push(("date_from", date_from.to_string()));
    query.push(("date_to", date_to.to_string()));
    if !group_by.is_empty() {
        query.push(("group_by", group_by.to_string()));
    }
    for t in tags {
        query.push(("tags[]", t.clone()));
    }
    for e in events {
        query.push(("event[]", e.clone()));
    }

    let body = client.get_with("/analytics/date", &query)?;

    if ctx.json {
        return output::json(&body);
    }

    let mut headers: Vec<String> = vec!["DATE".to_string()];
    for e in events {
        headers.push(e.to_uppercase());
    }
    let header_refs: Vec<&str> = headers.iter().map(String::as_str).collect();

    let stats: Vec<Value> = body["data"]
        .get("stats")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();

    let rows: Vec<Vec<String>> = stats
        .iter()
        .map(|stat| {
            let mut row = vec![jstr(stat, "date")];
            for e in events {
                row.push(jint(stat, e).to_string());
            }
            row
        })
        .collect();

    output::table(&header_refs, &rows);
    Ok(())
}

fn opens(
    ctx: &Ctx,
    path: &str,
    name_header: &str,
    date_from: &str,
    date_to: &str,
    domain: &str,
    tags: &[String],
) -> Result<()> {
    let (date_from, date_to) = default_date_range(date_from, date_to)?;

    let client = ctx.client()?;

    let mut query: Vec<(&str, String)> = Vec::new();
    if !domain.is_empty() {
        let domain_id = resolve_domain_id(&client, domain)?;
        query.push(("domain_id", domain_id));
    }
    query.push(("date_from", date_from.to_string()));
    query.push(("date_to", date_to.to_string()));
    for t in tags {
        query.push(("tags[]", t.clone()));
    }

    let body = client.get_with(path, &query)?;

    if ctx.json {
        return output::json(&body);
    }

    let stats: Vec<Value> = body["data"]
        .get("stats")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();

    let rows: Vec<Vec<String>> = stats
        .iter()
        .map(|stat| vec![jstr(stat, "name"), jint(stat, "count").to_string()])
        .collect();

    output::table(&[name_header, "COUNT"], &rows);
    Ok(())
}
