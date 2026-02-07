# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.x     | Yes       |
| < 1.0   | No        |

## Reporting a Vulnerability

If you discover a security vulnerability in mnemo, please report it responsibly.

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, email: **security@pilan.ai**

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (if any)

## Response Timeline

- **Acknowledgment**: Within 48 hours
- **Initial assessment**: Within 1 week
- **Fix release**: Within 2 weeks for critical issues

## Scope

mnemo stores AI coding session data locally in `~/.mnemo/mnemo.db`. Security considerations include:

- **Local data access**: mnemo reads session files from AI tools on your machine
- **SQLite database**: All data stored locally, no network transmission
- **MCP server**: Runs locally for Claude Desktop/Code integration
- **No telemetry**: mnemo does not send any data externally

## Security Design

- Database uses SQLite WAL mode with local file permissions
- FTS5 queries are sanitized before execution
- The proxy server (if used) binds to localhost only
- No API keys or secrets are stored in the database
