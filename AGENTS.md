<!-- Parent: ../AGENTS.md -->
<!-- Generated: 2026-01-30 | Updated: 2026-02-07 -->

# mnemo

## Purpose

**Memory for AI-assisted development** — Indexes AI coding sessions from 12+ tools (Claude Code, OpenCode, Gemini CLI, Cursor, etc.) into a unified, searchable SQLite database with FTS5 full-text search.

**Status**: Active development, Production-ready

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
│   ├── serve.go                 # MCP server
│   ├── blocks.go                # 5-hour usage block display
│   ├── projects.go              # Project management
│   ├── tools.go                 # Tool detection + path helpers
│   ├── add.go                   # Custom path indexing
│   ├── install.go               # Plugin installer
│   ├── context.go               # Context generation
│   ├── recent.go                # Recent sessions display
│   └── onboarding.go            # First-run experience
├── internal/
│   ├── db/                      # SQLite database layer
│   │   ├── sqlite.go            # Schema, migrations, init
│   │   ├── messages.go          # Message struct + InsertMessage
│   │   ├── sessions.go          # Session CRUD + GetRecentSessions
│   │   ├── search.go            # FTS5 search + BM25 ranking
│   │   ├── projects.go          # Project management + classification
│   │   ├── token_usage.go       # Token/cost tracking + API credentials
│   │   └── blocks.go            # 5-hour session blocks + usage stats
│   └── tui/                     # Bubble Tea TUI components
├── proxy/                       # HTTP proxy for Claude API injection
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
  ↓  parse JSONL / JSON / SQLite
internal/db/
  ├── sqlite.go        → Schema + init
  ├── messages.go      → Insert normalized messages
  ├── sessions.go      → Session tracking
  ├── search.go        → FTS5 full-text search
  ├── projects.go      → Project discovery
  ├── token_usage.go   → Token/cost accounting
  └── blocks.go        → Usage block analysis
  ↓
SQLite Database (~/.mnemo/mnemo.db)
```

### Common Patterns

- **Cobra CLI**: All commands use cobra framework
- **Bubble Tea TUI**: Interactive experiences use charmbracelet/bubbletea
- **SQLite + FTS5**: Single-file database with full-text search and BM25 ranking
- **One adapter per file**: Each tool gets its own `cmd/index_<tool>.go`
- **MCP Integration**: Model Context Protocol server for Claude Desktop/Cursor
- **Pure Go SQLite**: modernc.org/sqlite — no CGO, no system dependencies

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
