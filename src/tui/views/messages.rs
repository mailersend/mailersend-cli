use anyhow::Result;
use crossterm::event::KeyCode;
use ratatui::layout::Rect;
use ratatui::widgets::Paragraph;
use ratatui::Frame;
use serde_json::Value;

use super::{fmt_datetime, KeyResult};
use crate::api::{self, Client};
use crate::tui::components::detail::{DetailPanel, DetailRow};
use crate::tui::components::table::{Cell, Column, Table};
use crate::util::{jpath, jstr};

pub const EMPTY: &str = "No messages found.";

pub fn columns() -> Vec<Column> {
    vec![
        Column {
            title: "MESSAGE ID",
            width: 28,
        },
        Column {
            title: "CREATED",
            width: 19,
        },
        Column {
            title: "UPDATED",
            width: 19,
        },
    ]
}

pub fn fetch(client: &Client) -> Result<Vec<Value>> {
    // API returns oldest first; reverse so newest appear at the top.
    let mut items = api::fetch_all_paged(client, "/messages", &[], 0)?;
    items.reverse();
    Ok(items)
}

pub fn fetch_detail(client: &Client, message_id: &str) -> Result<Value> {
    Ok(client.get(&format!("/messages/{message_id}"))?["data"].clone())
}

pub struct View {
    pub table: Table,
    detail: DetailPanel,
    pub items: Vec<Value>,
    pub loading: bool,
    pub loading_detail: bool,
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
            loading_detail: false,
            showing_detail: false,
        }
    }

    pub fn set_loaded(&mut self, items: Vec<Value>) {
        let rows = items
            .iter()
            .map(|m| {
                vec![
                    Cell::plain(jstr(m, "id")),
                    Cell::plain(fmt_datetime(&jstr(m, "created_at"))),
                    Cell::plain(fmt_datetime(&jstr(m, "updated_at"))),
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
            KeyCode::Enter => return self.enter_detail(),
            KeyCode::Char('r') => {
                self.loading = true;
                self.table.set_loading(true);
                return KeyResult::Fetch;
            }
            _ => {}
        }
        KeyResult::None
    }

    fn enter_detail(&mut self) -> KeyResult {
        let Some(item) = self.items.get(self.table.cursor()) else {
            return KeyResult::None;
        };
        let id = jstr(item, "id");

        self.showing_detail = true;
        self.loading_detail = true;
        self.detail.set_title("Message Details");
        self.detail.set_rows(vec![
            DetailRow::new("Message ID", id.clone()),
            DetailRow::new("Created", fmt_datetime(&jstr(item, "created_at"))),
            DetailRow::new("", "Loading details..."),
        ]);
        KeyResult::FetchDetail(id)
    }

    pub fn set_detail(&mut self, detail: Value) {
        self.loading_detail = false;

        let emails: Vec<Value> = detail
            .get("emails")
            .and_then(Value::as_array)
            .cloned()
            .unwrap_or_default();

        let mut rows = vec![
            DetailRow::new("Message ID", jstr(&detail, "id")),
            DetailRow::new("Domain", jpath(&detail, "domain.name")),
            DetailRow::new("Created", fmt_datetime(&jstr(&detail, "created_at"))),
            DetailRow::new("Updated", fmt_datetime(&jstr(&detail, "updated_at"))),
        ];

        for (i, e) in emails.iter().enumerate() {
            if i > 0 {
                rows.push(DetailRow::new("", ""));
            }
            rows.push(DetailRow::new("", ""));

            let email_label = if emails.len() > 1 {
                format!("Email {}", i + 1)
            } else {
                "Email".to_string()
            };
            rows.push(DetailRow::new(email_label, jstr(e, "id")));
            rows.push(DetailRow::new("From", jstr(e, "from")));
            rows.push(DetailRow::new("Subject", jstr(e, "subject")));
            rows.push(DetailRow::new("Status", jstr(e, "status")));
            let tags: Vec<String> = e
                .get("tags")
                .and_then(Value::as_array)
                .map(|arr| {
                    arr.iter()
                        .filter_map(Value::as_str)
                        .map(str::to_owned)
                        .collect()
                })
                .unwrap_or_default();
            if !tags.is_empty() {
                rows.push(DetailRow::new("Tags", tags.join(", ")));
            }
        }

        self.detail.set_title("Message Details");
        self.detail.set_rows(rows);
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
