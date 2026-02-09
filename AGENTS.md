<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-01-30 | Updated: 2026-02-08 -->

# mnemo

## Purpose

**Memory for AI-assisted development** — Indexes AI coding sessions from 12+ tools (Claude Code, OpenCode, Gemini CLI, Cursor, etc.) into a unified, searchable SQLite database with FTS5 full-text search.

**Status**: Active development, Production-ready
**Version**: 1.3.0

## Key Files

| File | Description |
|------|-------------|
| `main.go` | Entry point — CLI initialization |
| `go.mod` | Go module definition (Go 1.23+) |
| `README.md` | Project overview, install, usage |
| `CONTRIBUTING.md` | Development setup and contribution guide |
| `CHANGELOG.md` | Version history and releases |
| `LICENSE` | MIT License (17588691 CANADA INC.) |

## Project Structure

```
mnemo/
├── main.go
├── cmd/                         # CLI commands (cobra)
│   ├── root.go                  # Root command + configure
│   ├── index.go                 # Indexing orchestrator + onboarding
│   ├── index_helpers.go         # Shared helpers (truncate, inferProvider, etc.)
│   ├── index_claude.go          # Claude Code adapter (JSONL)
│   ├── index_opencode.go        # OpenCode adapter (JSON)
│   ├── index_gemini.go          # Gemini CLI adapter (JSON)
│   ├── index_cursor.go          # Cursor adapter (SQLite)
│   ├── index_codex.go           # Codex CLI adapter (JSONL)
│   ├── index_amp.go             # Amp adapter (JSON + usage ledger)
│   ├── index_crush.go           # Crush adapter (SQLite)
│   ├── index_cline.go           # Cline/Roo/Kilo Code adapter (JSON)
│   ├── index_kiro.go            # Kiro adapter (JSON)
│   ├── index_antigravity.go     # Antigravity adapter (JSONL)
│   ├── index_vscode.go          # VS Code AI chat adapter (SQLite)
│   ├── search.go                # Full-text search command
│   ├── serve.go                 # MCP server (4 tools: search, context, recent, tools)
│   ├── blocks.go                # 5-hour usage block display
│   ├── projects.go              # Project management
│   ├── tools.go                 # Tool detection + path helpers
│   ├── add.go                   # Custom path indexing
│   ├── install.go               # Plugin installer
│   ├── context.go               # Context generation
│   ├── recent.go                # Recent sessions display
│   ├── status.go                # System status display
│   ├── version.go               # Version info
│   └── onboarding.go            # First-run experience
├── internal/
│   ├── db/                      # SQLite database layer
│   │   ├── sqlite.go            # Schema, migrations, init, execer interface
│   │   ├── messages.go          # Message CRUD (with Tx variants)
│   │   ├── sessions.go          # Session CRUD + typed RecentSession queries
│   │   ├── search.go            # FTS5 search + BM25 composite ranking
│   │   ├── projects.go          # Project discovery + classification
│   │   ├── token_usage.go       # Token/cost tracking + typed stats structs
│   │   └── blocks.go            # 5-hour session blocks + usage stats
│   └── tui/                     # Bubble Tea TUI components
│       ├── styles.go            # Catppuccin color palette + shared styles
│       └── projects.go          # Interactive project selector
├── proxy/                       # HTTP proxy for Claude API context injection
│   └── server.go                # Intercepts API calls, injects mnemo context
├── docs/                        # Documentation
├── assets/                      # Media assets
└── scripts/                     # Build and automation scripts
```

## For AI Agents

### Working In This Directory

1. **Adding CLI commands**: Create new file in `cmd/` following cobra pattern
2. **Adding a tool adapter**: Create `cmd/index_<tool>.go`, wire into orchestrator in `cmd/index.go`
3. **Database changes**: Modify relevant file in `internal/db/` (schema changes go in `sqlite.go`)
4. **Testing**: Run `go test ./...` before committing
5. **Building**: Run `go build -o /dev/null .` to verify compilation

### Architecture

```
CLI Commands (cmd/)
  ↓
Tool Adapters (cmd/index_*.go)
  ↓  parse JSONL / JSON / SQLite → atomic transactions
internal/db/
  ├── sqlite.go        → Schema + init + execer interface + BeginTx
  ├── messages.go      → Insert/delete messages (DB + Tx variants)
  ├── sessions.go      → Session tracking (DB + Tx variants)
  ├── search.go        → FTS5 full-text search + BM25 ranking
  ├── projects.go      → Project discovery + classification
  ├── token_usage.go   → Token/cost accounting + typed stats
  └── blocks.go        → Usage block analysis
  ↓
SQLite Database (~/.mnemo/mnemo.db)
```

### Key Design Patterns

