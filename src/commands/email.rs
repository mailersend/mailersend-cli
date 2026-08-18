use std::io::Read;

use anyhow::{anyhow, Result};
use clap::{Args, Subcommand};
use serde_json::json;

use crate::api::Client;
use crate::cli::Ctx;
use crate::output;
use crate::prompt;

#[derive(Args)]
pub struct Cmd {
    #[command(subcommand)]
    command: Sub,
}

#[derive(Subcommand)]
enum Sub {
    /// Send an email
    #[command(long_about = "Send an email via the MailerSend API.")]
    Send {
        /// sender email address
        #[arg(long, default_value = "")]
        from: String,
        /// sender name
        #[arg(long, default_value = "")]
        from_name: String,
        /// recipient email address (required)
        #[arg(long, default_value = "")]
        to: String,
        /// recipient name
        #[arg(long, default_value = "")]
        to_name: String,
        /// CC email address
        #[arg(long, default_value = "")]
        cc: String,
        /// BCC email address
        #[arg(long, default_value = "")]
        bcc: String,
        /// reply-to email address
        #[arg(long, default_value = "")]
        reply_to: String,
        /// email subject
        #[arg(long, default_value = "")]
        subject: String,
        /// plain text body
        #[arg(long, default_value = "")]
        text: String,
        /// HTML body
        #[arg(long, default_value = "")]
        html: String,
        /// path to file containing HTML body
        #[arg(long, default_value = "")]
        html_file: String,
        /// path to file containing plain text body
        #[arg(long, default_value = "")]
        text_file: String,
        /// template ID to use
        #[arg(long, default_value = "")]
        template_id: String,
        /// email tags
        #[arg(long, value_delimiter = ',')]
        tags: Option<Vec<String>>,
        /// unix timestamp for scheduled sending
        #[arg(long, default_value_t = 0)]
        send_at: i64,
        /// enable click tracking
        #[arg(long)]
        track_clicks: bool,
        /// enable open tracking
        #[arg(long)]
        track_opens: bool,
        /// enable content tracking
        #[arg(long)]
        track_content: bool,
    },
}

pub fn run(ctx: &Ctx, cmd: Cmd) -> Result<()> {
    let client = ctx.client()?;
    match cmd.command {
        Sub::Send {
            from,
            from_name,
            to,
            to_name,
            cc,
            bcc,
            reply_to,
            subject,
            text,
            html,
            html_file,
            text_file,
            template_id,
            tags,
            send_at,
            track_clicks,
            track_opens,
            track_content,
        } => send(
            ctx,
            &client,
            &from,
            &from_name,
            &to,
            &to_name,
            &cc,
            &bcc,
            &reply_to,
            &subject,
            &text,
            &html,
            &html_file,
            &text_file,
            &template_id,
            tags,
            send_at,
            track_clicks,
            track_opens,
            track_content,
        ),
    }
}

