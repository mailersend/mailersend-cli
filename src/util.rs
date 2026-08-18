use anyhow::{bail, Result};
use chrono::{Duration, NaiveDate, Utc};
use serde_json::Value;

use crate::api::{self, Client};

/// Accepts YYYY-MM-DD or a raw unix timestamp; returns a unix timestamp.
pub fn parse_date(value: &str) -> Result<i64> {
    if let Ok(d) = NaiveDate::parse_from_str(value, "%Y-%m-%d") {
        return Ok(d
            .and_hms_opt(0, 0, 0)
            .expect("valid midnight")
            .and_utc()
            .timestamp());
    }
    match value.parse::<i64>() {
        Ok(ts) => Ok(ts),
        Err(_) => bail!("invalid date {value:?}: use YYYY-MM-DD or a unix timestamp"),
    }
}

/// Returns (date_from, date_to) unix timestamps, defaulting either side of a
/// missing bound to the last 7 days.
pub fn default_date_range(date_from: &str, date_to: &str) -> Result<(i64, i64)> {
    let now = Utc::now();
    let week_ago = (now - Duration::days(7)).timestamp();
    let from = if date_from.is_empty() {
        week_ago
    } else {
        parse_date(date_from)?
    };
    let to = if date_to.is_empty() {
        now.timestamp()
    } else {
        parse_date(date_to)?
    };
    Ok((from, to))
}

/// Resolves a domain ID or hostname to a domain ID. Values that contain a
/// dot are treated as hostnames and looked up via the domains list.
pub fn resolve_domain_id(client: &Client, id_or_name: &str) -> Result<String> {
    if !id_or_name.contains('.') {
        return Ok(id_or_name.to_string());
    }
    let domains = api::fetch_all_paged(client, "/domains", &[], 0)
        .map_err(|e| anyhow::anyhow!("failed to list domains for resolution: {e}"))?;
    for d in &domains {
        if jstr(d, "name").eq_ignore_ascii_case(id_or_name) {
            return Ok(jstr(d, "id"));
        }
    }
    bail!("domain {id_or_name:?} not found")
}

/// Resolves a domain ID or hostname to a domain name. Values that contain a
/// dot are returned as-is; IDs are looked up via the domains list.
pub fn resolve_domain_name(client: &Client, id_or_name: &str) -> Result<String> {
    if id_or_name.contains('.') {
        return Ok(id_or_name.to_string());
    }
    let domains = api::fetch_all_paged(client, "/domains", &[], 0)
        .map_err(|e| anyhow::anyhow!("failed to list domains for resolution: {e}"))?;
    for d in &domains {
        if jstr(d, "id") == id_or_name {
            return Ok(jstr(d, "name"));
        }
    }
    bail!("domain ID {id_or_name:?} not found")
}

/// Extracts a string field from a JSON object ("" when absent). Numbers are
/// rendered with `to_string` so numeric IDs display cleanly.
pub fn jstr(v: &Value, key: &str) -> String {
    match v.get(key) {
        Some(Value::String(s)) => s.clone(),
        Some(Value::Number(n)) => n.to_string(),
        Some(Value::Bool(b)) => b.to_string(),
        _ => String::new(),
    }
}

/// Extracts a nested string via a dotted path (e.g. "domain.name").
pub fn jpath(v: &Value, path: &str) -> String {
    let mut cur = v;
    for part in path.split('.') {
        match cur.get(part) {
            Some(next) => cur = next,
            None => return String::new(),
        }
    }
    match cur {
        Value::String(s) => s.clone(),
        Value::Number(n) => n.to_string(),
        Value::Bool(b) => b.to_string(),
        Value::Null => String::new(),
        other => other.to_string(),
    }
}

/// Extracts an integer field (0 when absent).
pub fn jint(v: &Value, key: &str) -> i64 {
    v.get(key).and_then(Value::as_i64).unwrap_or(0)
}