- **Cobra CLI**: All commands use cobra framework
- **Bubble Tea TUI**: Interactive experiences use charmbracelet/bubbletea
- **SQLite + FTS5**: Single-file database with full-text search and BM25 ranking
- **One adapter per file**: Each tool gets its own `cmd/index_<tool>.go`
- **MCP Integration**: Model Context Protocol server for Claude Desktop/Cursor
- **Pure Go SQLite**: modernc.org/sqlite — no CGO, no system dependencies
- **execer interface**: Abstracts `*sql.DB` and `*sql.Tx` so insert/delete helpers work with both
- **Atomic transactions**: All indexers wrap delete+insert in a transaction via `BeginTx()` to prevent data loss from partial writes
- **Typed returns**: DB query functions return typed structs (`RecentSession`, `UsageStats`, `TokenStats`, etc.) instead of `map[string]interface{}`
- **Scan error logging**: All `rows.Scan` errors are logged via `log.Printf` before continuing
- **WAL mode**: Single-writer via `db.SetMaxOpenConns(1)`, WAL for concurrent reads
- **Snippet delimiters**: `⟪` / `⟫` (Unicode) to avoid XML injection

### Transaction Pattern

All indexers use this pattern for atomic writes:

```go
tx, err := db.BeginTx()
if err != nil { return }
defer func() { _ = tx.Rollback() }()

db.TxDeleteSessionMessages(tx, sessionID)
// ... insert messages ...
db.TxInsertSession(tx, session)

if err := tx.Commit(); err != nil { return }
```

Multi-session indexers (cursor, vscode, codex_history) use a closure pattern:
```go
for _, session := range sessions {
    func() {
        tx, _ := db.BeginTx()
        defer func() { _ = tx.Rollback() }()
        // ... per-session writes ...
        tx.Commit()
    }()
}
```

### Testing

```bash
go test ./...           # All tests
go test -cover ./...    # With coverage
go test ./internal/db/  # Specific package
go test -v -race ./...  # Verbose with race detection
```

## CLI Commands

| Command | Purpose | Example |
|---------|---------|---------|
| `mnemo index` | Index all AI sessions | `mnemo index --force` |
| `mnemo search` | Full-text search | `mnemo search "authentication"` |
| `mnemo recent` | Show recent sessions | `mnemo recent --days=7` |
| `mnemo context` | Generate project context | `mnemo context my-project` |
| `mnemo tools` | List detected AI tools | `mnemo tools` |
| `mnemo blocks` | Show 5-hour usage blocks | `mnemo blocks` |
| `mnemo projects` | Manage tracked projects | `mnemo projects` |
| `mnemo add` | Index a custom path | `mnemo add ~/docs` |
| `mnemo serve` | Start MCP server | `mnemo serve` |
| `mnemo install` | Install plugins/MCP config | `mnemo install claude-code` |
| `mnemo status` | Show system status | `mnemo status` |
| `mnemo version` | Print version info | `mnemo version` |

## Dependencies

| Dependency | Purpose |
|------------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/charmbracelet/bubbletea` | Terminal UI framework |
| `github.com/charmbracelet/lipgloss` | Terminal styling |
| `modernc.org/sqlite` | Pure Go SQLite (no CGO) |
| `github.com/mark3labs/mcp-go` | Model Context Protocol |

## Supported AI Tools

### Standalone Tools

| Tool | Status | Format | Session Location |
|------|--------|--------|------------------|
| Claude Code | Full support | JSONL | `~/.claude/projects/` |
| OpenCode | Full support | JSON | `~/.local/share/opencode/` |
| Gemini CLI | Full support | JSON | `~/.gemini/sessions/` |
| Cursor | Full support | SQLite | `~/Library/Application Support/Cursor/User/globalStorage/` |
| Crush | Full support | SQLite | `~/.crush/crush.db` |
| Kiro | Full support | JSON | `~/Library/Application Support/Kiro/` |
| Antigravity | Full support | JSONL | `~/.gemini/antigravity/code_tracker/` |
| Amp | Full support | JSON | `~/.local/share/amp/threads/` |
| Codex | Full support | JSONL | `~/.codex/sessions/` |

### VS Code Extensions (scans all IDEs)

| Extension | Format | IDEs Scanned |
|-----------|--------|--------------|
| Kilo Code | JSON | Code, Cursor, Windsurf, VSCodium, Antigravity, Kiro, Trae |
| Cline | JSON | Code, Cursor, Windsurf, VSCodium, Antigravity, Kiro, Trae |
| Roo Code | JSON | Code, Cursor, Windsurf, VSCodium, Antigravity, Kiro, Trae |

### Coming Soon

Windsurf (Protocol Buffers), Aider (markdown), GitHub Copilot (SQLite)

## Build & Release

```bash
go build -o mnemo .     # Build locally
./mnemo version         # Check version
```

Release process:
1. Update `CHANGELOG.md`
2. Tag version: `git tag v1.x.x`
3. Push tag: `git push origin v1.x.x`
4. goreleaser builds binaries + updates Homebrew tap

<!-- MANUAL: Notes below this line are preserved on regeneration -->
