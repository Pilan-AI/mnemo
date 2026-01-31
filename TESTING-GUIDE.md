# mnemo Testing Guide - User Manual

**Target**: Non-programmer user testing in 5-minute chunks  
**Format**: Checkboxes + observations  
**Goal**: Validate core functionality before business decisions

---

## 🎯 What You're Testing

mnemo is a CLI tool that indexes your AI coding conversations (Claude Code, OpenCode, etc.) and lets you search them. Think of it like a search engine for all your past AI chats.

**Core Features to Test:**
1. ✅ `tools` - Detect which AI tools you have installed
2. ✅ `index` - Index your conversations into a searchable database
3. ✅ `search` - Search across all indexed conversations
4. ✅ `recent` - Show recent sessions
5. ✅ `context` - Generate context summary for a project

---

## 📋 Pre-Test Setup (2 minutes)

### Checkpoint A: Verify Installation

```bash
cd /Users/raghu/Projects/PILAN-INTELLIGENCE-PRISM/code/products/mnemo
./mnemo --help
```

**Expected Output:**
```
mnemo - Memory for AI-assisted development

The faintest ink is more powerful than the strongest memory.
Index your past to build your future.

Your AI coding sessions — indexed, searchable, never forgotten.

Available Commands:
  add         Add manual note to mnemo index
  configure   Set mnemo injection mode (off/helper/assistant)
  context     Generate context summary for a project
  help        Help about any command
  index       Index AI tool conversations
  install     Install mnemo hooks for AI tools
  proxy       Run mnemo proxy server (DISABLED - under development)
  recent      Show recent sessions
  search      Search indexed conversations
  serve-mcp   Start MCP server for auto-injection (DISABLED - under development)
  tools       Detect installed AI tools
  version     Show mnemo version
```

**✅ Checklist:**
- [x] `./mnemo --help` works without errors
- [x] You see the list of commands above
- [x] You see the memory/ink philosophical quote (Tamil references removed)

**📝 Notes:**
```
[Add any observations or issues here]




```

---

## 🧪 Test 1: Tool Detection (3 minutes)

### What This Tests
mnemo should detect which AI coding tools you have installed by looking for their log directories.

### Commands to Run

```bash
cd /Users/raghu/Projects/PILAN-INTELLIGENCE-PRISM/code/products/mnemo
./mnemo tools
```

### Expected Output Example
```
✓ Claude Code     ~/.claude/projects
✓ Opencode        ~/.opencode/sessions

Detected: 2/11 tools

Supported tools:
  Claude Code    ✅ Full support
  Opencode       ✅ Full support
  Cursor         🔄 Coming soon
  Gemini CLI     🔄 Coming soon
  ...
```

### ✅ Checklist (nope huge problem)
- [ ] Command runs without errors
- [x] Shows "Claude Code" detected (you use this)
- [ ] Shows "Opencode" detected (if you use it)
- [ ] Shows total count (e.g., "Detected: 2/11 tools")
- [ ] Lists all 11 supported tools

### 📝 Notes - What You Observe
```
Number of tools detected: ____

Tools found:
- 


Any errors or unexpected behavior:




```

---

## 🧪 Test 2: Indexing (5 minutes)

### What This Tests
mnemo reads your AI conversation logs and builds a searchable index. This is the core feature.

### Commands to Run

```bash
cd /Users/raghu/Projects/PILAN-INTELLIGENCE-PRISM/code/products/mnemo
./mnemo index
```

### Expected Output Example
```
Indexing AI tool conversations...

✓ Claude Code: 662 sessions, 89,054 messages
✓ Opencode: 125 sessions, 12,430 messages

Total: 787 sessions, 101,484 messages indexed

Database: ~/.mnemo/mnemo.db (24.3 MB)
Index time: 3.4s
```

### ✅ Checklist
- [ ] Command runs without errors
- [ ] Shows session count for Claude Code
- [ ] Shows message count for each tool
- [ ] Creates database at `~/.mnemo/mnemo.db`
- [ ] Completes in reasonable time (<10 seconds)

### 📝 Notes - What You Observe
```
Sessions indexed: ____
Messages indexed: ____
Database size: ____ MB
Index time: ____ seconds

Any errors or warnings:




Was it slow? Did it feel fast?


```

---

## 🧪 Test 3: Search Functionality (5 minutes)

### What This Tests
Search across ALL your indexed AI conversations using keywords.

### Commands to Run

Try these searches (or use your own):

```bash
# Search for "authentication" related conversations
./mnemo search "authentication"

# Search for "error" related conversations
./mnemo search "error"

# Search for "mnemo" related work
./mnemo search "mnemo"
```

### Expected Output Example
```
Found 23 results for "authentication":

[1] my-saas-project
    Role: assistant
    Match: Implementing JWT >>>authentication<<< with refresh tokens...

[2] e-commerce-app
    Role: user  
    Match: How do I handle >>>authentication<<< errors when tokens expire?

[3] api-gateway
    Role: assistant
    Match: For >>>authentication<<< middleware, use the following pattern...
```

