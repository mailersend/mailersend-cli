use anyhow::Result;
use chrono::{Duration, Utc};
use crossterm::event::KeyCode;
use ratatui::layout::{Constraint, Layout, Rect};
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::Paragraph;
use ratatui::Frame;
use serde_json::Value;

use super::KeyResult;
use crate::api::Client;
use crate::tui::components::pad_right;
use crate::tui::components::table::{Cell, Column, Table};
use crate::tui::theme;
use crate::util::jint;
use crate::util::jstr;

pub const EMPTY: &str = "No analytics data found.";

const EVENTS: [&str; 6] = [
    "sent",
    "delivered",
    "opened",
    "clicked",
    "hard_bounced",
    "soft_bounced",
];

#[derive(Default)]
pub struct AnalyticsData {
    pub stats: Vec<Value>,
    pub date_from: String,
    pub date_to: String,
    pub sent: i64,
    pub delivered: i64,
    pub opens: i64,
    pub clicks: i64,
    pub bounced: i64,
}

pub fn columns() -> Vec<Column> {
    vec![
        Column {
            title: "DATE",
            width: 12,
        },
        Column {
            title: "SENT",
            width: 10,
        },
        Column {
            title: "DELIVERED",
            width: 10,
        },
        Column {
            title: "OPENS",
            width: 10,
        },
        Column {
            title: "CLICKS",
            width: 10,
        },
        Column {
            title: "BOUNCED",
            width: 10,
        },
    ]
}

pub fn fetch(client: &Client, days: i64) -> Result<AnalyticsData> {
    let now = Utc::now();
    let from = now - Duration::days(days);
    let mut query: Vec<(&str, String)> = vec![
        ("date_from", from.timestamp().to_string()),
        ("date_to", now.timestamp().to_string()),
        ("group_by", "days".to_string()),
    ];
    for e in EVENTS {
        query.push(("event[]", e.to_string()));
    }

    let body = client.get_with("/analytics/date", &query)?;
    let stats: Vec<Value> = body["data"]
        .get("stats")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();

    let mut data = AnalyticsData {
        stats: stats.clone(),
        date_from: from.format("%Y-%m-%d").to_string(),
        date_to: now.format("%Y-%m-%d").to_string(),
        ..Default::default()
    };
    for stat in &stats {
        data.sent += jint(stat, "sent");
        data.delivered += jint(stat, "delivered");
        data.opens += jint(stat, "opened");
        data.clicks += jint(stat, "clicked");
        data.bounced += jint(stat, "hard_bounced") + jint(stat, "soft_bounced");
    }
    Ok(data)
}

pub struct View {
    pub table: Table,
    pub data: AnalyticsData,
    pub loading: bool,
    date_range: &'static str,
}

impl View {
    pub fn new() -> Self {
        let mut table = Table::new(columns(), EMPTY);
        table.set_loading(true);
        Self {
            table,
            data: AnalyticsData::default(),
            loading: true,
            date_range: "7d",
        }
    }

    pub fn days(&self) -> i64 {
        match self.date_range {
            "30d" => 30,
            "90d" => 90,
            _ => 7,
        }
    }

    pub fn set_range(&mut self, range: &'static str) {
        self.date_range = range;
        self.loading = true;
    }

    pub fn set_loaded(&mut self, data: AnalyticsData) {
        let rows = data
            .stats
            .iter()
            .map(|stat| {
                vec![
                    Cell::plain(jstr(stat, "date")),
                    Cell::plain(jint(stat, "sent").to_string()),
                    Cell::plain(jint(stat, "delivered").to_string()),
                    Cell::plain(jint(stat, "opened").to_string()),
                    Cell::plain(jint(stat, "clicked").to_string()),
                    Cell::plain(
                        (jint(stat, "hard_bounced") + jint(stat, "soft_bounced")).to_string(),
                    ),
                ]
            })
            .collect();
        self.data = data;
        self.table.set_rows(rows);
        self.table.set_loading(false);
        self.loading = false;
    }

    pub fn handle_key(&mut self, code: KeyCode) -> KeyResult {
        match code {
            KeyCode::Char('j') | KeyCode::Down => self.table.move_down(),
            KeyCode::Char('k') | KeyCode::Up => self.table.move_up(),
            KeyCode::Char('g') => self.table.goto_top(),
            KeyCode::Char('G') => self.table.goto_bottom(),
            KeyCode::Char('r') => {
                self.loading = true;
                self.table.set_loading(true);
                return KeyResult::Fetch;
            }
            KeyCode::Char('1') => {
                self.set_range("7d");
                return KeyResult::Fetch;
            }
            KeyCode::Char('2') => {
                self.set_range("30d");
                return KeyResult::Fetch;
            }
            KeyCode::Char('3') => {
                self.set_range("90d");
                return KeyResult::Fetch;
            }
            _ => {}
        }
        KeyResult::None
    }

    pub fn render(&mut self, frame: &mut Frame, area: Rect, focused: bool) {
        let [chrome, table_area] =
            Layout::vertical([Constraint::Length(7), Constraint::Min(0)]).areas(area);
        frame.render_widget(Paragraph::new(self.summary_chrome()), chrome);

        self.table.set_focused(focused);
        self.table
            .set_size(table_area.width as usize, table_area.height as usize);
        frame.render_widget(Paragraph::new(self.table.lines()), table_area);
    }

    fn summary_chrome(&self) -> Vec<Line<'static>> {
        let stats: [(&str, String, Color); 5] = [
            ("Sent", self.data.sent.to_string(), theme::PRIMARY),
            ("Delivered", self.data.delivered.to_string(), theme::SUCCESS),
            ("Opens", self.data.opens.to_string(), theme::PRIMARY),
            ("Clicks", self.data.clicks.to_string(), theme::PRIMARY),
            ("Bounced", self.data.bounced.to_string(), theme::ERROR),
        ];

        let mut top: Vec<Span<'static>> = Vec::new();
        let mut label_line: Vec<Span<'static>> = Vec::new();
        let mut value_line: Vec<Span<'static>> = Vec::new();
        let mut bottom: Vec<Span<'static>> = Vec::new();

        for (i, (label, value, color)) in stats.iter().enumerate() {
            if i > 0 {
                top.push(Span::raw(" "));
                label_line.push(Span::raw(" "));
                value_line.push(Span::raw(" "));
                bottom.push(Span::raw(" "));
            }
            let inner = label.chars().count().max(value.chars().count());
            let width = inner + 4;
            let border = Style::new().fg(theme::MUTED);

            top.push(Span::styled(format!("╭{}╮", "─".repeat(width - 2)), border));
            label_line.push(Span::styled(
                format!("│  {}  │", pad_right(label, inner)),
                Style::new().fg(theme::MUTED),
            ));
            value_line.push(Span::styled(
                format!("│  {}  │", pad_right(value, inner)),
                Style::new().fg(*color).add_modifier(Modifier::BOLD),
            ));
            bottom.push(Span::styled(format!("╰{}╯", "─".repeat(width - 2)), border));
        }

        let range = format!(
            "Date range: {} to {} ({})",
            self.data.date_from, self.data.date_to, self.date_range
        );

        vec![
            Line::from(top),
            Line::from(label_line),
            Line::from(value_line),
            Line::from(bottom),
            Line::raw(""),
            Line::from(vec![
                Span::styled(range, Style::new().fg(theme::MUTED)),
                Span::raw("  "),
                Span::styled("[1]7d [2]30d [3]90d", Style::new().fg(theme::KEY)),
            ]),
            Line::raw(""),
        ]
    }
}
