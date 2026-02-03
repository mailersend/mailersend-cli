# CLAUDE.md

## Project

MailerSend CLI — a Go CLI and interactive TUI dashboard for the MailerSend API.

Module: `github.com/mailersend/mailersend-cli`
Go version: 1.25.5

## Build & Test

```bash
go build ./...          # build all packages
go test ./...           # run all tests
go build -o mailersend . # build binary
```

## Lint & Format

```bash
gofmt -w .              # format all Go files
golangci-lint run       # lint (used in CI and pre-commit hooks)
golangci-lint run --fix # lint with auto-fix
```

Always run `gofmt -w .` and `golangci-lint run` after making changes. CI runs both `golangci-lint run` and `go build ./...` on every push/PR.

## Project Structure

```
main.go                     # entry point
cmd/                        # cobra commands, one subdir per feature
  root.go                   # root command, registers all subcommands
  dashboard/                # TUI launcher
  email/, domain/, sms/...  # ~25 command groups
internal/
  config/                   # YAML config + multi-profile management
  sdkclient/                # SDK wrapper: retry, rate-limit, verbose logging
  output/                   # table/JSON/plain formatters
  prompt/                   # interactive prompts (charmbracelet/huh)
  cmdutil/                  # flag helpers, SDK client factory
  tui/                      # bubbletea TUI dashboard
    app.go                  # main app model
    keys.go                 # key bindings
    theme/                  # centralized color palette (AdaptiveColor)
    components/             # sidebar, table, statusbar, help, spinner, detail
    views/                  # domains, activity, analytics, messages, suppressions
    types/                  # shared message types and data structs
```

## Key Dependencies

- `github.com/mailersend/mailersend-go` — official SDK
- `github.com/spf13/cobra` — CLI framework
- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/lipgloss` — terminal styling
- `github.com/charmbracelet/huh` — interactive prompts

## Architecture Notes

- Commands follow pattern: `cmd/<feature>/<feature>.go` with cobra command, subcommands, `init()`, and handler functions.
- SDK client uses a custom `CLITransport` wrapping the HTTP client with retry (3 max, exponential backoff), rate-limit handling (429 + Retry-After), and verbose request logging.
- Config stored at `~/.config/mailersend/config.yaml` with multi-profile support. Token resolution: env var `MAILERSEND_API_TOKEN` > `--profile` flag > active profile > first profile.
- TUI colors are defined in `internal/tui/theme/theme.go` as `lipgloss.AdaptiveColor` values for light/dark terminal support. All TUI files reference theme vars, never hardcoded color strings. Use 256-palette values (16-255) for backgrounds and critical text to avoid remapping by terminal color schemes (Catppuccin, Solarized, etc.).
- Output supports `--json` flag for machine-readable output and respects `NO_COLOR` env var.

## Release

Tags matching `v*` trigger GoReleaser via `.github/workflows/release.yml`. Builds for Linux/macOS/Windows. Updates Homebrew cask and flake.nix automatically.

## Pre-commit Hooks

Configured via `lefthook.yml`: runs `golangci-lint run --fix` and `go build ./...` before each commit.
