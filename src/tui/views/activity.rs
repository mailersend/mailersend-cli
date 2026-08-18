use anyhow::Result;
use chrono::{Duration, Utc};
use crossterm::event::KeyCode;
use ratatui::layout::{Constraint, Layout, Rect};
use ratatui::style::{Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::Paragraph;
use ratatui::Frame;
use serde_json::Value;

use super::{fmt_datetime, KeyResult};
use crate::api::{self, Client};
use crate::tui::components::detail::{DetailPanel, DetailRow};
use crate::tui::components::table::{Cell, Column, Table};
use crate::tui::theme;
use crate::util::{jpath, jstr};

pub const EMPTY: &str = "Select a domain to view activity.";
pub const EMPTY_NO_DOMAINS: &str = "No domains found. Add a domain first.";

pub fn columns() -> Vec<Column> {
    vec![
        Column {
            title: "TIME",
            width: 19,
        },
        Column {
            title: "EVENT",
            width: 14,
        },
        Column {
            title: "RECIPIENT",
            width: 28,
        },
        Column {
            title: "SUBJECT",
            width: 30,
        },
    ]
}

pub fn fetch_domains(client: &Client) -> Result<Vec<Value>> {
    api::fetch_all_paged(client, "/domains", &[], 100)
}

pub fn fetch_activity(client: &Client, domain_id: &str) -> Result<Vec<Value>> {
    let now = Utc::now();
    let date_from = (now - Duration::days(30)).timestamp();
    let date_to = now.timestamp();
    api::fetch_all_paged(
        client,
        &format!("/activity/{domain_id}"),
        &[
            ("date_from", date_from.to_string()),
            ("date_to", date_to.to_string()),
        ],
        100,
    )
}

pub struct View {
    pub table: Table,
    detail: DetailPanel,
    pub items: Vec<Value>,
    pub domains: Vec<Value>,
    pub active_domain_idx: usize,
    pub loading: bool,
    pub loading_domains: bool,
    showing_detail: bool,
}

impl View {
    pub fn new() -> Self {
        Self {
            table: Table::new(columns(), EMPTY),
            detail: DetailPanel::new(),
            items: Vec::new(),
            domains: Vec::new(),
            active_domain_idx: 0,
            loading: true,
            loading_domains: true,
            showing_detail: false,
        }
    }

    pub fn set_domains(&mut self, domains: Vec<Value>) {
        self.loading_domains = false;
        self.domains = domains;
        self.active_domain_idx = 0;
    }

    pub fn no_domains(&mut self) {
        self.loading = false;
        self.loading_domains = false;
        self.table.set_empty_msg(EMPTY_NO_DOMAINS);
    }

    pub fn domain_id(&self) -> Option<String> {
        self.domains
            .get(self.active_domain_idx)
            .map(|d| jstr(d, "id"))
    }

    pub fn set_loaded(&mut self, items: Vec<Value>) {
        let rows = items
            .iter()
            .map(|item| {
                vec![
                    Cell::plain(fmt_datetime(&jstr(item, "created_at"))),
                    Cell::plain(jstr(item, "type")),
                    Cell::plain(jpath(item, "email.recipient.email")),
                    Cell::plain(jpath(item, "email.subject")),
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
                if !self.domains.is_empty() {
                    self.active_domain_idx = if self.active_domain_idx == 0 {
                        self.domains.len() - 1
                    } else {
                        self.active_domain_idx - 1
                    };
                    self.loading = true;
                    self.table.set_loading(true);
                    return KeyResult::Fetch;
                }
            }
            KeyCode::Char('l') | KeyCode::Right => {
                if !self.domains.is_empty() {
                    self.active_domain_idx = (self.active_domain_idx + 1) % self.domains.len();
                    self.loading = true;
                    self.table.set_loading(true);
                    return KeyResult::Fetch;
                }
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
        self.detail.set_title("Activity Event");
        self.detail.set_rows(vec![
            DetailRow::new("ID", jstr(item, "id")),
            DetailRow::new("Event Type", jstr(item, "type")),
            DetailRow::new("Time", fmt_datetime(&jstr(item, "created_at"))),
            DetailRow::new("From", jpath(item, "email.from")),
            DetailRow::new("To", jpath(item, "email.recipient.email")),
            DetailRow::new("Subject", jpath(item, "email.subject")),
        ]);
        self.showing_detail = true;
    }

    pub fn render(&mut self, frame: &mut Frame, area: Rect, focused: bool) {
        if self.showing_detail {
            frame.render_widget(Paragraph::new(self.detail.lines()), area);
            return;
        }

        let chrome_height = if self.domains.is_empty() && !self.loading_domains {
            0
        } else {
            4
        };

        let [chrome, table_area] =
            Layout::vertical([Constraint::Length(chrome_height), Constraint::Min(0)]).areas(area);

        if !self.domains.is_empty() {
            frame.render_widget(Paragraph::new(self.domain_chrome()), chrome);
        } else if self.loading_domains {
            frame.render_widget(
                Paragraph::new(Line::styled(
                    "Loading domains...",
                    Style::new().fg(theme::MUTED),
                )),
                chrome,
            );
        }

        self.table.set_focused(focused);
        self.table
            .set_size(table_area.width as usize, table_area.height as usize);
        frame.render_widget(Paragraph::new(self.table.lines()), table_area);
    }

    fn domain_chrome(&self) -> Vec<Line<'static>> {
        let mut tabs: Vec<Span<'static>> = Vec::new();
        let mut bar_width = 0usize;
        for (i, d) in self.domains.iter().enumerate() {
            let mut name = jstr(d, "name");
            if name.chars().count() > 20 {
                name = format!("{}...", name.chars().take(17).collect::<String>());
            }
            let label = format!("  {name}  ");
            bar_width += label.chars().count();
            let style = if i == self.active_domain_idx {
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
                format!(
                    "← → to switch domains | {} events (last 30 days)",
                    self.items.len()
                ),
                Style::new().fg(theme::MUTED),
            ),
            Line::raw(""),
        ]
    }
}
