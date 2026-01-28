<div align="center">

# mnemo

### Memory for AI-assisted development

> **Intelligence Crystallized!**

> "Plan panni pannama irundha ippudi dhan! Plan panni pannaum. Okay?"
>
> *"This is what happens when you do things without planning...
> You must plan properly!"*
>
> — Vaigai Puyal Vadivelu (2007)

<br>

**Don't be the Lochak-Mochak engineer. Be the one who ships.**

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

I grew up on mass cinema where humour and heroism taught me more about life than any textbook. Vadivelu's "Lochak, Mochak, Bachak" in *Pokkiri* showed me what chaos looks like when nobody really has a plan — everyone shouting moves, nobody landing a punch. Vijay's character, by contrast, cuts through the noise with quiet clarity, doing more than he says.

That contrast is burned into how I think about engineering: **don't be the Lochak-Mochak architect, be the one who actually ships.**

Years later, while building developer tools, I had *Jana Nayagan*'s "Raavana Mavandaa" on loop. The line *"Edhiriya anuppuna, sirikkiran, pazhagittan pola"* became a mirror: if you've been attacked by enough bugs, criticism, and setbacks, you stop panicking — you start smiling. You've *pazhagittan pola* with chaos.

Underneath all this is the line from *KGF* that became my personal operating system:

> **If you only have the courage that "1000 people are behind you", you might win one battle.**
> **When those same 1000 people get the courage that *you* are in front of them, you can win the world.**

I don't want mnemo to be just another devtool with 1000 users behind it. I want 1000 builders to feel braver because mnemo stands in front of them — simplifying complexity, catching Lochak-Mochak patterns before they spiral, and turning production fear into a calm, Raavana-style smile.

**mnemo** is my tribute:

- To **Vadivelu**, for teaching me that overconfidence without clarity is comedy.
- To **Vijay**, for showing how calm focus can turn noise into direction.
- To **KGF**, for reminding me that true leadership is when others borrow courage from you.

I'm Raghu. I build tools so developers don't just write code — they **lead** it.

### The Inspiration (Original Sources)

- 🎬 [Pokkiri - "Plan Panni Pannanum"](https://www.youtube.com/watch?v=XGbbKs9pUsM) — The meme that started it all
- 🎵 [Jana Nayagan - "Raavana Mavandaa"](https://www.youtube.com/watch?v=9OF_cF48mjA) — Anirudh's anthem of resilience
- 🎥 [KGF - Leadership Quote](https://www.youtube.com/watch?v=6FTnjjxmVTE) — "1000 people behind you vs in front of you"

---

## License

mnemo is dual-licensed:

- **AGPL v3** — Free for open source and personal use
- **Commercial License** — For proprietary/enterprise use

See [LICENSE](./LICENSE) for details.

---

<div align="center">

**[GitHub](https://github.com/Pilan-AI/mnemo)** · **[X](https://x.com/Pilan_AI)**

*"Plan panni pannanum" — but also index properly.*

</div>
