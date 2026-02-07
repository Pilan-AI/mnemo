# mnemo

**Instant local search and indexing across all your AI coding sessions.**

I'm an average Claude Code and OpenCode user. When I ran the numbers, I had 89,037 messages sitting across my AI coding tools. While organizing those chats, I realized they weren't just conversations — they were my 'decision journals'. Why I picked one architecture over another, how I debugged that weird race condition at 2am, what trade-offs I accepted and why. All of it scattered across 12 different tools in 5 different formats.

So I built mnemo. It indexes everything into one local SQLite database and gives you full-text search across all of it. No cloud, no accounts, everything stays on your machine.

## Install

```bash
# macOS / Linux
brew install Pilan-AI/tap/mnemo

# From source
go install github.com/Pilan-AI/mnemo@latest
```

## Quick start

```bash
# First run — interactive onboarding that finds your tools
mnemo index

# Search across everything
mnemo search "authentication flow"

# See what you've been working on
mnemo recent --days=7

# Load context into a new session
mnemo context my-project
```

That's it. `mnemo index` auto-detects your installed tools, parses their native formats, and builds the search index. Subsequent runs are incremental.

## Supported tools

mnemo reads the native storage format of each tool directly. No exports, no copy-paste, no API keys.

| Tool | Format | What it reads |
|------|--------|---------------|
| Claude Code | JSONL | `~/.claude/projects/`, transcripts, XDG paths |
| OpenCode | JSON | `~/.local/share/opencode/` message + session dirs |
| Gemini CLI | JSON | `~/.gemini/sessions/` + new tmp/chats format |
| Cursor | SQLite | globalStorage `state.vscdb` composer data |
| Crush | SQLite | `~/.crush/crush.db` + per-project databases |
| Codex | JSONL | `~/.codex/sessions/` + archived sessions |
| Amp | JSON | `~/.local/share/amp/threads/` with usage ledger |
| Kiro | JSON | globalStorage workspace-sessions |
| Antigravity | JSONL | `~/.gemini/antigravity/code_tracker/` |
| Kilo Code | JSON | VS Code extension `tasks/ui_messages.json` |
| Cline | JSON | VS Code extension `tasks/ui_messages.json` |
| Roo Code | JSON | VS Code extension `tasks/ui_messages.json` |

Windsurf, Aider, and GitHub Copilot support coming soon.

## Plugins

If you use Claude Code or OpenCode, the plugins give you a much deeper integration than raw MCP. Your AI assistant gets access to your past sessions as context — it remembers what you discussed last week.

### Claude Code plugin

```bash
mnemo install claude-code
```

This installs the [mnemo-memory plugin](https://github.com/Pilan-AI/pilan-plugins) which gives you:

- **Auto-context** — past session context loads automatically when you start working
- `/mnemo-memory:remember <query>` — search past sessions from inside Claude Code
- `/mnemo-memory:recall` — load full project memory into your current session
- **Memory agent** — a specialized subagent for deep context retrieval across projects

### OpenCode plugin

```bash
mnemo install opencode
```

Adds mnemo as an MCP tool inside OpenCode. Search and context commands available directly in your coding session.

## MCP server

For Claude Desktop, Cursor, and other MCP-compatible clients:

```bash
mnemo serve
```

Exposes four tools: `mnemo_search`, `mnemo_context`, `mnemo_recent`, `mnemo_tools`.

```bash
# Or install the MCP config directly
mnemo install mcp
```

## Commands

| Command | What it does |
|---------|-------------|
| `mnemo index` | Index all detected AI tool sessions |
| `mnemo search <query>` | Full-text search with BM25 ranking |
| `mnemo recent` | Show recent sessions (default: 7 days) |
| `mnemo context <project>` | Generate project context summary |
| `mnemo tools` | List detected AI tools and session counts |
| `mnemo blocks` | Show 5-hour usage blocks with token burn rates |
| `mnemo projects` | Manage tracked project directories |
| `mnemo serve` | Start MCP server |
| `mnemo install` | Install plugins and MCP config |
| `mnemo add <path>` | Index a custom path |

## How it works

```
┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ Claude Code │ │  OpenCode   │ │ Gemini CLI  │ │   Cursor    │  + 8 more
│   (JSONL)   │ │   (JSON)    │ │   (JSON)    │ │  (SQLite)   │
└──────┬──────┘ └──────┬──────┘ └──────┬──────┘ └──────┬──────┘
       │               │               │               │
       └───────────────┴───────┬───────┴───────────────┘
                               │
                               ▼
                  ┌────────────────────────┐
                  │      mnemo index       │
                  │                        │
                  │  auto-detect tools     │
                  │  parse native formats  │
                  │  normalize + dedupe    │
                  └───────────┬────────────┘
                              │
                              ▼
                  ┌────────────────────────┐
                  │   ~/.mnemo/mnemo.db    │
                  │                        │
                  │  SQLite + FTS5 index   │
                  │  sessions · messages   │
                  │  projects · tokens     │
                  └──┬────┬────┬─────┬─────┘
                     │    │    │     │
                     ▼    ▼    ▼     ▼
                  search  ·  context · recent · serve
                  (BM25)    (project)  (days)   (MCP)
                     │    │    │     │
                     ▼    ▼    ▼     ▼
                  ┌────────────────────────┐
                  │  Claude Code plugin    │
                  │  OpenCode plugin       │
                  │  MCP clients (Cursor,  │
                  │  Claude Desktop, etc.) │
                  └────────────────────────┘
```

1. `mnemo index` scans each tool's native storage (JSONL, SQLite, JSON)
2. Messages are normalized into `~/.mnemo/mnemo.db` — a single SQLite file with FTS5 full-text search
3. `mnemo search` runs BM25-ranked queries across all indexed content
4. Results return matched snippets with project name, tool, model, and session context

The database is a single file. Back it up, move it between machines, query it with any SQLite client.

## Platform support

- **macOS** (Apple Silicon + Intel)
- **Linux** (arm64 + amd64)
- **Windows** (amd64)

Built with pure-Go SQLite ([modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)) — no CGO, no system dependencies. Single binary, runs anywhere.

## Why not grep?

grep searches text. mnemo searches **sessions**.

- grep can't parse 12 different formats (JSONL, SQLite, JSON) into meaningful conversations
- grep doesn't rank results by relevance (mnemo uses BM25)
- grep doesn't know that a Claude Code JSONL file and a Cursor SQLite database contain the same kind of data
- grep gives you matching lines; mnemo gives you snippets with project, tool, and session context

If you use one tool and remember exact strings, grep works. If you use multiple tools and want to find "that auth discussion from last week," you need mnemo.

## What's next

mnemo is the first tool from [Pilan](https://pilan.ai). Later this month, we're launching a native macOS app that sits on top of mnemo — knowledge graph, pattern recognition, session intelligence. If mnemo is the memory, Pilan is the brain.

## Uninstall

```bash
brew uninstall mnemo
# or: rm $(which mnemo)

# Remove indexed data (optional)
rm -rf ~/.mnemo
```

## License

MIT. See [LICENSE](./LICENSE).

---

[GitHub](https://github.com/Pilan-AI/mnemo) · [X](https://x.com/Pilan_AI) · [Pilan](https://pilan.ai)

Built by [@0xraghu](https://x.com/Pilan_AI)

---

<div align="center">

*எண்ணென்ப ஏனை எழுத்தென்ப — இவ்விரண்டும்*
*கண்ணென்ப வாழும் உயிர்க்கு.*

Numbers and letters — these two
are the eyes of all who live.

— Thiruvalluvar, Tirukkural 392

</div>
