use anyhow::Result;
use clap::Args;

use crate::cli::Ctx;
use crate::output;
use crate::util::jint;

#[derive(Args)]
#[command(long_about = "Display your current API quota usage.")]
pub struct Cmd {}

pub fn run(ctx: &Ctx, _cmd: Cmd) -> Result<()> {
    let client = ctx.client()?;

    let body = client.get("/api-quota")?;

    if ctx.json {
        return output::json(&body);
    }

    let quota = jint(&body, "quota");
    let remaining = jint(&body, "remaining");
    let used = quota - remaining;

    let rows = vec![
        vec!["Total".to_string(), quota.to_string()],
        vec!["Used".to_string(), used.to_string()],
        vec!["Remaining".to_string(), remaining.to_string()],
    ];
    output::table(&["FIELD", "VALUE"], &rows);
    Ok(())
}
