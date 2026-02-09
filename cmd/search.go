// search.go implements the CLI search command with three output formats:
// human (terminal-friendly), context (token-efficient for AI), and json (structured).
package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// xmlTagRe strips XML/HTML-like tags from first_query for clean display.
var xmlTagRe = regexp.MustCompile(`<[^>]+>`)

var (
	searchLimit   int
	searchDays    int
	searchContext bool
	searchJSON    bool
	searchRaw     bool
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across all indexed conversations",
	Long: `Full-text search across all your AI coding sessions using SQLite FTS5.

Output modes:
  (default)    Session-grouped results with compact display
  --context    Token-efficient format for AI context injection (~500 tokens)
  --json       Full JSON output for programmatic use
  --raw        Legacy message-level output`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.Join(args, " ")
		startTime := time.Now()

		if err := db.InitDB(); err != nil {
			fmt.Printf("Error opening database: %v\n", err)
			fmt.Println("Run 'mnemo index' first to build the search index.")
			return
		}
		defer db.CloseDB()

		// Legacy raw mode: original message-level output
		if searchRaw {
			runRawSearch(query, startTime)
			return
		}

		results, err := db.SearchGrouped(query, searchLimit)
		if err != nil {
			fmt.Printf("Search error: %v\n", err)
			return
		}

		if len(results) == 0 {
			if searchContext {
				fmt.Print("No past sessions found.\n")
			} else if searchJSON {
				fmt.Print("[]\n")
			} else {
				fmt.Println("No results found.")
				fmt.Println()
				fmt.Println("Tips:")
				fmt.Println("  - Try different keywords")
				fmt.Println("  - Run 'mnemo index --force' to rebuild the index")
			}
			return
		}

		switch {
		case searchContext:
			printContextFormat(query, results)
		case searchJSON:
			printJSONFormat(results)
		default:
			printHumanFormat(query, results, startTime)
		}
	},
}

// printHumanFormat renders Tier 1: compact session-grouped cards for terminal.
func printHumanFormat(query string, results []db.SessionMatch, startTime time.Time) {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("#5fd7ff"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#87d787"))

	fmt.Println()
	fmt.Printf("  %s %s\n", cyan.Render("search"), dim.Render(fmt.Sprintf("\"%s\"", query)))
	fmt.Println()

	for _, r := range results {
		// Session title from first_query (cleaned)
		title := cleanTitle(r.FirstQuery)
		if title == "" {
			title = r.Project
		}
		if len(title) > 80 {
			title = title[:77] + "..."
		}

		// Relative time
		ago := relativeTime(r.StartTime)

		// Header: project + match count + time + tool
		fmt.Printf("  %s  %s  %s  %s\n",
			lipgloss.NewStyle().Bold(true).Render(r.Project),
			dim.Render(fmt.Sprintf("%d matches", r.MatchCount)),
			dim.Render(ago),
			dim.Render(r.Tool),
		)

		// Title line (first_query)
		if r.FirstQuery != "" && r.FirstQuery != r.Project {
			fmt.Printf("  %s\n", green.Render(truncate(title, 80)))
		}

		// Snippet with highlights
		if r.Snippet != "" {
			snippet := r.Snippet
			// Replace FTS5 delimiters with ANSI first, then clean XML
			snippet = strings.ReplaceAll(snippet, "⟪", "\033[1;33m")
			snippet = strings.ReplaceAll(snippet, "⟫", "\033[0m")
			snippet = xmlTagRe.ReplaceAllString(snippet, "")
			snippet = strings.TrimSpace(snippet)
			if snippet != "" {
				fmt.Printf("  %s\n", dim.Render("...")+snippet)
			}
		}

		fmt.Println()
	}

	elapsed := time.Since(startTime).Round(time.Millisecond)
	fmt.Printf("  %s\n\n", dim.Render(fmt.Sprintf("%d sessions  %v", len(results), elapsed)))
}

// printContextFormat renders Tier 2: ultra-token-efficient for AI context injection.
// Target: ~50 tokens per result, ~500 tokens total for 5 results.
func printContextFormat(query string, results []db.SessionMatch) {
	fmt.Printf("PAST_SESSIONS(query=\"%s\"):\n", query)
	for i, r := range results {
		ago := relativeTimeShort(r.StartTime)
		title := cleanTitle(r.FirstQuery)
		if title == "" {
			title = r.Project
		}
		if len(title) > 100 {
			title = title[:97] + "..."
		}

		fmt.Printf("%d. [%s/%s/%dhits] \"%s\" — %s\n",
			i+1, r.Project, ago, r.MatchCount, title, r.Tool)
	}
}

// printJSONFormat renders Tier 3: full structured JSON for AETHER/Pilan.
func printJSONFormat(results []db.SessionMatch) {
	type jsonResult struct {
		SessionID    string    `json:"session_id"`
		Project      string    `json:"project"`
		FirstQuery   string    `json:"first_query"`
		MessageCount int       `json:"message_count"`
		Tool         string    `json:"tool"`
		StartTime    time.Time `json:"start_time"`
		MatchCount   int       `json:"match_count"`
		Score        float64   `json:"score"`
		Snippet      string    `json:"snippet"`
		SnippetRole  string    `json:"snippet_role"`
	}

	out := make([]jsonResult, len(results))
	for i, r := range results {
		out[i] = jsonResult{
			SessionID:    r.SessionID,
			Project:      r.Project,
			FirstQuery:   r.FirstQuery,
			MessageCount: r.MessageCount,
			Tool:         r.Tool,
			StartTime:    r.StartTime,
			MatchCount:   r.MatchCount,
			Score:        math.Round(r.FinalScore*1000) / 1000,
			Snippet:      strings.ReplaceAll(strings.ReplaceAll(r.Snippet, "⟪", ""), "⟫", ""),
			SnippetRole:  r.SnippetRole,
		}
	}

	data, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(data))
}

