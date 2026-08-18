use anyhow::Result;
use clap::{Args, Subcommand};

use crate::api;
use crate::cli::Ctx;
use crate::output;
use crate::prompt;
use crate::util::{default_date_range, jpath, jstr, resolve_domain_id};

#[derive(Args)]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// List activity for a domain
    #[command(long_about = "List activity events for a domain via the MailerSend API.")]
    List {
        /// domain name or ID (required)
        #[arg(long, default_value = "")]
        domain: String,
        /// maximum number of results to return
        #[arg(long, default_value_t = 0)]
        limit: u64,
        /// start date as YYYY-MM-DD or unix timestamp (required)
        #[arg(long, default_value = "")]
        date_from: String,
        /// end date as YYYY-MM-DD or unix timestamp (required)
        #[arg(long, default_value = "")]
        date_to: String,
        /// event types to filter (queued, sent, delivered, soft_bounced, hard_bounced, opened, clicked, unsubscribed, spam_complaints)
        #[arg(long, value_delimiter = ',')]
        event: Vec<String>,
    },
    /// Get activity details
    #[command(
        arg_required_else_help = true,
        long_about = "Get detailed information about a specific activity."
    )]
    Get {
        /// activity ID
        activity_id: String,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    match cmd.command {
        Sub::List {
            domain,
            limit,
            date_from,
            date_to,
            event,
        } => list(ctx, &domain, limit, &date_from, &date_to, &event),
        Sub::Get { activity_id } => get(ctx, &activity_id),
    }
}

fn list(
    ctx: &Ctx,
    domain: &str,
    limit: u64,
    date_from: &str,
    date_to: &str,
    events: &[String],
) -> Result<()> {
    let domain = prompt::require_arg(domain, "domain", "Domain name or ID")?;

    let (date_from, date_to) = default_date_range(date_from, date_to)?;

    let client = ctx.client()?;

    let domain_id = resolve_domain_id(&client, &domain)?;

    let mut query: Vec<(&str, String)> = vec![
        ("date_from", date_from.to_string()),
        ("date_to", date_to.to_string()),
    ];
    for e in events {
        query.push(("event[]", e.clone()));
    }

    let items = api::fetch_all_paged(&client, &format!("/activity/{domain_id}"), &query, limit)?;

    if ctx.json {
        return output::json(&items);
    }

    let rows: Vec<Vec<String>> = items
        .iter()
        .map(|item| {
            vec![
                jstr(item, "id"),
                jstr(item, "type"),
                jpath(item, "email.from"),
                output::truncate(&jpath(item, "email.subject"), 40),
                jstr(item, "created_at"),
            ]
        })
        .collect();

    output::table(&["ID", "TYPE", "FROM", "SUBJECT", "CREATED AT"], &rows);
    Ok(())
}

fn get(ctx: &Ctx, activity_id: &str) -> Result<()> {
    let client = ctx.client()?;

    let body = client.get(&format!("/activities/{activity_id}"))?;

    if ctx.json {
        return output::json(&body["data"]);
    }

    let d = &body["data"];
    println!("{:<20} {}", "ID:", jstr(d, "id"));
    println!("{:<20} {}", "Type:", jstr(d, "type"));
    println!("{:<20} {}", "From:", jpath(d, "email.from"));
    println!("{:<20} {}", "Subject:", jpath(d, "email.subject"));
    println!("{:<20} {}", "Status:", jpath(d, "email.status"));
    println!(
        "{:<20} {}",
        "Recipient Email:",
        jpath(d, "email.recipient.email")
    );
    println!("{:<20} {}", "Created At:", jstr(d, "created_at"));

    Ok(())
}
