# Contributing to mnemo

Thanks for your interest in contributing to mnemo.

## Development Setup

```bash
# Clone the repo
git clone https://github.com/Pilan-AI/mnemo.git
cd mnemo

# Install dependencies
go mod download

# Build
go build -o mnemo .

# Run tests
go test ./...

# Run linter
golangci-lint run
```

**Requirements**: Go 1.23+

## Project Structure

```
mnemo/
├── main.go                    # Entry point
├── cmd/                       # CLI commands (cobra)
│   ├── root.go                # Root command + configure
│   ├── index.go               # Indexing orchestrator + onboarding
│   ├── index_helpers.go       # Shared helpers (truncate, inferProvider, etc.)
│   ├── index_claude.go        # Claude Code adapter (JSONL)
│   ├── index_opencode.go      # OpenCode adapter (JSON)
│   ├── index_gemini.go        # Gemini CLI adapter (JSON)
│   ├── index_cursor.go        # Cursor adapter (SQLite)
│   ├── index_codex.go         # Codex CLI adapter (JSONL)
│   ├── index_amp.go           # Amp adapter (JSON + usage ledger)
│   ├── index_crush.go         # Crush adapter (SQLite)
│   ├── index_cline.go         # Cline/Roo/Kilo Code adapter (JSON)
│   ├── index_kiro.go          # Kiro adapter (JSON)
│   ├── index_antigravity.go   # Antigravity adapter (JSONL)
│   ├── index_vscode.go        # VS Code AI chat adapter (SQLite)
│   ├── search.go              # Full-text search command
│   ├── serve.go               # MCP server for Claude Desktop/Code
│   ├── blocks.go              # 5-hour usage block display
│   ├── projects.go            # Project management
│   ├── tools.go               # Tool detection
│   ├── add.go                 # Knowledge source indexing
│   ├── install.go             # Plugin installer
│   ├── context.go             # Context generation
│   ├── recent.go              # Recent sessions display
│   └── onboarding.go          # First-run experience
├── internal/
│   ├── db/                    # SQLite database layer
│   │   ├── sqlite.go          # Schema, migrations, init
│   │   ├── messages.go        # Message CRUD
│   │   ├── sessions.go        # Session CRUD
│   │   ├── search.go          # FTS5 full-text search
│   │   ├── projects.go        # Project management
│   │   ├── token_usage.go     # Token/cost tracking
│   │   └── blocks.go          # 5-hour session blocks
│   └── tui/                   # Bubble Tea TUI components
└── proxy/                     # HTTP proxy for Claude API injection
```

## Making Changes

1. Fork the repo and create a branch from `main`
2. Make your changes
3. Add tests for new functionality
4. Run `go test ./...` and `go vet ./...`
5. Submit a pull request

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Keep functions focused and under ~200 lines where possible
- Use `fmt.Errorf("...: %w", err)` for error wrapping
- Add doc comments for exported types and functions

## Adding a New Tool Adapter

mnemo indexes AI coding sessions from 12+ tools. To add support for a new tool:

1. Add a detection function in `cmd/tools.go` that locates session files
2. Create a new `cmd/index_<tool>.go` file with the indexing function (see existing adapters for patterns)
3. Wire it into the orchestrator in `cmd/index.go`
4. Add the tool name to the `knownTools` list
5. Add a test case in `cmd/index_adapters_test.go`
6. Update `README.md` with the new tool

## Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./internal/db/...

# Verbose
go test -v -race ./...
```

## Commit Messages

Use [conventional commits](https://www.conventionalcommits.org/):

- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation only
- `refactor:` code change that neither fixes a bug nor adds a feature
- `test:` adding or updating tests
- `chore:` maintenance tasks

## Reporting Bugs

Open an issue with:
- mnemo version (`mnemo version`)
- OS and architecture
- Steps to reproduce
- Expected vs actual behavior

## Feature Requests

Open an issue describing:
- The problem you're trying to solve
- Your proposed solution
- Any alternatives you considered

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
