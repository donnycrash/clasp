# Plan: CLASP — Claude Analytics & Standards Platform

## Context

Lumenalta needs two things for their Claude Code rollout:

1. **Usage visibility** — Company-wide insight into how developers use Claude Code (tokens, sessions, productivity metrics)
2. **Config distribution** — A way to share CLAUDE.md rules, custom skills, hooks, and settings across the org so every dev gets the company's standard Claude Code setup

This is a **new standalone Go binary** — separate from clanopy — that devs install on their Mac or PC. It handles both uploading insights data AND syncing org-wide Claude Code configurations from a shared Git repo.

## Project Location

New directory: `/Users/donovancrewe/Documents/LM/clasp/`

## Architecture

**Cron/launchd-only approach** — no persistent daemon. The OS scheduler invokes `clasp upload` on a schedule. The binary collects data, redacts sensitive fields, uploads to the endpoint, updates a watermark, and exits.

- macOS: `launchd` plist in `~/Library/LaunchAgents/`
- Windows: Scheduled Task via `schtasks`
- Developer can also run `clasp upload` manually anytime

## Project Structure

```
clasp/
  main.go
  cmd/
    root.go              # cobra root command
    upload.go            # collect → redact → upload pipeline
    sync.go              # sync org configs from shared Git repo
    run.go               # "run" command: upload + sync (used by scheduler)
    auth.go              # auth login / status / logout
    config.go            # config show / set
    install.go           # install / uninstall OS scheduler
    status.go            # show watermark, last upload, pending sessions
    version.go
  internal/
    collector/
      stats.go           # Parse stats-cache.json
      sessions.go        # Parse session-meta/*.json
      facets.go          # Parse facets/*.json
      collector.go       # Orchestrate: join sessions+facets, apply watermark
    redactor/
      redactor.go        # Field-level redaction engine (keep/hash/omit)
      config.go          # Redaction rule types and defaults
    uploader/
      client.go          # HTTP client with retry + exponential backoff
      payload.go         # Payload structs and JSON marshaling
    auth/
      provider.go        # Provider interface + Identity struct
      registry.go        # Provider registry (map of name → constructor)
      github.go          # GitHub OAuth device flow provider
      apikey.go          # API key provider
      token.go           # Token storage (keychain / credential store)
    sync/
      repo.go            # Git clone/pull of org config repo
      layer.go           # Layered config installer (org layer + local overrides)
      manifest.go        # Parse org repo manifest (what to install where)
      diff.go            # Show what changed on sync
    watermark/
      watermark.go       # Track uploaded sessions + stats date cursor
    config/
      config.go          # App config (YAML loading)
      paths.go           # Platform-specific path resolution
    platform/
      launchd.go         # macOS plist generation + launchctl
      taskscheduler.go   # Windows schtasks
      service.go         # Common interface
  scripts/
    install.sh           # macOS bootstrap (download + launchd setup)
    install.ps1          # Windows bootstrap (download + scheduled task)
  .goreleaser.yml
  go.mod
  CLAUDE.md
```

## Data Sources

All in `~/.claude/`:

1. **`stats-cache.json`** — Daily aggregated metrics: tokens by model (opus/sonnet/haiku), message counts, session counts, tool call counts, peak hours, cost USD
2. **`usage-data/session-meta/*.json`** — Per-session: duration, tool counts, languages, lines added/removed, files modified, git commits/pushes, tool errors, feature flags (MCP, web search, etc.)
3. **`usage-data/facets/*.json`** — Session quality: goal categories, outcome, satisfaction, helpfulness, session type, summaries

## CLI Commands

| Command | Description |
|---|---|
| `clasp upload` | One-shot: collect, redact, upload, update watermark |
| `clasp sync` | Pull latest org configs from shared Git repo |
| `clasp sync --diff` | Show what would change without applying |
| `clasp run` | Upload + sync (this is what the scheduler calls) |
| `clasp auth login` | Auth with configured provider (GitHub/API key) |
| `clasp auth login --provider <name>` | Auth with specific provider |
| `clasp auth status` | Show current identity |
| `clasp auth logout` | Clear stored credentials |
| `clasp config show` | Print current config |
| `clasp config set <key> <value>` | Set a config value |
| `clasp install` | Set up OS-level scheduled task (launchd/schtasks) |
| `clasp uninstall` | Remove scheduled task + optionally config |
| `clasp status` | Show watermark, last upload, sync status, pending sessions |
| `clasp version` | Print version |

## Authentication

