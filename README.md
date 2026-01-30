# MailerSend CLI

A command-line interface for the [MailerSend API](https://www.mailersend.com/). Send emails and SMS, manage domains, templates, webhooks, recipients, suppressions, and more — all from your terminal.

## Installation

### From source

Requires Go 1.25+.

```bash
git clone https://github.com/mailersend/mailersend-cli.git
cd mailersend-cli
go build -o mailersend .
```

Move the binary to somewhere on your `$PATH`:

```bash
sudo mv mailersend /usr/local/bin/
```

## Authentication

The CLI authenticates using MailerSend API keys. OAuth is not supported at this time.

Log in with your API token:

```bash
mailersend auth login
```

You'll be prompted to enter your MailerSend API token. You can generate one from your [MailerSend dashboard](https://www.mailersend.com/) under API Tokens. The token is stored locally in a config file.

Check auth status:

```bash
mailersend auth status
```

Log out:

```bash
mailersend auth logout
```

### Multiple profiles

You can manage multiple API tokens using profiles:

```bash
mailersend profile add --name staging
mailersend profile add --name production
mailersend profile list
mailersend profile switch staging
```

Use a specific profile for a single command:

```bash
mailersend domain list --profile production
```

### Environment variable

You can also set the API token via environment variable:

```bash
export MAILERSEND_API_TOKEN="mlsn.your_token_here"
```

## Global flags

Every command supports these flags:

| Flag | Description |
|------|-------------|
| `--json` | Output raw JSON instead of formatted tables |
| `--verbose`, `-v` | Print HTTP request and response details |
| `--profile <name>` | Use a specific auth profile |
| `--help`, `-h` | Show help for any command |

## Commands

### Email

```bash
# Send an email
mailersend email send \
  --from "sender@yourdomain.com" \
  --from-name "Sender Name" \
  --to "recipient@example.com" \
  --to-name "Recipient" \
  --subject "Hello" \
  --text "Plain text body" \
  --html "<h1>HTML body</h1>"

# Send from a file
mailersend email send \
  --from "sender@yourdomain.com" \
  --to "recipient@example.com" \
  --subject "Newsletter" \
  --html-file ./newsletter.html \
  --text-file ./newsletter.txt

# Send using a template
mailersend email send \
  --from "sender@yourdomain.com" \
  --to "recipient@example.com" \
  --template-id "x2p0347z969lzdrn"

# Schedule an email (unix timestamp)
mailersend email send \
  --from "sender@yourdomain.com" \
  --to "recipient@example.com" \
  --subject "Scheduled" \
  --text "This is scheduled" \
  --send-at 1735689600

# With tracking and tags
mailersend email send \
  --from "sender@yourdomain.com" \
  --to "recipient@example.com" \
  --subject "Tracked" \
  --text "Body" \
  --track-opens --track-clicks \
  --tags "campaign,welcome"
```

### Bulk Email

```bash
# Send bulk email from a JSON file
mailersend bulk-email send --file emails.json

# Check bulk email status
mailersend bulk-email status <bulk_email_id>
```

The JSON file should contain an array of email objects:

```json
[
  {
    "from": {"email": "sender@yourdomain.com"},
    "to": [{"email": "recipient1@example.com"}],
    "subject": "Bulk Email 1",
    "text": "Hello from bulk"
  },
  {
    "from": {"email": "sender@yourdomain.com"},
    "to": [{"email": "recipient2@example.com"}],
    "subject": "Bulk Email 2",
    "text": "Hello from bulk"
  }
]
```

### Domains

```bash
# List domains
mailersend domain list
mailersend domain list --limit 10

# Get domain details
mailersend domain get yourdomain.com

# Add a new domain
mailersend domain add --name yourdomain.com

# Show DNS records
mailersend domain dns yourdomain.com

# Verify domain
mailersend domain verify yourdomain.com

# Update domain settings
mailersend domain update-settings yourdomain.com --track-clicks --track-opens

# Delete a domain
mailersend domain delete yourdomain.com
```

### Recipients

```bash
# List recipients
mailersend recipient list --limit 20

# List recipients for a specific domain
mailersend recipient list --domain-id yourdomain.com

# Get recipient details
mailersend recipient get <recipient_id>

# Delete a recipient
mailersend recipient delete <recipient_id>
```

### Sender Identities

```bash
# List identities
mailersend identity list --limit 10
mailersend identity list --domain-id yourdomain.com

# Create an identity
mailersend identity create \
  --domain-id yourdomain.com \
  --name "Support" \
  --email "support@yourdomain.com"

# Get identity (by ID or email)
mailersend identity get <id>
mailersend identity get support@yourdomain.com

# Update an identity
mailersend identity update <id> --name "Customer Support"

# Delete an identity
mailersend identity delete <id>
```

### Templates

```bash
# List templates
mailersend template list
mailersend template list --limit 10 --domain-id yourdomain.com

# Get template details
mailersend template get <template_id>

# Delete a template
mailersend template delete <template_id>
```

### Webhooks

```bash
# List webhooks
mailersend webhook list --domain-id yourdomain.com

# Create a webhook
mailersend webhook create \
  --domain-id yourdomain.com \
  --name "My Webhook" \
  --url "https://example.com/webhook" \
  --events "activity.sent,activity.delivered"

# Get webhook details
mailersend webhook get <webhook_id>

# Update a webhook
mailersend webhook update <webhook_id> --name "Updated Webhook"

# Delete a webhook
mailersend webhook delete <webhook_id>
```

### Messages

```bash
# List messages
mailersend message list --limit 10

# Get message details
mailersend message get <message_id>

# List scheduled messages
mailersend message scheduled list --domain-id yourdomain.com

# Get scheduled message
mailersend message scheduled get <message_id>

# Cancel a scheduled message
mailersend message scheduled delete <message_id>
```

### Activity

```bash
# List activity for a domain
mailersend activity list --domain-id yourdomain.com

# Get activity details
mailersend activity get <activity_id>
```

### Analytics

```bash
# Analytics by date
mailersend analytics date --domain-id yourdomain.com --date-from 2025-01-01 --date-to 2025-01-31

# Analytics by country
mailersend analytics country --domain-id yourdomain.com --date-from 2025-01-01 --date-to 2025-01-31

# Analytics by user agent name
mailersend analytics ua-name --domain-id yourdomain.com --date-from 2025-01-01 --date-to 2025-01-31

# Analytics by user agent type
mailersend analytics ua-type --domain-id yourdomain.com --date-from 2025-01-01 --date-to 2025-01-31
```

### Suppressions

Manage blocklist, hard bounces, spam complaints, unsubscribes, and on-hold entries.

```bash
# List suppressions (works for all types)
mailersend suppression blocklist list --limit 10
mailersend suppression hard-bounces list --limit 10
mailersend suppression spam-complaints list --limit 10
mailersend suppression unsubscribes list --limit 10
mailersend suppression on-hold list --limit 10

# Filter by domain
mailersend suppression blocklist list --domain-id yourdomain.com

# Add to blocklist (by recipient email)
mailersend suppression blocklist add --domain-id yourdomain.com --recipients "spam@example.com"

# Add to blocklist (by pattern)
mailersend suppression blocklist add --domain-id yourdomain.com --patterns "*@spamdomain.com"

# Add hard bounce / spam complaint / unsubscribe
mailersend suppression hard-bounces add --domain-id yourdomain.com --recipients "bounce@example.com"
mailersend suppression spam-complaints add --domain-id yourdomain.com --recipients "spam@example.com"
mailersend suppression unsubscribes add --domain-id yourdomain.com --recipients "unsub@example.com"

# Delete specific entries
mailersend suppression blocklist delete --ids id1,id2

# Delete all entries for a domain
mailersend suppression blocklist delete --all --domain-id yourdomain.com
```

### Inbound Routes

```bash
# List inbound routes
mailersend inbound list --limit 10
mailersend inbound list --domain-id yourdomain.com

# Create an inbound route
mailersend inbound create \
  --domain-id yourdomain.com \
  --name "My Inbound Route" \
  --match-filter-type match_all \
  --inbound-domain yourdomain.com \
  --inbound-priority 0 \
  --catch-filter-type catch_all \
  --forwards "https://example.com/inbound"

# Get route details
mailersend inbound get <route_id>

# Update a route
mailersend inbound update <route_id> --name "Updated Route"

# Delete a route
mailersend inbound delete <route_id>
```

### API Tokens

```bash
# List tokens
mailersend token list --limit 10

# Get token details
mailersend token get <token_id>

# Create a token
mailersend token create \
  --name "My Token" \
  --domain-id yourdomain.com \
  --scopes "email_full,domains_read"

# Update token name
mailersend token update <token_id> --name "Renamed Token"

# Pause / unpause a token
mailersend token update-status <token_id> --status pause
mailersend token update-status <token_id> --status unpause

# Delete a token
mailersend token delete <token_id>
```

### Account Users

```bash
# List users
mailersend user list

# Get user details
mailersend user get <user_id>

# Update a user
mailersend user update <user_id> --role admin

# Delete a user
mailersend user delete <user_id>
```

### User Invites

```bash
# List invites
mailersend user invite list

# Create an invite
mailersend user invite create --email "newuser@example.com" --role "custom"

# Get invite details
mailersend user invite get <invite_id>

# Resend an invite
mailersend user invite resend <invite_id>

# Cancel an invite
mailersend user invite cancel <invite_id>
```

### SMTP Users

All SMTP commands require `--domain-id`.

```bash
# List SMTP users
mailersend smtp list --domain-id yourdomain.com

# Get SMTP user details
mailersend smtp get <smtp_user_id> --domain-id yourdomain.com

# Create an SMTP user
mailersend smtp create --domain-id yourdomain.com --name "My SMTP User"

# Update an SMTP user
mailersend smtp update <smtp_user_id> --domain-id yourdomain.com --name "Updated SMTP"

# Delete an SMTP user
mailersend smtp delete <smtp_user_id> --domain-id yourdomain.com
```

### API Quota

```bash
mailersend quota
```

### Email Verification

```bash
# Verify a single email
mailersend verification verify user@example.com

# Verify asynchronously
mailersend verification verify-async user@example.com

# Check async verification status
mailersend verification status <verification_id>

# List verification lists
mailersend verification list list
```

### SMS

> SMS commands require SMS to be enabled on your MailerSend account.

#### Send SMS

```bash
mailersend sms send --from "+1234567890" --to "+0987654321" --text "Hello from CLI"
```

#### SMS Messages

```bash
mailersend sms message list --limit 10
mailersend sms message get <message_id>
```

#### SMS Activity

```bash
mailersend sms activity list --limit 10
mailersend sms activity list --sms-number-id <id> --date-from 2025-01-01 --date-to 2025-12-31
```

#### SMS Phone Numbers

```bash
mailersend sms number list --limit 10
mailersend sms number get <number_id>
mailersend sms number update <number_id> --paused
mailersend sms number update <number_id> --paused=false
mailersend sms number delete <number_id>
```

#### SMS Recipients

```bash
mailersend sms recipient list --limit 10
mailersend sms recipient get <recipient_id>
mailersend sms recipient update <recipient_id> --status opt_out
```

#### SMS Inbound Routes

```bash
mailersend sms inbound list --limit 10
mailersend sms inbound create \
  --sms-number-id <id> \
  --name "My SMS Route" \
  --forward-url "https://example.com/sms-hook"
mailersend sms inbound get <route_id>
mailersend sms inbound update <route_id> --name "Updated SMS Route"
mailersend sms inbound delete <route_id>
```

#### SMS Webhooks

```bash
mailersend sms webhook list --sms-number-id <id>
mailersend sms webhook create \
  --sms-number-id <id> \
  --name "My SMS Webhook" \
  --url "https://example.com/sms-webhook" \
  --events "sms.sent,sms.delivered"
mailersend sms webhook get <webhook_id>
mailersend sms webhook update <webhook_id> --name "Updated SMS Webhook"
mailersend sms webhook delete <webhook_id>
```

## Domain name resolution

Any flag that accepts `--domain-id` will accept both a domain name (e.g. `yourdomain.com`) or a raw domain ID (e.g. `q3enl6kk0z042vwr`). When a domain name is provided, it is automatically resolved to the corresponding ID.

## Shell completion

Generate shell completions for your shell:

```bash
# Bash
source <(mailersend completion bash)

# Zsh
mailersend completion zsh > "${fpath[1]}/_mailersend"

# Fish
mailersend completion fish | source

# PowerShell
mailersend completion powershell | Out-String | Invoke-Expression
```

## JSON output

Add `--json` to any command to get raw JSON output, useful for scripting:

```bash
# Pipe to jq
mailersend domain list --json | jq '.[].name'

# Extract an ID
mailersend identity create --domain-id yourdomain.com --name "Test" --email "test@yourdomain.com" --json | jq -r '.data.id'
```

## License

See [LICENSE](LICENSE) for details.
