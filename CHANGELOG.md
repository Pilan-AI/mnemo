# Changelog

All notable changes to mnemo will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Planned
- Hybrid search with vector similarity
- Markdown export for Git-friendly backups
- Thinking block extraction for reasoning analysis

---

## [1.3.0] - 2026-02-08

### Added
- **Auto-context injection**: `mnemo inject` command for Claude Code `UserPromptSubmit` hook — searches past sessions on every prompt and injects relevant context automatically
- **Injection modes**: `off` / `helper` / `assistant` — configurable via `mnemo configure` with read-merge-write config persistence
- **Onboarding injection setup**: Interactive mode selection during first-run onboarding
- **Transaction support**: All indexers now use atomic transactions — a session either fully indexes or rolls back, preventing partial writes on interruption
- **Typed return structs**: `RecentSession`, `UsageStats`, `ToolUsageSummary`, `ModelUsageSummary` replace raw `map[string]interface{}` returns
- **Scan error logging**: All `rows.Scan` failures across the db layer now emit `log.Printf` diagnostics instead of silently continuing
- **Comprehensive godoc**: File-level comments, exported function/type docs across all packages (~95% coverage)

### Changed
- **`execer` interface**: Database functions accept `execer` (shared by `*sql.DB` and `*sql.Tx`) enabling both direct and transactional calls
- `InsertSession` / `InsertMessage` now have `Tx` variants for use within transactions
- Updated AGENTS.md with v1.3.0 patterns, transaction examples, and complete file inventory

### Fixed
- 50 bugs identified through comprehensive QA audit: error handling, `rows.Err()` checks, type assertion safety, temporal decay edge cases, UTC normalization, transaction atomicity, streaming JSONL parser resilience, HTTP status forwarding in proxy

---

## [1.2.0] - 2026-02-07

### Added
- **Intelligent search**: Session-grouped results ranked by BM25 relevance, temporal decay, match density, and user-message preference
- **Three output tiers**: `--context` for token-efficient AI injection (~250 tokens for 5 results), `--json` for structured programmatic access, default compact cards for humans
- **Inline onboarding**: Replaces Bubble Tea TUI with brew-install-style output that persists in scrollback, per-tool brand colors, and post-scan discoveries
- **New tool support**: Codex, Amp, Kiro, Antigravity, Cline, Kilo Code, Roo Code, Crush
- **`mnemo status`** command showing database stats and background index status
- **`mnemo blocks`** command for 5-hour usage blocks with token burn rates
- **`mnemo projects`** command for tracked directory management
- **Projects TUI** for interactive directory management
- **MCP server** uses session-grouped search with token-efficient formatting

### Changed
- Modularized codebase: 12 per-tool adapter files, domain-specific db files
- Search default limit reduced from 10 to 5 sessions (higher quality results)
- MCP `mnemo_search` and `mnemo_context` now return session-level results instead of raw messages
- Snippet extraction prefers user messages over assistant responses
- `first_query` cleaned of XML tags, system directives, and noise patterns

### Fixed
- Flexible timestamp parsing for mixed Go/SQLite date formats
- FTS5 snippet delimiters no longer consumed by XML tag stripping
- Removed unused dependencies (bubbles, yaml)
- Resolved all linting errors

### Performance
- Composite scoring formula: `(BM25 - density_bonus - user_bonus) * exp(-0.03 * days_old)`
- 85% token reduction for MCP context injection

---

## [1.1.0] - 2026-01-29

### Added
- Beautiful onboarding TUI with visual discovery experience
- SQLite database backend replacing JSON file storage
- FTS5 full-text search with snippet highlighting
- Multiple Claude Code location support (projects, transcripts, backup)
- Force reindex flag: `mnemo index --force`
- `mnemo onboard` command for stunning first-time setup
- `mnemo install` command for plugin installation
- `mnemo add` command for adding new knowledge sources
- internal/db package for SQLite operations
- internal/tui package for terminal UI components with Bubble Tea

### Changed
- Enhanced message parsing for multiple content formats
- Better project name extraction from paths
- Improved error handling and user guidance
- Better warnings for low session counts
- Search results now show snippets with keyword highlighting

### Performance
- Faster indexing with SQLite
- Much faster queries on large datasets (FTS5)
- Better data integrity

## [1.0.0] - 2026-01-24

### Added
- Initial release of mnemo
- Index Claude Code and OpenCode conversations
- Full-text search across indexed sessions
- CLI commands: index, search, recent, context
- Support for 12+ AI coding tools

### Changed
- Initial public release

[1.3.0]: https://github.com/Pilan-AI/mnemo/releases/tag/v1.3.0
[1.2.0]: https://github.com/Pilan-AI/mnemo/releases/tag/v1.2.0
[1.1.0]: https://github.com/Pilan-AI/mnemo/releases/tag/v1.1.0
[1.0.0]: https://github.com/Pilan-AI/mnemo/releases/tag/v1.0.0