// runRawSearch preserves the legacy message-level output.
func runRawSearch(query string, startTime time.Time) {
	fmt.Printf("Searching for: %s\n", query)
	fmt.Println()

	results, err := db.Search(query, searchLimit)
	if err != nil {
		fmt.Printf("Search error: %v\n", err)
		return
	}

	if len(results) == 0 {
		fmt.Println("No results found.")
		return
	}

	fmt.Printf("Found %d results:\n\n", len(results))

	for i, r := range results {
		fmt.Printf("[%d] %s\n", i+1, r.Project)
		fmt.Printf("    Tool: %s | Role: %s\n", r.Tool, r.Role)
		fmt.Printf("    Session: %s\n", r.SessionID)

		snippet := r.Snippet
		snippet = strings.ReplaceAll(snippet, "⟪", "\033[1;33m")
		snippet = strings.ReplaceAll(snippet, "⟫", "\033[0m")
		fmt.Printf("    Match: %s\n", snippet)
		fmt.Printf("    Relevance: %.2f\n", r.Rank)
		fmt.Println()
	}

	fmt.Printf("Search completed in %v\n", time.Since(startTime))
}

// cleanTitle extracts a clean, human-readable title from first_query.
// Strips XML tags, system-reminder noise, takes first meaningful line.
func cleanTitle(s string) string {
	// Strip XML/HTML tags
	s = xmlTagRe.ReplaceAllString(s, "")
	// Take first non-empty, non-noise line
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") {
			continue
		}
		// Skip common noise patterns
		if strings.HasPrefix(line, "[ALL BACKGROUND") || strings.HasPrefix(line, "**Completed") ||
			strings.HasPrefix(line, "- `bg_") || strings.HasPrefix(line, "Use `background") ||
			strings.HasPrefix(line, "[BRAINSTORM") || strings.HasPrefix(line, "Note:") {
			continue
		}
		return line
	}
	// All lines were noise — return empty to fall back to project name
	return ""
}

// relativeTime returns a human-friendly relative time string.
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%dd ago", days)
	default:
		return t.Format("Jan 2")
	}
}

// relativeTimeShort returns a compact relative time for context injection.
func relativeTimeShort(t time.Time) string {
	if t.IsZero() {
		return "?"
	}
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return t.Format("Jan2")
	}
}

func init() {
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 5, "Maximum sessions to show")
	searchCmd.Flags().IntVarP(&searchDays, "days", "d", 0, "Limit to last N days")
	searchCmd.Flags().BoolVar(&searchContext, "context", false, "Token-efficient output for AI context injection")
	searchCmd.Flags().BoolVar(&searchJSON, "json", false, "Full JSON output for programmatic use")
	searchCmd.Flags().BoolVar(&searchRaw, "raw", false, "Legacy message-level output")
	rootCmd.AddCommand(searchCmd)
}
