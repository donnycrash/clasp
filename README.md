# CLASP — Claude Analytics & Standards Platform

A standalone CLI tool that collects Claude Code usage data from developer machines and uploads it to a company endpoint, while also syncing org-wide Claude Code configurations (rules, skills, hooks, settings) from a shared Git repo.

## What It Does

**📊 Insights Upload** — Reads Claude Code's local usage data (`~/.claude/`) and uploads it to your company's analytics endpoint:
- Token usage by model (Opus, Sonnet, Haiku)
- Session metadata (duration, tools used, languages, lines changed)
- Session quality metrics (outcomes, satisfaction, helpfulness)
- Configurable field-level redaction (hash project paths, omit prompts)

**🔄 Org Config Sync** — Pulls shared Claude Code configurations from a Git repo and installs them as a layered config that never overwrites local customizations:
- CLAUDE.md rules and guidelines
- Custom skills / slash commands
- Hooks and settings
- Tag-based opt-in for team-specific configs

**⏰ Scheduled Background Runs** — Installs as a macOS launchd job or Windows Scheduled Task that runs daily, uploading insights and syncing configs automatically.

## Install

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/donnycrash/clasp/main/scripts/install.sh | bash
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/donnycrash/clasp/main/scripts/install.ps1 | iex
```

### From Source

```bash
git clone https://github.com/donnycrash/clasp.git
cd clasp
go build -o clasp .
```

## Quick Start

```bash
# 1. Authenticate with GitHub
clasp auth login

# 2. Configure your endpoint
clasp config set endpoint https://insights.yourcompany.com/api/v1/upload

# 3. (Optional) Configure org config sync
clasp config set sync.repo git@github.com:yourorg/claude-org-config.git

# 4. Upload insights now
clasp upload

# 5. Sync org configs now
clasp sync

# 6. Install background scheduler (daily runs)
clasp install
```

## Commands

| Command | Description |
|---|---|
| `clasp upload` | Collect, redact, and upload Claude Code usage data |
| `clasp sync` | Pull latest org configs from shared Git repo |
| `clasp sync --diff` | Preview what would change without applying |
| `clasp run` | Upload + sync combined (what the scheduler calls) |
| `clasp auth login` | Authenticate (GitHub OAuth or API key) |
| `clasp auth login --provider apikey` | Authenticate with an API key instead |
| `clasp auth status` | Show current identity and auth state |
| `clasp auth logout` | Clear stored credentials |
| `clasp config show` | Print current configuration |
| `clasp config set <key> <value>` | Set a config value (supports dotted keys) |
| `clasp install` | Register as a background scheduled task |
| `clasp uninstall` | Remove background scheduled task |
| `clasp uninstall --purge` | Remove scheduled task and all config/data |
| `clasp status` | Show upload watermark, pending sessions, auth state |
| `clasp version` | Print version |

## Configuration

Config lives at `~/.config/clasp/config.yaml` (override with `CLASP_CONFIG_DIR` env var):

```yaml
# Where to send usage data
endpoint: "https://insights.yourcompany.com/api/v1/upload"

# How often the scheduler runs
schedule_interval: "24h"

# Where Claude Code stores its data (default: ~/.claude)
claude_data_dir: "~/.claude"

# Authentication
auth:
  provider: github         # or "apikey"
  github:
    client_id: "Iv1.abc123..."  # Your registered GitHub OAuth App
  apikey: {}

# Privacy — control what gets sent
redaction:
  project_path: hash       # SHA-256 hash (default)
  first_prompt: omit       # Remove entirely (default)
  brief_summary: omit      # Remove entirely (default)
  underlying_goal: omit    # Remove entirely (default)
  friction_detail: omit    # Remove entirely (default)
  # Options per field: "keep", "hash", "omit"

# Upload tuning
upload:
  batch_size: 50
  retry_max: 3
  retry_backoff: "30s"
  timeout: "30s"

# Org config sync
sync:
  repo: "git@github.com:yourorg/claude-org-config.git"
  branch: "main"
  auto_sync: true          # Sync on scheduled runs
  tags: [frontend]         # Only install configs tagged with these