**Pluggable auth provider interface** — ships with GitHub OAuth and API Key, designed so new providers (SSO/OIDC, SAML, etc.) can be added without changing the core.

```go
// internal/auth/provider.go
type Provider interface {
    Name() string                          // "github", "apikey", etc.
    Login(ctx context.Context) error       // Interactive login flow
    GetIdentity() (Identity, error)        // Who is this developer?
    GetAuthHeader() (string, error)        // "Bearer xxx" or "X-API-Key xxx"
    Logout() error                         // Clear stored credentials
    IsAuthenticated() bool                 // Check if valid creds exist
}

type Identity struct {
    Username    string // display name / GitHub username
    Email       string // optional
    ProviderID  string // unique ID from the provider
    Provider    string // "github", "apikey", etc.
}
```

**Shipped providers:**

1. **GitHub OAuth Device Flow** (RFC 8628) — default
   - `auth login --provider github` (or just `auth login`)
   - Posts to `https://github.com/login/device/code`, user enters code at `github.com/login/device`
   - Polls for token, calls `GET https://api.github.com/user` for identity
   - Token stored in OS keychain (macOS Keychain / Windows Credential Manager), fallback to encrypted file

2. **API Key** — simple fallback
   - `auth login --provider apikey`
   - Prompts for key, stores in keychain
   - Identity derived from key (backend maps key → developer)
   - Sent as `X-API-Key` header

**Config selects the provider:**
```yaml
auth:
  provider: github  # or "apikey"
  github:
    client_id: "Iv1.abc123..."  # company's registered GitHub OAuth App
  apikey: {}  # no extra config needed
```

**Adding a new provider:** Implement the `Provider` interface, register it in `internal/auth/registry.go`. No changes to upload pipeline — it just calls `provider.GetAuthHeader()` and `provider.GetIdentity()`.

## Config

Location: `~/.config/clasp/config.yaml`

```yaml
endpoint: "https://insights.example.com/api/v1/upload"
schedule_interval: "24h"
claude_data_dir: "~/.claude"

auth:
  provider: github         # or "apikey"
  github:
    client_id: "Iv1.abc123..."
  apikey: {}

redaction:
  project_path: hash        # SHA-256 hash
  first_prompt: omit        # remove entirely
  brief_summary: omit
  underlying_goal: omit
  friction_detail: omit
  # Any unlisted field defaults to "keep"

upload:
  batch_size: 50
  retry_max: 3
  retry_backoff: "30s"
  timeout: "30s"

sync:
  repo: "git@github.com:lumenalta/claude-org-config.git"  # or HTTPS
  branch: "main"
  auto_sync: true          # sync on scheduled runs
  local_cache: "~/.config/clasp/org-config"      # where the repo is cloned
```

## Redaction System

Field-level, compile-time typed. Three actions:
- **keep** — send as-is (default for most fields)
- **hash** — SHA-256 hex string (default for `project_path`)
- **omit** — empty string (default for `first_prompt`, `brief_summary`, `underlying_goal`, `friction_detail`)

Explicit struct with one field per redactable property. No reflection magic — fully auditable.

## Watermark

File: `~/.config/clasp/watermark.json`

```json
{
  "last_upload_time": "2026-03-24T02:00:00Z",
  "stats_cache_uploaded_through_date": "2026-03-22",
  "session_ids_uploaded": {
    "uuid1": "2026-03-24T02:00:00Z",
    "uuid2": "2026-03-24T02:00:00Z"
  }
}
```

- Sessions: skip UUIDs already in map; compact entries older than 90 days
- Stats: upload daily entries after `stats_cache_uploaded_through_date`
- Watermark updated atomically (write temp → rename) only on successful upload

## Upload Payload Format

```json
{
  "metadata": {
    "tool_name": "clasp",  // Claude Analytics & Standards Platform
    "tool_version": "1.0.0",
    "os": "darwin",
    "arch": "arm64",
    "hostname_hash": "a1b2c3...",
    "github_username": "donovancrewe",
    "upload_timestamp": "2026-03-24T14:00:00Z",
    "batch_id": "uuid"
  },
  "stats_summary": {
    "period_start": "2026-03-23",
    "period_end": "2026-03-24",
    "daily_activity": [...],
    "daily_model_tokens": [...],
    "model_usage": { "claude-opus-4-6": { "input_tokens": ..., "output_tokens": ..., ... } }
  },
  "sessions": [
    {
      "session_id": "...",
      "project_path": "hashed...",
      "start_time": "...",
      "duration_minutes": 29,
      "tool_counts": {"Bash": 9, "Read": 5},
      "languages": {"TypeScript": 4},
      "lines_added": 35,
      "lines_removed": 21,
      "input_tokens": 77,
      "output_tokens": 6814,
      "facets": {
        "outcome": "mostly_achieved",
        "claude_helpfulness": "very_helpful",
        "session_type": "iterative_refinement",
        ...
      },
      ...
    }
  ]
}
```

