use std::sync::mpsc::{self, Receiver, Sender};
use std::thread;
use std::time::Duration;

use anyhow::Result;
use crossterm::event::{self, Event, KeyCode, KeyEvent, KeyEventKind, KeyModifiers};
use ratatui::backend::Backend;
use ratatui::layout::{Constraint, Layout, Rect};
use ratatui::style::{Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, Borders, Paragraph};
use ratatui::{Frame, Terminal};
use serde_json::Value;

use crate::api::Client;

use super::components::{help, sidebar::Sidebar, spinner::Spinner, statusbar::StatusBar};
use super::theme;
use super::views::{self, analytics::AnalyticsData, suppressions, KeyResult, ViewType};

pub enum Msg {
    Domains(Result<Vec<Value>>),
    ActivityDomains(Result<Vec<Value>>),
    Activity(Result<Vec<Value>>),
    Analytics(Result<AnalyticsData>),
    Messages(Result<Vec<Value>>),
    MessageDetail(Result<Value>),
    Suppressions(Result<Vec<suppressions::Item>>),
}

#[derive(PartialEq, Eq)]
enum Focus {
    Sidebar,
    Content,
}

pub struct App {
    client: Client,
    profile: String,
    tx: Sender<Msg>,
    rx: Receiver<Msg>,
    sidebar: Sidebar,
    statusbar: StatusBar,
    spinner: Spinner,
    domains: views::domains::View,
    activity: views::activity::View,
    analytics: views::analytics::View,
    messages: views::messages::View,
    suppressions: suppressions::View,
    active: ViewType,
    focus: Focus,
    show_help: bool,
    err: Option<String>,
}

impl App {
    pub fn new(client: Client, profile: String) -> Self {
        let (tx, rx) = mpsc::channel();
        Self {
            client,
            profile,
            tx,
            rx,
            sidebar: Sidebar::new(),
            statusbar: StatusBar::new(),
            spinner: Spinner::new("Loading..."),
            domains: views::domains::View::new(),
            activity: views::activity::View::new(),
            analytics: views::analytics::View::new(),
            messages: views::messages::View::new(),
            suppressions: suppressions::View::new(),
            active: ViewType::Domains,
            focus: Focus::Content,
            show_help: false,
            err: None,
        }
    }

    pub fn run<B: Backend>(&mut self, terminal: &mut Terminal<B>) -> Result<()> {
        self.fetch_active();

        loop {
            self.drain_msgs();
            terminal.draw(|frame| self.draw(frame))?;

            if event::poll(Duration::from_millis(100))? {
                match event::read()? {
                    Event::Key(key) if key.kind == KeyEventKind::Press && self.handle_key(key) => {
                        return Ok(());
                    }
                    _ => {}
                }
            }

            self.spinner.tick();
        }
    }

    fn drain_msgs(&mut self) {
        while let Ok(msg) = self.rx.try_recv() {
            match msg {
                Msg::Domains(Ok(items)) => self.domains.set_loaded(items),
                Msg::Domains(Err(e)) => {
                    self.domains.loading = false;
                    self.domains.table.set_loading(false);
                    self.err = Some(e.to_string());
                }
                Msg::ActivityDomains(Ok(domains)) => {
                    self.activity.set_domains(domains);
                    match self.activity.domain_id() {
                        Some(id) => self.spawn_activity(&id),
                        None => self.activity.no_domains(),
                    }
                }
                Msg::ActivityDomains(Err(e)) => {
                    self.activity.loading = false;
                    self.activity.loading_domains = false;
                    self.err = Some(e.to_string());
                }
                Msg::Activity(Ok(items)) => self.activity.set_loaded(items),
                Msg::Activity(Err(e)) => {
                    self.activity.loading = false;
                    self.activity.table.set_loading(false);
                    self.err = Some(e.to_string());
                }
                Msg::Analytics(Ok(data)) => self.analytics.set_loaded(data),
                Msg::Analytics(Err(e)) => {
                    self.analytics.loading = false;
                    self.analytics.table.set_loading(false);
                    self.err = Some(e.to_string());
                }
                Msg::Messages(Ok(items)) => self.messages.set_loaded(items),
                Msg::Messages(Err(e)) => {
                    self.messages.loading = false;
                    self.messages.table.set_loading(false);
                    self.err = Some(e.to_string());
                }
                Msg::MessageDetail(Ok(detail)) => self.messages.set_detail(detail),
                Msg::MessageDetail(Err(e)) => {
                    self.messages.loading_detail = false;
                    self.err = Some(e.to_string());
                }
                Msg::Suppressions(Ok(items)) => self.suppressions.set_loaded(items),
                Msg::Suppressions(Err(e)) => {
                    self.suppressions.loading = false;
                    self.suppressions.table.set_loading(false);
                    self.err = Some(e.to_string());
                }
            }
        }
        if self.active_view_loading() {
            self.spinner.start();
        } else {
            self.spinner.stop();
        }
    }

