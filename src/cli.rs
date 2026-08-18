use anyhow::Result;
use clap::{CommandFactory, Parser, Subcommand};
use clap_complete::Shell;

use crate::api::Client;
use crate::commands;
use crate::config;

// cargo-dist requires the git tag to match Cargo.toml's version, so the
// crate version is authoritative. Commit and date are stamped by build.rs.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");
const COMMIT: &str = match option_env!("MAILERSEND_COMMIT") {
    Some(c) => c,
    None => "none",
};
const BUILD_DATE: &str = match option_env!("MAILERSEND_BUILD_DATE") {
    Some(d) => d,
    None => "unknown",
};

/// Global flags shared by every command.
pub struct Ctx {
    pub profile: Option<String>,
    pub verbose: bool,
    pub json: bool,
}

impl Ctx {
    /// Builds an authenticated API client for the selected profile.
    pub fn client(&self) -> Result<Client> {
        let token = config::get_token(self.profile.as_deref())?;
        Ok(Client::new(token, self.verbose))
    }
}

#[derive(Parser)]
#[command(
    name = "mailersend",
    version = VERSION,
    about = "MailerSend CLI — manage your email infrastructure from the terminal",
    long_about = "A command-line interface for the MailerSend API. Send emails, manage domains, templates, webhooks, and more."
)]
struct Root {
    /// config profile to use
    #[arg(long, global = true)]
    profile: Option<String>,

    /// show HTTP request/response details
    #[arg(long, short = 'v', global = true)]
    verbose: bool,

    /// output as JSON
    #[arg(long, global = true)]
    json: bool,

    #[command(subcommand)]
    command: Option<Command>,
}

#[derive(Subcommand)]
enum Command {
    /// Launch the interactive TUI dashboard
    Dashboard,
    /// Send and manage emails
    Email(commands::email::Cmd),
    /// Manage domains
    Domain(commands::domain::Cmd),
    /// Manage messages
    Message(commands::message::Cmd),
    /// Manage templates
    Template(commands::template::Cmd),
    /// View analytics
    Analytics(commands::analytics::Cmd),
    /// View activity
    Activity(commands::activity::Cmd),
    /// Manage webhooks
    Webhook(commands::webhook::Cmd),
    /// Manage email verification
    Verification(commands::verification::Cmd),
    /// Authenticate with MailerSend
    Auth(commands::auth::Cmd),
    /// Manage config profiles
    Profile(commands::profile::Cmd),
    /// Manage recipients
    Recipient(commands::recipient::Cmd),
    /// Manage sender identities
    Identity(commands::identity::Cmd),
    /// Manage suppressions
    Suppression(commands::suppression::Cmd),
    /// Manage inbound routes
    Inbound(commands::inbound::Cmd),
    /// Manage API tokens
    Token(commands::token::Cmd),
    /// Manage users
    User(commands::user::Cmd),
    /// Manage SMTP users
    Smtp(commands::smtp::Cmd),
    /// View API quota
    Quota(commands::quota::Cmd),
    /// Manage bulk email
    #[command(name = "bulk-email")]
    Bulkemail(commands::bulkemail::Cmd),
    /// Manage SMS
    Sms(commands::sms::Cmd),
    /// Generate shell completion scripts
    Completion {
        /// shell to generate completions for
        #[arg(value_enum)]
        shell: Shell,
    },
    /// Print the version of mailersend
    Version,
}

pub fn run() -> i32 {
    let root = Root::parse();
    let ctx = Ctx {
        profile: root.profile,
        verbose: root.verbose,
        json: root.json,
    };

    let Some(command) = root.command else {
        Root::command().print_help().ok();
        return 0;
    };

    let result = match command {
        Command::Dashboard => crate::tui::run(&ctx),
        Command::Email(cmd) => commands::email::run(&ctx, cmd),
        Command::Domain(cmd) => commands::domain::run(&ctx, cmd),
        Command::Message(cmd) => commands::message::run(&ctx, cmd),
        Command::Template(cmd) => commands::template::run(&ctx, cmd),
        Command::Analytics(cmd) => commands::analytics::run(&ctx, cmd),
        Command::Activity(cmd) => commands::activity::run(&ctx, cmd),
        Command::Webhook(cmd) => commands::webhook::run(&ctx, cmd),
        Command::Verification(cmd) => commands::verification::run(&ctx, cmd),
        Command::Auth(cmd) => commands::auth::run(&ctx, cmd),
        Command::Profile(cmd) => commands::profile::run(&ctx, cmd),
        Command::Recipient(cmd) => commands::recipient::run(&ctx, cmd),
        Command::Identity(cmd) => commands::identity::run(&ctx, cmd),
        Command::Suppression(cmd) => commands::suppression::run(&ctx, cmd),
        Command::Inbound(cmd) => commands::inbound::run(&ctx, cmd),
        Command::Token(cmd) => commands::token::run(&ctx, cmd),
        Command::User(cmd) => commands::user::run(&ctx, cmd),
        Command::Smtp(cmd) => commands::smtp::run(&ctx, cmd),
        Command::Quota(cmd) => commands::quota::run(&ctx, cmd),
        Command::Bulkemail(cmd) => commands::bulkemail::run(&ctx, cmd),
        Command::Sms(cmd) => commands::sms::run(&ctx, cmd),
        Command::Completion { shell } => {
            let mut cmd = Root::command();
            clap_complete::generate(shell, &mut cmd, "mailersend", &mut std::io::stdout());
            Ok(())
        }
        Command::Version => {
            println!("mailersend v{VERSION} ({COMMIT}) built {BUILD_DATE}");
            Ok(())
        }
    };

    match result {
        Ok(()) => 0,
        Err(err) => {
            // In --json mode, surface the raw API error body when available.
            if ctx.json {
                if let Some(api_err) = err.downcast_ref::<crate::api::ApiError>() {
                    if let Some(raw) = &api_err.raw_body {
                        let _ = crate::output::json(raw);
                        return 1;
                    }
                }
            }
            crate::output::error(&format!("{err}"));
            1
        }
    }
}