Sessions and facets are **joined** — facets nested inside the session object.

## Upload Error Handling

- Exponential backoff: 30s → 60s → 120s (3 retries)
- `X-Batch-ID` header for server-side idempotency
- 409 Conflict treated as success (already uploaded)
- 5xx: retry. 4xx (except 409): fail immediately
- Failures logged to `~/.config/clasp/upload.log`
- Watermark only updated for successfully uploaded sessions

## Org Config Sync

### Concept: Layered Configuration

Claude Code already supports a layered config model (project CLAUDE.md → user settings). We add an **org layer** that sits between global defaults and user customizations:

```
┌─────────────────────────────┐
│ User's local customizations │  ← ~/.claude/settings.json, project CLAUDE.md (highest priority)
├─────────────────────────────┤
│ Org layer (managed by sync) │  ← ~/.claude/org/ (synced from Git repo)
├─────────────────────────────┤
│ Claude Code defaults        │  (lowest priority)
└─────────────────────────────┘
```

Local customizations are **never overwritten**. The org layer is a separate directory that Claude Code reads alongside the user's own config. On conflict, local wins.

### Org Config Repo Structure

The shared Git repo (e.g., `github.com/lumenalta/claude-org-config`) has this structure:

```
claude-org-config/
  manifest.yaml              # declares what gets installed and where
  claude-md/
    global.md                # org-wide CLAUDE.md rules (appended to user's)
    frontend.md              # team-specific rules (opt-in via manifest)
    backend.md
  skills/
    company-review/          # custom skills distributed to all devs
      skill.md
    deploy-check/
      skill.md
  hooks/
    pre-commit.sh            # shared hook scripts
    post-tool.sh
  settings/
    base.json                # org base settings (merged under user's)
```

### Manifest Format

```yaml
# manifest.yaml — declares what to install and where
version: 1

# CLAUDE.md content — appended to ~/.claude/CLAUDE.md (org section)
claude_md:
  - source: claude-md/global.md
    scope: global            # applies to all projects
  - source: claude-md/frontend.md
    scope: global
    tags: [frontend]         # only installed if dev opts in to "frontend" tag

# Skills — copied to ~/.claude/plugins/ or equivalent
skills:
  - source: skills/company-review/
  - source: skills/deploy-check/

# Hooks — registered in org settings layer
hooks:
  - source: hooks/pre-commit.sh
    event: pre-commit

# Settings — merged as org base layer
settings:
  - source: settings/base.json
```

### How Sync Works

1. `clasp sync` (or auto-triggered by `run`)
2. **Git pull**: Clone repo to `~/.config/clasp/org-config/` (first time) or `git pull` (subsequent)
3. **Parse manifest**: Read `manifest.yaml` to know what to install
4. **Install org layer**:
   - CLAUDE.md content → written to `~/.claude/org/CLAUDE.md` (Claude Code reads this as an additional context file)
   - Skills → copied to `~/.claude/org/skills/`
   - Settings → written to `~/.claude/org/settings.json`
   - Hooks → scripts copied to `~/.claude/org/hooks/`, registered in org settings
5. **Diff output**: Print what changed (new skills added, rules updated, etc.)
6. **No local files touched**: Everything goes into `~/.claude/org/` — the user's own `~/.claude/settings.json`, CLAUDE.md, etc. are untouched

### Tag-Based Opt-In

Devs can opt into specific config tags in their local config:

```yaml
# ~/.config/clasp/config.yaml
sync:
  repo: "git@github.com:lumenalta/claude-org-config.git"
  tags: [frontend, mobile]   # only install configs tagged with these
```

This lets teams share team-specific rules without forcing them on everyone.

### Claude Code Integration

For Claude Code to read the org layer, the tool needs to ensure Claude Code picks up configs from `~/.claude/org/`. Claude Code's CLAUDE.md resolution already reads from `~/.claude/CLAUDE.md` — the sync tool appends an `# Org Configuration` section to it (or uses the project-level include mechanism if available). For settings, Claude Code reads `settings.json` — the org layer would need to be merged or referenced.

**Pragmatic approach for v1:** The sync tool writes org CLAUDE.md rules directly into `~/.claude/CLAUDE.md` wrapped in clear markers:

