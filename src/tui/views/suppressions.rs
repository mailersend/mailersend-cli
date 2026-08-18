use anyhow::Result;
use crossterm::event::KeyCode;
use ratatui::layout::{Constraint, Layout, Rect};
use ratatui::style::{Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::Paragraph;
use ratatui::Frame;
use serde_json::Value;

use super::{fmt_datetime, KeyResult};
use crate::api::Client;
use crate::tui::components::detail::{DetailPanel, DetailRow};
use crate::tui::components::table::{Cell, Column, Table};
use crate::tui::theme;
use crate::util::{jpath, jstr};

pub const EMPTY: &str = "No suppression entries found.";

#[derive(Clone, Copy, PartialEq, Eq)]
pub enum Tab {
    Blocklist,
    HardBounces,
    SpamComplaints,
    Unsubscribes,
}

impl Tab {
    pub const ALL: [Tab; 4] = [
        Tab::Blocklist,
        Tab::HardBounces,
        Tab::SpamComplaints,
        Tab::Unsubscribes,
    ];

    pub fn label(self) -> &'static str {
        match self {
            Tab::Blocklist => "Blocklist",
            Tab::HardBounces => "Hard Bounces",
            Tab::SpamComplaints => "Spam Complaints",
            Tab::Unsubscribes => "Unsubscribes",
        }
    }

    fn path(self) -> &'static str {
        match self {
            Tab::Blocklist => "/suppressions/blocklist",
            Tab::HardBounces => "/suppressions/hard-bounces",
            Tab::SpamComplaints => "/suppressions/spam-complaints",
            Tab::Unsubscribes => "/suppressions/unsubscribes",
        }
    }

    pub fn next(self) -> Tab {
        Tab::ALL[(self as usize + 1) % Tab::ALL.len()]
    }

    pub fn prev(self) -> Tab {
        Tab::ALL[(self as usize + Tab::ALL.len() - 1) % Tab::ALL.len()]
    }
}

pub struct Item {
    pub id: String,
    pub pattern: String,
    pub type_: String,
    pub reason: String,
    pub created_at: String,
}

pub fn columns() -> Vec<Column> {
    vec![
        Column {
            title: "EMAIL/PATTERN",
            width: 35,
        },
        Column {
            title: "TYPE/REASON",
            width: 25,
        },
        Column {
            title: "CREATED",
            width: 19,
        },
    ]
}

pub fn fetch(client: &Client, tab: Tab) -> Result<Vec<Item>> {
    let body = client.get(tab.path())?;
    let data: Vec<Value> = body
        .get("data")
        .and_then(Value::as_array)
        .cloned()
        .unwrap_or_default();

    Ok(data
        .iter()
        .map(|d| {
            let (pattern, type_, reason) = match tab {
                Tab::Blocklist => (jstr(d, "pattern"), jstr(d, "type"), String::new()),
                Tab::HardBounces => (
                    jpath(d, "recipient.email"),
                    String::new(),
                    jstr(d, "reason"),
                ),
                Tab::SpamComplaints => (
                    jpath(d, "recipient.email"),
                    String::new(),
                    "Spam complaint".to_string(),
                ),
                Tab::Unsubscribes => (
                    jpath(d, "recipient.email"),
                    String::new(),
                    jstr(d, "readable_reason"),
                ),
            };
            Item {
                id: jstr(d, "id"),
                pattern,
                type_,
                reason,
                created_at: fmt_datetime(&jstr(d, "created_at")),
            }
        })
        .collect())
}

pub struct View {
    pub table: Table,
    detail: DetailPanel,
    pub items: Vec<Item>,
    pub loading: bool,
    pub active_tab: Tab,
    showing_detail: bool,
}

impl View {
    pub fn new() -> Self {
        let mut table = Table::new(columns(), EMPTY);
        table.set_loading(true);
        Self {
            table,
            detail: DetailPanel::new(),
            items: Vec::new(),
            loading: true,
            active_tab: Tab::Blocklist,
            showing_detail: false,
        }
    }

