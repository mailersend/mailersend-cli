use anyhow::Result;
use crossterm::event::KeyCode;
use ratatui::layout::Rect;
use ratatui::widgets::Paragraph;
use ratatui::Frame;
use serde_json::Value;

use super::{fmt_date, fmt_datetime, KeyResult};
use crate::api::{self, Client};
use crate::tui::components::detail::{DetailPanel, DetailRow};
use crate::tui::components::table::{Cell, Column, Table};
use crate::tui::theme;
use crate::util::jstr;

pub const EMPTY: &str = "No domains found. Add a domain to get started.";

pub fn columns() -> Vec<Column> {
    vec![
        Column {
            title: "NAME",
            width: 30,
        },
        Column {
            title: "VERIFIED",
            width: 10,
        },
        Column {
            title: "DNS",
            width: 8,
        },
        Column {
            title: "TRACKING",
            width: 10,
        },
        Column {
            title: "CREATED",
            width: 12,
        },
    ]
}

pub fn fetch(client: &Client) -> Result<Vec<Value>> {
    api::fetch_all_paged(client, "/domains", &[], 100)
}

fn jbool(v: &Value, key: &str) -> bool {
    v.get(key).and_then(Value::as_bool).unwrap_or(false)
}

fn check(ok: bool) -> Cell {
    if ok {
        Cell::colored("✓", theme::SUCCESS)
    } else {
        Cell::colored("✗", theme::ERROR)
    }
}

fn yes_no(ok: bool) -> &'static str {
    if ok {
        "Yes"
    } else {
        "No"
    }
}

pub struct View {
    pub table: Table,
    detail: DetailPanel,
    pub items: Vec<Value>,
    pub loading: bool,
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
            showing_detail: false,
        }
    }

    pub fn set_loaded(&mut self, items: Vec<Value>) {
        let rows = items
            .iter()
            .map(|d| {
                vec![
                    Cell::plain(jstr(d, "name")),
                    check(jbool(d, "is_verified")),
                    check(jbool(d, "is_dns_active")),
                    check(jbool(d, "tracking")),
                    Cell::plain(fmt_date(&jstr(d, "created_at"))),
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
        let Some(d) = self.items.get(self.table.cursor()) else {
            return;
        };
        self.detail
            .set_title(format!("Domain: {}", jstr(d, "name")));
        self.detail.set_rows(vec![
            DetailRow::new("ID", jstr(d, "id")),
            DetailRow::new("Name", jstr(d, "name")),
            DetailRow::new("Verified", yes_no(jbool(d, "is_verified"))),
            DetailRow::new("DNS Active", yes_no(jbool(d, "is_dns_active"))),
            DetailRow::new(
                "Tracking",
                if jbool(d, "tracking") {
                    "Enabled"
                } else {
                    "Disabled"
                },
            ),
            DetailRow::new("Created", fmt_datetime(&jstr(d, "created_at"))),
        ]);
        self.showing_detail = true;
    }

    pub fn render(&mut self, frame: &mut Frame, area: Rect, focused: bool) {
        if self.showing_detail {
            frame.render_widget(Paragraph::new(self.detail.lines()), area);
            return;
        }
        self.table.set_focused(focused);
        self.table
            .set_size(area.width as usize, area.height as usize);
        frame.render_widget(Paragraph::new(self.table.lines()), area);
    }
}
