# mnemo Quick Start

Get started with mnemo in 5 minutes.

---

## 1. Installation

```bash
# macOS / Linux (Homebrew)
brew install mnemo

# Or build from source
go install github.com/Pilan-AI/mnemo@latest
```

**Optional:** See the welcome experience
```bash
mnemo onboarding
```

---

## 2. First Run - Index Your Sessions

```bash
mnemo index
```

**What happens:**
- 🔍 mnemo scans for AI coding tools (Claude Code, Cursor, Gemini, etc.)
- 📚 Indexes all conversation history
- 💾 Stores searchable index in `~/.mnemo/mnemo.db`

**Example output:**
```
Detecting AI tools...
  ✓ Claude Code     ~/.claude/projects
  ✓ Opencode        ~/.local/share/opencode

Indexing conversations...
  ✓ Claude Code: 662 sessions, 89054 messages
  ✓ Opencode: 124 sessions, 15234 messages

Total: 786 sessions, 104288 messages indexed
Index saved to: ~/.mnemo/mnemo.db
```

---

## 3. Search Your History

```bash
# Search for anything
mnemo search "authentication"

# Find recent work
mnemo recent

# Get context for a project
mnemo context my-saas-app
```

**Search example:**
```bash
$ mnemo search "OAuth implementation"

Found 12 results:

[1] my-saas-app (session_abc123)
    Project: my-saas-app
    Tool: claude
    Messages: 87
    
    Query: Help me implement OAuth2 authentication with GitHub...
    
    Assistant: I'll help you set up OAuth2 with GitHub. First, you need to...
    
[2] api-backend (session_def456)
    ...
```

---

## 4. See Available Tools

```bash
mnemo tools
```

Shows which AI coding assistants are installed and detected.

---

## 5. Add Documentation

```bash
# Index your project docs
mnemo add my-docs ~/projects/my-app/docs --name="My App Docs"

# Now searches include your documentation
mnemo search "API endpoints"
```

---

## Common Workflows

### Find Past Solutions
```bash
# "How did I solve X last month?"
mnemo search "rate limiting implementation"
```

### Resume a Conversation
```bash
# See recent sessions
mnemo recent --days=7

# Get full context for a project
mnemo context billing-service > CONTEXT.md
```

### Search Across All Tools
```bash
# Finds results from Claude, Cursor, Gemini, etc.
mnemo search "database migration"
```

---

## Configuration

mnemo stores everything in `~/.mnemo/`:
```
~/.mnemo/
├── mnemo.db          # SQLite database (all indexed data)
└── config.json       # Settings (created on first run)
```

### Re-index Everything
```bash
mnemo index --force
```

### Clear Index
```bash
rm -rf ~/.mnemo
mnemo index
```

---

## Supported AI Tools

| Tool | Status |
|------|--------|
| Claude Code | ✅ Fully supported |
| Opencode | ✅ Fully supported |
| Cursor | 🔄 Coming soon |
| Gemini CLI | 🔄 Coming soon |
| Windsurf | 🔄 Coming soon |
| Aider | 🔄 Coming soon |

---

## Troubleshooting

### No sessions found?

**Check if tools are installed:**
```bash
mnemo tools
```

**Check tool directories exist:**
```bash
ls ~/.claude/projects        # Claude Code
ls ~/.local/share/opencode   # Opencode
```

### Index is outdated?

Re-run indexing:
```bash
mnemo index
```

### Want to start fresh?

```bash
rm -rf ~/.mnemo
mnemo index
```

---

## Next Steps

- **Search:** Try `mnemo search "your query"`
- **Browse:** Use `mnemo recent` to see what's indexed
- **Context:** Generate project summaries with `mnemo context`
- **Docs:** Add your project docs with `mnemo add`

---

**Questions?** See the [main README](./README.md) or open an issue on GitHub.