    fn active_view_loading(&self) -> bool {
        match self.active {
            ViewType::Domains => self.domains.loading,
            ViewType::Activity => self.activity.loading,
            ViewType::Analytics => self.analytics.loading,
            ViewType::Messages => self.messages.loading,
            ViewType::Suppressions => self.suppressions.loading,
        }
    }

    fn active_item_count(&self) -> usize {
        match self.active {
            ViewType::Domains => self.domains.items.len(),
            ViewType::Activity => self.activity.items.len(),
            ViewType::Analytics => self.analytics.data.stats.len(),
            ViewType::Messages => self.messages.items.len(),
            ViewType::Suppressions => self.suppressions.items.len(),
        }
    }

    fn fetch_active(&mut self) {
        self.err = None;
        match self.active {
            ViewType::Domains => {
                self.domains.loading = true;
                self.domains.table.set_loading(true);
                self.spawn_domains();
            }
            ViewType::Activity => {
                self.activity.loading = true;
                self.activity.loading_domains = true;
                self.spawn_domains_for_activity();
            }
            ViewType::Analytics => {
                self.analytics.loading = true;
                self.analytics.table.set_loading(true);
                let days = self.analytics.days();
                self.spawn_analytics(days);
            }
            ViewType::Messages => {
                self.messages.loading = true;
                self.messages.table.set_loading(true);
                self.spawn_messages();
            }
            ViewType::Suppressions => {
                self.suppressions.loading = true;
                self.suppressions.table.set_loading(true);
                let tab = self.suppressions.active_tab;
                self.spawn_suppressions(tab);
            }
        }
        self.spinner
            .set_label(format!("Loading {}...", self.active.label()));
        self.spinner.start();
    }

    fn refresh_activity(&mut self) {
        self.err = None;
        self.activity.loading = true;
        match self.activity.domain_id() {
            Some(id) => self.spawn_activity(&id),
            None => {
                self.activity.loading = false;
                self.activity.table.set_loading(false);
            }
        }
    }

    fn spawn_domains(&self) {
        let client = self.client.clone();
        let tx = self.tx.clone();
        thread::spawn(move || {
            let _ = tx.send(Msg::Domains(views::domains::fetch(&client)));
        });
    }

    fn spawn_domains_for_activity(&self) {
        let client = self.client.clone();
        let tx = self.tx.clone();
        thread::spawn(move || {
            let _ = tx.send(Msg::ActivityDomains(views::activity::fetch_domains(
                &client,
            )));
        });
    }

    fn spawn_activity(&self, domain_id: &str) {
        let client = self.client.clone();
        let tx = self.tx.clone();
        let domain_id = domain_id.to_string();
        thread::spawn(move || {
            let _ = tx.send(Msg::Activity(views::activity::fetch_activity(
                &client, &domain_id,
            )));
        });
    }

    fn spawn_analytics(&self, days: i64) {
        let client = self.client.clone();
        let tx = self.tx.clone();
        thread::spawn(move || {
            let _ = tx.send(Msg::Analytics(views::analytics::fetch(&client, days)));
        });
    }

    fn spawn_messages(&self) {
        let client = self.client.clone();
        let tx = self.tx.clone();
        thread::spawn(move || {
            let _ = tx.send(Msg::Messages(views::messages::fetch(&client)));
        });
    }

    fn spawn_message_detail(&self, message_id: String) {
        let client = self.client.clone();
        let tx = self.tx.clone();
        thread::spawn(move || {
            let _ = tx.send(Msg::MessageDetail(views::messages::fetch_detail(
                &client,
                &message_id,
            )));
        });
    }

    fn spawn_suppressions(&self, tab: suppressions::Tab) {
        let client = self.client.clone();
        let tx = self.tx.clone();
        thread::spawn(move || {
            let _ = tx.send(Msg::Suppressions(suppressions::fetch(&client, tab)));
        });
    }

    /// Returns true when the app should quit.
    fn handle_key(&mut self, key: KeyEvent) -> bool {
        let code = key.code;

        if self.show_help {
            if matches!(code, KeyCode::Char('?') | KeyCode::Esc | KeyCode::Backspace) {
                self.show_help = false;
            }
            return false;
        }

        // Date-range keys belong to the analytics view while it is focused.
        if self.focus == Focus::Content && self.active == ViewType::Analytics {
            let range = match code {
                KeyCode::Char('1') => Some("7d"),
                KeyCode::Char('2') => Some("30d"),
                KeyCode::Char('3') => Some("90d"),
                _ => None,
            };
            if let Some(range) = range {
                self.analytics.set_range(range);
                self.fetch_active();
                return false;
            }
        }

        match code {
            KeyCode::Char('c') if key.modifiers.contains(KeyModifiers::CONTROL) => {
                return true;
            }
            KeyCode::Char('q') => return true,
            KeyCode::Char('?') => {
                self.show_help = true;
                return false;
            }
            KeyCode::Tab => {
                self.focus = match self.focus {
                    Focus::Sidebar => Focus::Content,
                    Focus::Content => Focus::Sidebar,
                };
                return false;
            }
            KeyCode::Char('1') => {
                self.switch_view(ViewType::Domains);
                return false;
            }
            KeyCode::Char('2') => {
                self.switch_view(ViewType::Activity);
                return false;
            }
            KeyCode::Char('3') => {
                self.switch_view(ViewType::Analytics);
                return false;
            }
            KeyCode::Char('4') => {
                self.switch_view(ViewType::Messages);
                return false;
            }
            KeyCode::Char('5') => {
                self.switch_view(ViewType::Suppressions);
                return false;
            }
            _ => {}
        }

        match self.focus {
            Focus::Sidebar => self.handle_sidebar_key(code),
            Focus::Content => self.handle_content_key(code),
        }

        false
    }