```markdown
<!-- BEGIN ORG CONFIG - managed by clasp, do not edit -->
... org rules here ...
<!-- END ORG CONFIG -->
```

On each sync, the section between markers is replaced. User content outside the markers is preserved. Same approach for settings: merge org settings into `settings.json` under a clearly marked org key, or maintain a separate settings file if Claude Code supports it.

## Installation

**macOS (`install.sh` / `clasp install`):**
- Downloads binary from GitHub Releases to `~/.local/bin/`
- Generates launchd plist at `~/Library/LaunchAgents/com.lumenalta.clasp.plist`
- Runs `launchctl load`

**Windows (`install.ps1` / `clasp install`):**
- Downloads .exe
- Creates scheduled task via `schtasks /create /tn "CLASP" /sc daily /st 02:00`

**Uninstall:** `clasp uninstall` removes the scheduled task and optionally the config directory.

## Reference Files

- `experiments/clanopy/.goreleaser.yml` — template for GoReleaser config (add `windows` to goos)
- `experiments/clanopy/install.sh` — pattern for OS/arch detection, GitHub Releases download
- `experiments/clanopy/internal/auth/oauth.go` — reference for OAuth flow (adapt to device flow)
- `experiments/clanopy/internal/config/paths.go` — pattern for platform-aware path resolution

## Dependencies

Minimal:
- `github.com/spf13/cobra` — CLI framework
- `gopkg.in/yaml.v3` — config parsing
- `log/slog` — structured logging (stdlib)
- `os/exec` — for `git clone`/`git pull` (stdlib, no git library needed)
- Everything else from Go stdlib (net/http, crypto/sha256, encoding/json, os)

## Implementation Order

### Phase 1: Insights Upload (steps 1-8)
1. **Scaffold** — `go.mod`, `main.go`, `cmd/root.go`, `cmd/version.go`, `.goreleaser.yml`
2. **Config** — `internal/config/` (YAML loading, path resolution, defaults)
3. **Collector** — `internal/collector/` (parse stats-cache, session-meta, facets; join sessions+facets)
4. **Watermark** — `internal/watermark/` (read/write, atomic update, compaction)
5. **Redactor** — `internal/redactor/` (field-level keep/hash/omit)
6. **Uploader** — `internal/uploader/` (HTTP client, payload structs, retry, batch splitting)
7. **Upload command** — `cmd/upload.go` (wire collector → redactor → uploader pipeline)
8. **Auth** — `internal/auth/` + `cmd/auth.go` (pluggable provider interface, GitHub device flow, API key)

### Phase 2: Org Config Sync (steps 9-11)
9. **Sync engine** — `internal/sync/repo.go` (git clone/pull), `manifest.go` (parse manifest.yaml)
10. **Layer installer** — `internal/sync/layer.go` (install CLAUDE.md, skills, hooks, settings into org layer with marker-based injection)
11. **Sync + Run commands** — `cmd/sync.go` (manual sync with --diff), `cmd/run.go` (upload + sync combined)

### Phase 3: Platform & Distribution (steps 12-15)
12. **Install/Uninstall** — `internal/platform/` + `cmd/install.go` (launchd plist calls `run` not just `upload`)
13. **Status + Config commands** — `cmd/status.go`, `cmd/config.go`
14. **Install scripts** — `scripts/install.sh`, `scripts/install.ps1`
15. **GoReleaser** — `.goreleaser.yml` for cross-platform binary releases

## Verification

1. **Unit tests:** `go test ./...` — collector parsing, redaction logic, watermark, payload marshaling, manifest parsing
2. **Upload test:** Create test data in temp `~/.claude/`, run `clasp upload` against httptest server, inspect payload JSON
3. **Auth test:** Run `clasp auth login`, verify identity returned and token stored
4. **Sync test:** Create a test org config repo (local git repo), run `clasp sync`, verify:
   - Org CLAUDE.md rules appear between markers in `~/.claude/CLAUDE.md`
   - Skills copied to correct location
   - Settings merged correctly
   - User's local files outside markers are untouched
5. **Idempotency test:** Run sync twice, verify no duplicate content
6. **Tag filtering test:** Set `tags: [frontend]`, verify only frontend-tagged configs are installed
7. **Install test:** Run `clasp install`, verify launchd plist exists and runs `run` command
8. **Redaction test:** Configure `project_path: hash`, verify hashed paths in payload
9. **Watermark test:** Run upload twice, verify no duplicate sessions
