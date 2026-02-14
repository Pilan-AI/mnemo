package cmd

// inject.go implements the UserPromptSubmit hook for Claude Code.
// It reads the user's prompt from stdin JSON (Claude Code hook protocol),
// checks the injection_mode from config, searches mnemo, and
// returns additionalContext as JSON to stdout.
//
// Claude Code passes hook input as JSON on stdin:
//   {"prompt": "user message", "session_id": "...", "hook_event_name": "UserPromptSubmit"}
//
// Usage in hooks.json:
//   { "type": "command", "command": "mnemo inject", "timeout": 5 }

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Pilan-AI/mnemo/internal/db"
	"github.com/spf13/cobra"
)

var injectCmd = &cobra.Command{
	Use:    "inject",
	Short:  "Context injection hook for Claude Code (UserPromptSubmit)",
	Hidden: true, // Internal command, not shown in help
	Run: func(cmd *cobra.Command, args []string) {
		prompt := readPromptFromStdin()
		if prompt == "" {
			// No prompt available, pass through
			printHookResult("", "")
			return
		}

		mode := getInjectionMode()
		if mode == "off" {
			printHookResult("", "")
			return
		}

		// In helper mode, only inject for code/debug related prompts
		if mode == "helper" && !isCodeRelated(prompt) {
			printHookResult("", "")
			return
		}

		// Extract search keywords from the prompt
		query := extractKeywords(prompt)
		if query == "" {
			printHookResult("", "")
			return
		}

		// Search mnemo for relevant past sessions (read-only, no DDL)
		if err := db.InitReadOnly(); err != nil {
			printHookResult("", "")
			return
		}
		defer db.CloseDB()

		results, err := db.SearchGrouped(query, 5)
		if err != nil || len(results) == 0 {
			printHookResult("", "")
			return
		}

		// Build compact context string (~500 tokens max)
		context := formatInjectionContext(results)
		printHookResult(context, "")
	},
}

// getInjectionMode reads injection_mode from ~/.mnemo/config.json.
// Returns "off", "helper", or "assistant". Defaults to "off".
func getInjectionMode() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "off"
	}

	configPath := filepath.Join(home, ".mnemo", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "off"
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return "off"
	}

	if mode, ok := config["injection_mode"].(string); ok {
		return mode
	}

	return "off"
}

// isCodeRelated checks if the prompt is about coding/debugging.
// Used in "helper" mode to filter non-technical prompts.
func isCodeRelated(prompt string) bool {
	lower := strings.ToLower(prompt)
	codeKeywords := []string{
		"bug", "error", "fix", "debug", "crash", "fail",
		"implement", "build", "compile", "test", "refactor",
		"function", "class", "method", "api", "endpoint",
		"import", "package", "module", "dependency",
		"git", "commit", "merge", "branch", "pr", "pull request",
		"deploy", "docker", "kubernetes", "ci", "cd",
		"database", "query", "schema", "migration",
		"swift", "swiftui", "react", "typescript", "python", "go",
		"xcode", "simulator", "appkit", "uikit",
		"how do i", "how to", "what's wrong", "why does",
		"doesn't work", "not working", "broken",
	}

	for _, kw := range codeKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// extractKeywords pulls meaningful search terms from a prompt.
// Strips common filler words to improve FTS5 search quality.
func extractKeywords(prompt string) string {
	// Take first 200 chars to keep search focused
	if len(prompt) > 200 {
		prompt = prompt[:200]
	}

	// Remove common filler words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "shall": true, "can": true,
		"i": true, "you": true, "he": true, "she": true, "it": true,
		"we": true, "they": true, "me": true, "him": true, "her": true,
		"us": true, "them": true, "my": true, "your": true, "his": true,
		"its": true, "our": true, "their": true,
		"this": true, "that": true, "these": true, "those": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "with": true, "by": true, "from": true, "as": true,
		"into": true, "about": true, "between": true, "through": true,
		"and": true, "but": true, "or": true, "not": true, "so": true,
		"if": true, "then": true, "than": true, "too": true, "very": true,
		"just": true, "also": true, "only": true, "now": true, "here": true,
		"there": true, "when": true, "where": true, "how": true, "what": true,
		"which": true, "who": true, "whom": true, "why": true,
		"all": true, "each": true, "every": true, "both": true,
		"please": true, "pls": true, "thanks": true, "hey": true, "hi": true,
		"let": true, "make": true, "get": true, "go": true, "know": true,
		"think": true, "want": true, "need": true, "look": true, "tell": true,
	}

	words := strings.Fields(strings.ToLower(prompt))
	var keywords []string
	for _, w := range words {
		// Strip punctuation
		w = strings.Trim(w, ".,!?;:\"'`()[]{}/<>@#$%^&*+=~")
		if len(w) < 2 || stopWords[w] {
			continue
		}
		keywords = append(keywords, w)
	}

	if len(keywords) > 5 {
		keywords = keywords[:5]
	}

	// Use OR so FTS5 matches ANY keyword, not ALL (implicit AND).
	// This dramatically improves recall for natural language prompts.
	return strings.Join(keywords, " OR ")
}

// formatInjectionContext builds a compact context string from search results.
func formatInjectionContext(results []db.SessionMatch) string {
	var sb strings.Builder
	sb.WriteString("mnemo: Relevant past sessions found:\n")

	for i, r := range results {
		if i >= 3 {
			break
		}

		title := cleanTitle(r.FirstQuery)
		if title == "" {
			title = r.Project
		}
		if len(title) > 120 {
			title = title[:117] + "..."
		}

		ago := relativeTimeShort(r.StartTime)
		fmt.Fprintf(&sb, "  %d. [%s] %s (%s, %d msgs, %s)\n",
			i+1, r.Project, title, r.Tool, r.MessageCount, ago)

		// Include snippet if available
		snippet := strings.ReplaceAll(strings.ReplaceAll(r.Snippet, "⟪", ""), "⟫", "")
		snippet = xmlTagRe.ReplaceAllString(snippet, "")
		snippet = strings.TrimSpace(snippet)
		if snippet != "" && len(snippet) > 20 {
			if len(snippet) > 200 {
				snippet = snippet[:197] + "..."
			}
			fmt.Fprintf(&sb, "     > %s\n", snippet)
		}
	}

	sb.WriteString("Use /remember or /recall for full context from these sessions.")
	return sb.String()
}

// printHookResult outputs the Claude Code hook response.
// For UserPromptSubmit, plain text stdout is added as context per the docs.
// Only use JSON when blocking a prompt (decision: "block").
func printHookResult(additionalContext string, _ string) {
	if additionalContext != "" {
		// Plain text stdout is injected as context for UserPromptSubmit
		fmt.Println(additionalContext)
		return
	}
	// No context — exit cleanly (exit 0 with no output = allow prompt)
}

// readPromptFromStdin reads the user's prompt from Claude Code hook JSON on stdin.
// Falls back to CLAUDE_USER_PROMPT env var for backward compatibility.
func readPromptFromStdin() string {
	// Try reading JSON from stdin (Claude Code hook protocol)
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(io.LimitReader(os.Stdin, 64*1024))
		if err == nil && len(data) > 0 {
			var input map[string]any
			if json.Unmarshal(data, &input) == nil {
				if p, ok := input["prompt"].(string); ok && p != "" {
					return p
				}
			}
		}
	}

	// Fallback: env var (for manual testing)
	return os.Getenv("CLAUDE_USER_PROMPT")
}

func init() {
	rootCmd.AddCommand(injectCmd)
}