    fn handle_sidebar_key(&mut self, code: KeyCode) {
        match code {
            KeyCode::Char('j') | KeyCode::Down => {
                self.sidebar.next();
                self.switch_view(self.sidebar.active());
            }
            KeyCode::Char('k') | KeyCode::Up => {
                self.sidebar.prev();
                self.switch_view(self.sidebar.active());
            }
            KeyCode::Enter | KeyCode::Char('l') | KeyCode::Right => {
                self.focus = Focus::Content;
            }
            _ => {}
        }
    }

    fn handle_content_key(&mut self, code: KeyCode) {
        let result = match self.active {
            ViewType::Domains => self.domains.handle_key(code),
            ViewType::Activity => self.activity.handle_key(code),
            ViewType::Analytics => self.analytics.handle_key(code),
            ViewType::Messages => self.messages.handle_key(code),
            ViewType::Suppressions => self.suppressions.handle_key(code),
        };
        match result {
            KeyResult::None => {}
            KeyResult::Fetch => match self.active {
                ViewType::Activity => self.refresh_activity(),
                _ => self.fetch_active(),
            },
            KeyResult::FetchDetail(id) => self.spawn_message_detail(id),
        }
    }

    fn switch_view(&mut self, kind: ViewType) {
        if self.active == kind {
            return;
        }
        self.active = kind;
        self.sidebar.set_active(kind);
        self.fetch_active();
    }

    fn draw(&mut self, frame: &mut Frame) {
        let area = frame.area();
        if area.width == 0 || area.height == 0 {
            return;
        }

        let [header_area, content_area, status_area] = Layout::vertical([
            Constraint::Length(2),
            Constraint::Min(0),
            Constraint::Length(2),
        ])
        .areas(area);

        self.draw_header(frame, header_area);

        let [sidebar_area, main_area] = Layout::horizontal([
            Constraint::Length(super::components::sidebar::WIDTH),
            Constraint::Min(0),
        ])
        .areas(content_area);

        self.sidebar.set_focused(self.focus == Focus::Sidebar);
        self.sidebar.render(frame, sidebar_area);

        self.draw_main(frame, main_area);
        self.draw_status(frame, status_area);

        if self.show_help {
            help::render(frame, area);
        }
    }

    fn draw_header(&self, frame: &mut Frame, area: Rect) {
        let block = Block::new()
            .borders(Borders::BOTTOM)
            .border_style(Style::new().fg(theme::MUTED));
        let inner = block.inner(area);
        frame.render_widget(block, area);

        let title = " MailerSend Dashboard ";
        let profile = format!("profile: {}", self.profile);
        let gap = (inner.width as usize)
            .saturating_sub(title.chars().count() + profile.chars().count() + 4)
            .max(1);

        let line = Line::from(vec![
            Span::styled(
                title,
                Style::new().fg(theme::PRIMARY).add_modifier(Modifier::BOLD),
            ),
            Span::raw(" ".repeat(gap)),
            Span::styled(profile, Style::new().fg(theme::MUTED)),
        ]);
        frame.render_widget(Paragraph::new(line), inner);
    }

    fn draw_main(&mut self, frame: &mut Frame, area: Rect) {
        let mut inner = area;
        inner.x += 1;
        inner.width = inner.width.saturating_sub(2);

        if let Some(err) = &self.err {
            let [err_area, rest] =
                Layout::vertical([Constraint::Length(2), Constraint::Min(0)]).areas(inner);
            frame.render_widget(
                Paragraph::new(Line::styled(
                    format!("Error: {err}"),
                    Style::new().fg(theme::ERROR),
                )),
                err_area,
            );
            inner = rest;
        }

        let focused = self.focus == Focus::Content;
        match self.active {
            ViewType::Domains => self.domains.render(frame, inner, focused),
            ViewType::Activity => self.activity.render(frame, inner, focused),
            ViewType::Analytics => self.analytics.render(frame, inner, focused),
            ViewType::Messages => self.messages.render(frame, inner, focused),
            ViewType::Suppressions => self.suppressions.render(frame, inner, focused),
        }
    }

    fn draw_status(&mut self, frame: &mut Frame, area: Rect) {
        self.statusbar.set_profile(self.profile.clone());

        if self.active_view_loading() {
            self.statusbar.set_left(self.active.label());
            self.statusbar.set_loading(true, self.spinner.view());
        } else {
            self.statusbar.set_left(format!(
                "{} ({})",
                self.active.label(),
                self.active_item_count()
            ));
            self.statusbar.set_loading(false, "");
        }

        self.statusbar.render(frame, area);
    }
}
