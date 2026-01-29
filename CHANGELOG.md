# Changelog

All notable changes to mnemo will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[1.1.0]: https://github.com/Pilan-AI/mnemo/releases/tag/v1.1.0
[1.0.0]: https://github.com/Pilan-AI/mnemo/releases/tag/v1.0.0
