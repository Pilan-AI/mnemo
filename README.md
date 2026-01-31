<div align="center">

# mnemo

### Memory for AI-assisted development

> **Intelligence Crystallized!**
>
> எண்ணென்ப ஏனை எழுத்தென்ப இவ்விரண்டும்
> கண்ணென்ப வாழும் உயிர்க்கு.
>
> *(Numbers and Letters are the two eyes for the living)*
>
> — Thiruvalluvar, Thirukkural

<br>

**Index your past to build your future.**

</div>

---

## Install

```bash
# macOS / Linux
brew install mnemo

# Or build from source
go install github.com/Pilan-AI/mnemo@latest
```

## What is this?

Your AI coding sessions — **indexed, searchable, never forgotten**.

```bash
# See what AI tools you have
$ mnemo tools
  ✓ Claude Code     ~/.claude/projects
  ✓ Cursor          ~/.cursor
  ✓ Gemini CLI      ~/.gemini
  ...
  Detected: 6/12+ tools

# Index all your conversations
$ mnemo index
  ✓ Claude Code: 662 sessions, 89054 messages
  Total: 662 sessions, 89054 messages indexed

# Search across everything
$ mnemo search "authentication flow"
  Found 23 results:
  [1] my-saas-project (420 messages)
      Query: Help me implement OAuth2...

# See recent work
$ mnemo recent --days=7

# Generate context for a new session
$ mnemo context my-project > CONTEXT.md
```

## Supported Tools

Works with **any** AI coding assistant:

| Tool | Status | Format |
|------|--------|--------|
| Claude Code | ✅ Full support | JSONL |
| Opencode | ✅ Full support | JSONL |
| Cursor | 🔄 Coming soon | SQLite |
| Gemini CLI | 🔄 Coming soon | JSON |
| Windsurf | 🔄 Coming soon | JSON |
| Aider | 🔄 Coming soon | Markdown |
| GitHub Copilot | 🔄 Coming soon | JSON |
| Roo Code | 🔄 Coming soon | JSON |
| Kilo Code | 🔄 Coming soon | JSON |
| Amp | 🔄 Coming soon | JSON |
| Cline | 🔄 Coming soon | JSON |

---

## Why mnemo?

Every developer using AI assistants has this problem:

> "I built something similar last week... what did I do?"
>
> "What was that command Claude gave me for Kubernetes?"
>
> "I had this exact conversation before, where is it?"

Your AI conversations are **gold** — full of solved problems, working code, architectural decisions. But they're scattered across tools, buried in log files, impossible to search.

**mnemo fixes this.**

One index. All tools. Instant search.

---

<div align="center">

## The Story Behind mnemo

**Built in 21 days. Total cost: $420.**

</div>

I'm **Raghu** — find me at `@Pilan_AI` on GitHub and X.

Every developer using AI assistants faces the same problem: valuable conversations scattered across tools, impossible to search, lost to time. After months of watching myself and others repeatedly solve the same problems because we couldn't find our past solutions, I built mnemo.

The philosophy is simple: **your AI conversations are knowledge artifacts that deserve to be searchable, reusable, and permanent.**

mnemo exists because:

- **Memory matters** — Every solved problem, every working solution, every architectural decision in your AI conversations is valuable
- **Search is power** — Being able to instantly find "how did I solve X last month?" transforms how you work
- **Tools should unite, not divide** — Your knowledge shouldn't be fragmented across different AI assistants

I built mnemo in 21 days because I needed it yesterday. The $420 cost proves that focused execution beats endless planning.

If you're tired of re-solving problems you've already solved, mnemo is for you.

---

## Uninstall

```bash
# If installed via Homebrew
brew uninstall mnemo

# If installed via Go
rm $(which mnemo)

# Remove all indexed data (optional)
rm -rf ~/.mnemo
```

---

## License

mnemo is dual-licensed:

- **AGPL v3** — Free for open source and personal use
- **Commercial License** — For proprietary/enterprise use

See [LICENSE](./LICENSE) for details.

---

<div align="center">

**[GitHub](https://github.com/Pilan-AI/mnemo)** · **[X](https://x.com/Pilan_AI)**

*Memory indexed. Knowledge unlocked.*

</div>