```

## Data Sources

CLASP reads three types of data from `~/.claude/`:

| Source | Path | What It Contains |
|---|---|---|
| Stats | `stats-cache.json` | Daily token counts by model, message/session/tool counts, peak hours |
| Sessions | `usage-data/session-meta/*.json` | Per-session: duration, tools, languages, lines changed, git activity, errors |
| Facets | `usage-data/facets/*.json` | Session quality: goals, outcomes, satisfaction, helpfulness, summaries |

Only **new data since the last upload** is sent (tracked via a watermark file). Duplicate uploads are prevented.

## Privacy & Redaction

By default, CLASP applies sensible redaction before uploading:

| Field | Default | What It Does |
|---|---|---|
| `project_path` | `hash` | SHA-256 hash — groups sessions by project without revealing paths |
| `first_prompt` | `omit` | Removed entirely |
| `brief_summary` | `omit` | Removed entirely |
| `underlying_goal` | `omit` | Removed entirely |
| `friction_detail` | `omit` | Removed entirely |

You can override any field to `keep` (send as-is), `hash` (SHA-256), or `omit` (remove) in the config.

## Org Config Sync

CLASP can pull shared Claude Code configurations from a Git repo and install them without overwriting developers' local customizations.

### How It Works

```
┌─────────────────────────────┐
│ Developer's local configs   │  ← highest priority (never overwritten)
├─────────────────────────────┤
│ Org layer (managed by CLASP)│  ← synced from Git repo
├─────────────────────────────┤
│ Claude Code defaults        │  ← lowest priority
└─────────────────────────────┘
```

### Org Config Repo Structure

Create a repo with this structure:

```
claude-org-config/
  manifest.yaml              # Declares what to install
  claude-md/
    global.md                # Org-wide CLAUDE.md rules
    frontend.md              # Team-specific (opt-in via tags)
  skills/
    company-review/
      skill.md
  hooks/
    pre-commit.sh
  settings/
    base.json
```

### Manifest Format

```yaml
version: 1

claude_md:
  - source: claude-md/global.md
    scope: global
  - source: claude-md/frontend.md
    scope: global
    tags: [frontend]         # Only for devs with "frontend" tag

skills:
  - source: skills/company-review/

hooks:
  - source: hooks/pre-commit.sh
    event: pre-commit

settings:
  - source: settings/base.json
```

Developers opt into team-specific configs via `tags` in their local config:

```yaml
sync:
  tags: [frontend, mobile]
```

## Authentication

CLASP supports pluggable auth providers. Two ship out of the box:

### GitHub OAuth (default)

Uses the [Device Flow](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#device-flow) — works in terminals, SSH sessions, and headless environments:

```bash
clasp auth login
# Opens github.com/login/device — enter the displayed code
```

### API Key

Simple key-based auth for environments where OAuth isn't practical:

```bash
clasp auth login --provider apikey
# Prompts for your API key
```

### Adding Custom Providers

Implement the `Provider` interface in `internal/auth/` and register it. The upload pipeline just calls `GetAuthHeader()` and `GetIdentity()` — no other changes needed.

## Upload Payload

The JSON payload sent to your endpoint looks like this:

```json
{
  "metadata": {
    "tool_name": "clasp",
    "tool_version": "1.0.0",
    "os": "darwin",
    "arch": "arm64",
    "hostname_hash": "a1b2c3...",
    "github_username": "developer",
    "upload_timestamp": "2026-03-24T14:00:00Z",
    "batch_id": "uuid"
  },
  "stats_summary": {
    "period_start": "2026-03-23",
    "period_end": "2026-03-24",
    "daily_activity": [{ "date": "...", "message_count": 450, "session_count": 3, "tool_call_count": 85 }],
    "daily_model_tokens": [{ "date": "...", "tokens_by_model": { "claude-opus-4-6": 50000 } }],
    "model_usage": { "claude-opus-4-6": { "input_tokens": 1000, "output_tokens": 50000 } }
  },
  "sessions": [
    {
      "session_id": "...",
      "project_path": "hashed...",
      "duration_minutes": 29,
      "tool_counts": { "Bash": 9, "Read": 5, "Edit": 6 },
      "languages": { "TypeScript": 4 },
      "lines_added": 35,
      "lines_removed": 21,
      "facets": {
        "outcome": "mostly_achieved",
        "claude_helpfulness": "very_helpful"
      }
    }
  ]
}
```

## Development

```bash
# Build
go build -o clasp .

# Run tests (123 tests across 7 packages)
CGO_ENABLED=0 go test ./...

# Run with verbose output
CGO_ENABLED=0 go test ./... -v
```

### Project Structure

```
clasp/
  cmd/           CLI commands (cobra)
  internal/
    collector/   Parse Claude Code data files
    redactor/    Field-level privacy redaction
    uploader/    HTTP client with retry
    auth/        Pluggable auth providers (GitHub OAuth, API key)
    sync/        Git-based org config distribution
    watermark/   Track uploaded data to avoid duplicates
    config/      YAML config loading with defaults
    platform/    OS-specific scheduling (launchd, schtasks, systemd)
  scripts/       Install scripts for macOS/Windows
```

## License

MIT