#[allow(clippy::too_many_arguments)]
#[allow(clippy::fn_params_excessive_bools)]
fn send(
    ctx: &Ctx,
    client: &Client,
    from: &str,
    from_name: &str,
    to: &str,
    to_name: &str,
    cc: &str,
    bcc: &str,
    reply_to: &str,
    subject: &str,
    text: &str,
    html: &str,
    html_file: &str,
    text_file: &str,
    template_id: &str,
    tags: Option<Vec<String>>,
    send_at: i64,
    track_clicks: bool,
    track_opens: bool,
    track_content: bool,
) -> Result<()> {
    let to = prompt::require_arg(to, "to", "Recipient email address")?;

    let mut from = from.to_string();
    let mut subject = subject.to_string();
    let mut text = text.to_string();
    let mut html = html.to_string();
    let mut template_id = template_id.to_string();

    if from.is_empty() && prompt::is_interactive() {
        from = prompt::input("Sender email address", "")?;
    }

    if subject.is_empty() && prompt::is_interactive() {
        subject = prompt::input("Subject", "")?;
    }

    if html.is_empty()
        && text.is_empty()
        && html_file.is_empty()
        && text_file.is_empty()
        && template_id.is_empty()
        && prompt::is_interactive()
    {
        let options = vec![
            "text".to_string(),
            "html".to_string(),
            "template-id".to_string(),
        ];
        let content_type = prompt::select_labeled("Email content type", options.clone(), options)?;
        match content_type.as_str() {
            "text" => text = prompt::input("Plain text body", "")?,
            "html" => html = prompt::input("HTML body", "")?,
            "template-id" => template_id = prompt::input("Template ID", "")?,
            _ => {}
        }
    }

    if !html_file.is_empty() {
        html = std::fs::read_to_string(html_file)
            .map_err(|e| anyhow!("failed to read HTML file: {e}"))?;
    }

    if !text_file.is_empty() {
        text = std::fs::read_to_string(text_file)
            .map_err(|e| anyhow!("failed to read text file: {e}"))?;
    }

    if html.is_empty() && text.is_empty() && template_id.is_empty() && !prompt::is_interactive() {
        let mut buf = String::new();
        std::io::stdin()
            .read_to_string(&mut buf)
            .map_err(|e| anyhow!("failed to read stdin: {e}"))?;
        if !buf.is_empty() {
            html = buf;
        }
    }

    let mut body = json!({ "to": [{ "email": to, "name": to_name }] });
    if !from.is_empty() {
        body["from"] = json!({ "email": from, "name": from_name });
    }
    if !cc.is_empty() {
        body["cc"] = json!([{ "email": cc, "name": "" }]);
    }
    if !bcc.is_empty() {
        body["bcc"] = json!([{ "email": bcc, "name": "" }]);
    }
    if !reply_to.is_empty() {
        body["reply_to"] = json!({ "email": reply_to, "name": "" });
    }
    if !subject.is_empty() {
        body["subject"] = json!(subject);
    }
    if !html.is_empty() {
        body["html"] = json!(html);
    }
    if !text.is_empty() {
        body["text"] = json!(text);
    }
    if !template_id.is_empty() {
        body["template_id"] = json!(template_id);
    }
    if let Some(tags) = tags {
        if !tags.is_empty() {
            body["tags"] = json!(tags);
        }
    }
    if send_at != 0 {
        body["send_at"] = json!(send_at);
    }
    if track_clicks || track_opens || track_content {
        body["settings"] = json!({
            "track_clicks": track_clicks,
            "track_opens": track_opens,
            "track_content": track_content,
        });
    }

    let resp = client.request_full("POST", "/email", &[], Some(&body))?;

    if ctx.json {
        let mut result = json!({ "status": "sent" });
        if let Some(id) = resp.header("x-message-id").filter(|s| !s.is_empty()) {
            result["message_id"] = json!(id);
        }
        return output::json(&result);
    }

    match resp.header("x-message-id").filter(|s| !s.is_empty()) {
        Some(id) => output::success(&format!("Email queued successfully. Message ID: {id}")),
        None => output::success("Email queued successfully."),
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::api::ENV_LOCK;
    use serde_json::Value;
    use std::thread::JoinHandle;

    struct Captured {
        method: String,
        url: String,
        auth: String,
        body: String,
    }

    fn mock_202(message_id: &str) -> (Client, JoinHandle<Captured>) {
        let server = tiny_http::Server::http("127.0.0.1:0").expect("bind mock server");
        let addr = format!("http://{}", server.server_addr());
        let mid = message_id.to_string();
        let handle = std::thread::spawn(move || {
            let mut req = server.recv().expect("receive request");
            let mut body = String::new();
            req.as_reader()
                .read_to_string(&mut body)
                .expect("read body");
            let cap = Captured {
                method: req.method().to_string(),
                url: req.url().to_string(),
                auth: req
                    .headers()
                    .iter()
                    .find(|h| h.field.equiv("Authorization"))
                    .map(|h| h.value.to_string())
                    .unwrap_or_default(),
                body,
            };
            let resp = tiny_http::Response::from_string(String::new())
                .with_status_code(202)
                .with_header(
                    tiny_http::Header::from_bytes(&b"x-message-id"[..], mid.as_bytes())
                        .expect("header"),
                );
            req.respond(resp).expect("respond");
            cap
        });
        let client = {
            let _guard = ENV_LOCK.lock().unwrap();
            std::env::set_var("MAILERSEND_API_BASE_URL", &addr);
            let client = Client::new("test-token-xyz".into(), false);
            std::env::remove_var("MAILERSEND_API_BASE_URL");
            client
        };
        (client, handle)
    }

    fn test_ctx(json: bool) -> Ctx {
        Ctx {
            profile: None,
            verbose: false,
            json,
        }
    }

    #[test]
    fn send_posts_full_body() {
        let (client, server) = mock_202("msg-abc-123");

        send(
            &test_ctx(false),
            &client,
            "sender@example.com",
            "Sender",
            "recipient@example.com",
            "Recipient",
            "cc@example.com",
            "bcc@example.com",
            "reply@example.com",
            "Hello",
            "plain body",
            "<b>bold</b>",
            "",
            "",
            "tmpl-1",
            Some(vec!["tag1".to_string(), "tag2".to_string()]),
            1700000000,
            true,
            true,
            true,
        )
        .expect("send");

        let cap = server.join().expect("server thread");
        assert_eq!(cap.method, "POST");
        assert_eq!(cap.url, "/email");
        assert_eq!(cap.auth, "Bearer test-token-xyz");

        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["from"]["email"], "sender@example.com");
        assert_eq!(body["from"]["name"], "Sender");
        assert_eq!(body["to"][0]["email"], "recipient@example.com");
        assert_eq!(body["to"][0]["name"], "Recipient");
        assert_eq!(body["cc"][0]["email"], "cc@example.com");
        assert_eq!(body["bcc"][0]["email"], "bcc@example.com");
        assert_eq!(body["reply_to"]["email"], "reply@example.com");
        assert_eq!(body["subject"], "Hello");
        assert_eq!(body["text"], "plain body");
        assert_eq!(body["html"], "<b>bold</b>");
        assert_eq!(body["template_id"], "tmpl-1");
        assert_eq!(body["tags"].as_array().map(Vec::len), Some(2));
        assert_eq!(body["send_at"], 1700000000);
        assert_eq!(body["settings"]["track_clicks"], true);
        assert_eq!(body["settings"]["track_opens"], true);
        assert_eq!(body["settings"]["track_content"], true);
    }

    #[test]
    fn send_reads_html_from_file() {
        let path = std::env::temp_dir().join(format!("ms-cli-email-{}.html", std::process::id()));
        std::fs::write(&path, "<h1>From File</h1>").expect("write temp file");

        let (client, server) = mock_202("msg-file-123");

        send(
            &test_ctx(false),
            &client,
            "sender@example.com",
            "",
            "test@example.com",
            "",
            "",
            "",
            "",
            "File test",
            "",
            "",
            path.to_str().expect("path"),
            "",
            "",
            None,
            0,
            false,
            false,
            false,
        )
        .expect("send");

        let cap = server.join().expect("server thread");
        let body: Value = serde_json::from_str(&cap.body).expect("json body");
        assert_eq!(body["html"], "<h1>From File</h1>");
        std::fs::remove_file(&path).ok();
    }

    #[test]
    fn send_json_output_succeeds() {
        let (client, server) = mock_202("msg-json-123");

        send(
            &test_ctx(true),
            &client,
            "sender@example.com",
            "",
            "test@example.com",
            "",
            "",
            "",
            "",
            "JSON test",
            "body",
            "",
            "",
            "",
            "",
            None,
            0,
            false,
            false,
            false,
        )
        .expect("send --json");
        server.join().expect("server thread");
    }

    #[test]
    fn send_missing_to_errors() {
        let client = {
            let _guard = ENV_LOCK.lock().unwrap();
            Client::new("test-token".into(), false)
        };

        let result = send(
            &test_ctx(false),
            &client,
            "",
            "",
            "",
            "",
            "",
            "",
            "",
            "No recipient",
            "",
            "",
            "",
            "",
            "",
            None,
            0,
            false,
            false,
            false,
        );

        assert!(result.is_err());
        assert_eq!(result.unwrap_err().to_string(), "--to is required");
    }
}