    pub fn set_loaded(&mut self, items: Vec<Item>) {
        let rows = items
            .iter()
            .map(|item| {
                let type_or_reason = if item.reason.is_empty() {
                    item.type_.clone()
                } else {
                    item.reason.clone()
                };
                vec![
                    Cell::plain(item.pattern.clone()),
                    Cell::plain(type_or_reason),
                    Cell::plain(item.created_at.clone()),
                ]
            })
            .collect();
        self.items = items;
        self.table.set_rows(rows);
        self.table.set_loading(false);
        self.loading = false;
    }

    pub fn handle_key(&mut self, code: KeyCode) -> KeyResult {
        if self.showing_detail {
            if matches!(code, KeyCode::Esc | KeyCode::Backspace | KeyCode::Char('q')) {
                self.showing_detail = false;
            }
            return KeyResult::None;
        }

        match code {
            KeyCode::Char('j') | KeyCode::Down => self.table.move_down(),
            KeyCode::Char('k') | KeyCode::Up => self.table.move_up(),
            KeyCode::Char('g') => self.table.goto_top(),
            KeyCode::Char('G') => self.table.goto_bottom(),
            KeyCode::Char('h') | KeyCode::Left => {
                self.active_tab = self.active_tab.prev();
                self.loading = true;
                return KeyResult::Fetch;
            }
            KeyCode::Char('l') | KeyCode::Right => {
                self.active_tab = self.active_tab.next();
                self.loading = true;
                return KeyResult::Fetch;
            }
            KeyCode::Enter => self.show_detail(),
            KeyCode::Char('r') => {
                self.loading = true;
                self.table.set_loading(true);
                return KeyResult::Fetch;
            }
            _ => {}
        }
        KeyResult::None
    }

    fn show_detail(&mut self) {
        let Some(item) = self.items.get(self.table.cursor()) else {
            return;
        };
        self.detail.set_title("Suppression Entry");
        let mut rows = vec![
            DetailRow::new("ID", item.id.clone()),
            DetailRow::new("Email/Pattern", item.pattern.clone()),
            DetailRow::new("List Type", self.active_tab.label()),
        ];
        if !item.type_.is_empty() {
            rows.push(DetailRow::new("Type", item.type_.clone()));
        }
        if !item.reason.is_empty() {
            rows.push(DetailRow::new("Reason", item.reason.clone()));
        }
        rows.push(DetailRow::new("Created", item.created_at.clone()));
        self.detail.set_rows(rows);
        self.showing_detail = true;
    }

    pub fn render(&mut self, frame: &mut Frame, area: Rect, focused: bool) {
        if self.showing_detail {
            frame.render_widget(Paragraph::new(self.detail.lines()), area);
            return;
        }

        let [chrome, table_area] =
            Layout::vertical([Constraint::Length(4), Constraint::Min(0)]).areas(area);
        frame.render_widget(Paragraph::new(self.tab_chrome()), chrome);

        self.table.set_focused(focused);
        self.table
            .set_size(table_area.width as usize, table_area.height as usize);
        frame.render_widget(Paragraph::new(self.table.lines()), table_area);
    }

    fn tab_chrome(&self) -> Vec<Line<'static>> {
        let mut tabs: Vec<Span<'static>> = Vec::new();
        let mut bar_width = 0usize;
        for tab in Tab::ALL {
            let label = format!("  {}  ", tab.label());
            bar_width += label.chars().count();
            let style = if tab == self.active_tab {
                Style::new()
                    .fg(theme::PRIMARY)
                    .bg(theme::BG_SELECTED)
                    .add_modifier(Modifier::BOLD)
            } else {
                Style::new()
            };
            tabs.push(Span::styled(label, style));
        }

        vec![
            Line::from(tabs),
            Line::styled("─".repeat(bar_width), Style::new().fg(theme::MUTED)),
            Line::styled(
                format!("← → to switch tabs | {} items", self.items.len()),
                Style::new().fg(theme::MUTED),
            ),
            Line::raw(""),
        ]
    }
}