### ✅ Checklist
- [x] Search returns results (not "0 results")
- [x] Results show project names
- [x] Results show highlighted matches (>>> <<<)
- [x] Results show role (user/assistant)
- [x] Search is fast (<1 second)

### 📝 Notes - What You Observe

**Search 1: "authentication"**
```
Number of results: ____
Search time: ____ ms
Quality of results (relevant?): ____




```

**Search 2: Your own keyword**
```
Keyword you searched: ____
Number of results: ____
Were results relevant to your keyword?




```

**Any issues:**
```
Did search fail for any keyword?
Were results confusing?
Was anything highlighted wrong?




```

---

## 🧪 Test 4: Recent Sessions (3 minutes)

### What This Tests
Show your most recent AI coding sessions.

### Commands to Run

```bash
# Show last 10 sessions
./mnemo recent

# Show last 20 sessions
./mnemo recent --limit 20
```

### Expected Output Example
```
Recent sessions:

[ses_abc123] my-saas-project
  └─ "Help me implement JWT authentication for the payment API"
      125 messages | claude | 2026-01-29 14:30

[ses_def456] portfolio-website
  └─ "Build a responsive navbar with React and Tailwind CSS"  
      43 messages | claude | 2026-01-28 09:15

[ses_ghi789] mnemo
  └─ "Fix the 200K token bug in MCP search results"
      89 messages | claude | 2026-01-27 18:45
```

### ✅ Checklist
- [ ] Shows recent sessions in reverse chronological order (newest first)
- [ ] Shows session ID
- [ ] Shows project name
- [ ] Shows first query/question from that session
- [ ] Shows message count
- [ ] Shows timestamp
- [ ] `--limit` flag works to show more/fewer results

### 📝 Notes - What You Observe
```
Number of sessions shown: ____
Oldest session date: ____
Newest session date: ____

Do you recognize these sessions?




Are the first queries accurate to what you remember?




```

---

## 🧪 Test 5: Context Generation (4 minutes)

### What This Tests
Generate a context summary for a specific project to use when starting a new AI session.

### Commands to Run

```bash
# Generate context for a project you've worked on
# Replace "my-project" with an actual project name from your sessions
./mnemo context my-saas-project

# Try another project
./mnemo context mnemo
```

### Expected Output Example
```
# Context for my-saas-project

## Overview
Project with 12 sessions, 450 messages
Last activity: 2026-01-29 14:30

## Recent Work
- Session [ses_abc123]: JWT authentication implementation (2026-01-29)
  - Initial query: Help me implement JWT auth for payment API
  
- Session [ses_def456]: Database schema design (2026-01-28)  
  - Initial query: Design PostgreSQL schema for user accounts

## Key Topics
- Authentication (8 sessions)
- Database design (4 sessions)
- API development (6 sessions)
```

### ✅ Checklist
- [ ] Shows project overview (session count, message count)
- [ ] Shows recent sessions for that project
- [ ] Shows first query from each session
- [ ] Output is formatted as Markdown
- [ ] Could copy-paste this into a new AI conversation

### 📝 Notes - What You Observe
```
Project you tested: ____
Number of sessions found: ____

Is the context summary useful?




Would you actually use this when starting a new AI chat?




What's missing that you wish was there?




```

---

## 🧪 Test 6: Edge Cases & Error Handling (3 minutes)

### What This Tests
How mnemo handles unusual inputs or errors.

### Commands to Run

```bash
# Search with no results
./mnemo search "xyznonexistentkeyword123"

# Recent with huge limit
./mnemo recent --limit 10000

# Context for non-existent project
./mnemo context "fake-project-that-does-not-exist"
```

### ✅ Checklist
- [ ] No crashes with weird inputs
- [ ] "No results" is handled gracefully
- [ ] Non-existent projects show clear message
- [ ] Large limits don't break anything

### 📝 Notes - What You Observe
```
Did anything crash?




Were error messages helpful?




Any confusing behavior?




```

---

## 📊 Final Summary

### Overall Experience (Rate 1-5)

- **Ease of Use**: ____/5 (1 = confusing, 5 = intuitive)
- **Speed**: ____/5 (1 = slow, 5 = instant)
- **Usefulness**: ____/5 (1 = pointless, 5 = game-changer)
- **Reliability**: ____/5 (1 = crashes, 5 = rock solid)

### What Worked Well
```
List 3 things that worked smoothly:
1. 
2. 
3. 
```

### What Needs Improvement
```
List 3 things that need fixing:
1. 
2. 
3. 
```

### Would You Actually Use This?
```
Honest answer: Yes / No / Maybe

Why or why not?





```

### Business Model Feedback
```
Knowing what you know now, what would you pay for this?

- Local features (search, index): FREE / $5/mo / $10/mo / Other: ____
- Cloud features (if we add them): $____/mo
- Enterprise (for teams): $____/year

What features would make you pay?




```

---

## 🚨 Critical Issues (If Any)

```
Did mnemo completely fail to work?
Were there major bugs that prevented testing?
Is there something fundamentally broken?

If yes, describe in detail:





```

---

**End of mnemo Testing Guide**

Save this file with your notes and we'll review together!
