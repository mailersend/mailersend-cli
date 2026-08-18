pub mod activity;
pub mod analytics;
pub mod domains;
pub mod messages;
pub mod suppressions;

use chrono::DateTime;

#[derive(Clone, Copy, PartialEq, Eq)]
pub enum ViewType {
    Domains,
    Activity,
    Analytics,
    Messages,
    Suppressions,
}

impl ViewType {
    pub const ALL: [ViewType; 5] = [
        ViewType::Domains,
        ViewType::Activity,
        ViewType::Analytics,
        ViewType::Messages,
        ViewType::Suppressions,
    ];

    pub fn index(self) -> usize {
        match self {
            ViewType::Domains => 0,
            ViewType::Activity => 1,
            ViewType::Analytics => 2,
            ViewType::Messages => 3,
            ViewType::Suppressions => 4,
        }
    }

    pub fn label(self) -> &'static str {
        match self {
            ViewType::Domains => "Domains",
            ViewType::Activity => "Activity",
            ViewType::Analytics => "Analytics",
            ViewType::Messages => "Messages",
            ViewType::Suppressions => "Suppressions",
        }
    }

    pub fn icon(self) -> &'static str {
        match self {
            ViewType::Domains => "◉",
            ViewType::Activity => "◈",
            ViewType::Analytics => "◆",
            ViewType::Messages => "◇",
            ViewType::Suppressions => "◌",
        }
    }
}
pub fn fmt_date(value: &str) -> String {
    DateTime::parse_from_rfc3339(value)
        .map(|date| date.format("%Y-%m-%d").to_string())
        .unwrap_or_else(|_| value.to_string())
}

pub fn fmt_datetime(value: &str) -> String {
    DateTime::parse_from_rfc3339(value)
        .map(|date| date.format("%Y-%m-%d %H:%M:%S").to_string())
        .unwrap_or_else(|_| value.to_string())
}

pub enum KeyResult {
    None,
    Fetch,
    FetchDetail(String),
}
