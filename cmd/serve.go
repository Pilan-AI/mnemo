package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start mnemo MCP server",
	Long: `Start mnemo as an MCP (Model Context Protocol) server.

This allows Claude Desktop and other MCP clients to search your indexed
conversations directly through the MCP protocol.

Add to your Claude Desktop config:
  {
    "mcpServers": {
      "mnemo": {
        "command": "mnemo",
        "args": ["serve"]
      }
    }
  }`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := serveMCP(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

// serveMCP starts an MCP (Model Context Protocol) server over stdio, exposing
// mnemo's search, context, recent, and tools capabilities to MCP clients like
// Claude Desktop and Claude Code.
func serveMCP() error {
	s := server.NewMCPServer(
		"mnemo",
		"1.0.0",
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	if err := db.InitDB(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	searchTool := mcp.NewTool("mnemo_search",
		mcp.WithDescription("Search across all indexed AI coding conversations"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query - can be keywords, code patterns, or natural language"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of results to return (default: 10)"),
		),
		mcp.WithString("project",
			mcp.Description("Filter results by project name (optional)"),
		),
	)

	s.AddTool(searchTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := request.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}

		limit := request.GetInt("limit", 5)
		projectFilter := request.GetString("project", "")

		results, err := db.SearchGrouped(query, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}

		if len(results) == 0 {
			return mcp.NewToolResultText("No past sessions found."), nil
		}

		if projectFilter != "" {
			filtered := make([]db.SessionMatch, 0)
			for _, r := range results {
				if strings.EqualFold(r.Project, projectFilter) {
					filtered = append(filtered, r)
				}
			}
			results = filtered
		}

		// Tier 2: Token-efficient structured output for AI context injection
		var output strings.Builder
		output.WriteString(fmt.Sprintf("PAST_SESSIONS(query=\"%s\"):\n", query))

		for i, r := range results {
			ago := formatRelativeShort(r.StartTime)
			title := r.FirstQuery
			if title == "" {
				title = r.Project
			}
			if len(title) > 100 {
				title = title[:97] + "..."
			}

			output.WriteString(fmt.Sprintf("%d. [%s/%s/%dhits] \"%s\" — %s\n",
				i+1, r.Project, ago, r.MatchCount, title, r.Tool))
		}

		return mcp.NewToolResultText(output.String()), nil
	})

	contextTool := mcp.NewTool("mnemo_context",
		mcp.WithDescription("Generate context summary for a project based on past sessions"),
		mcp.WithString("project",
			mcp.Required(),
			mcp.Description("Project name to get context for"),
		),
	)

	s.AddTool(contextTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		project, err := request.RequireString("project")
		if err != nil {
			return mcp.NewToolResultError("project parameter is required"), nil
		}

		results, err := db.SearchGrouped(project, 10)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("context search failed: %v", err)), nil
		}

		if len(results) == 0 {
			return mcp.NewToolResultText(fmt.Sprintf("No context found for project: %s", project)), nil
		}

		// Filter to matching project
		filtered := make([]db.SessionMatch, 0)
		for _, r := range results {
			if strings.EqualFold(r.Project, project) || strings.Contains(strings.ToLower(r.Project), strings.ToLower(project)) {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) > 0 {
			results = filtered
		}

		var output strings.Builder
		output.WriteString(fmt.Sprintf("PROJECT_CONTEXT(%s):\n", project))

		for i, r := range results {
			ago := formatRelativeShort(r.StartTime)
			title := r.FirstQuery
			if title == "" {
				title = "(no query)"
			}
			if len(title) > 100 {
				title = title[:97] + "..."
			}
			output.WriteString(fmt.Sprintf("%d. [%s/%dmsg] \"%s\" — %s\n",
				i+1, ago, r.MessageCount, title, r.Tool))
		}

		return mcp.NewToolResultText(output.String()), nil
	})

	recentTool := mcp.NewTool("mnemo_recent",
		mcp.WithDescription("Show recent AI coding sessions"),
		mcp.WithNumber("limit",
			mcp.Description("Maximum number of sessions to return (default: 10)"),
		),
	)

	s.AddTool(recentTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := request.GetInt("limit", 10)

		sessions, err := db.GetRecentSessions(limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get recent sessions: %v", err)), nil
		}

		if len(sessions) == 0 {
			return mcp.NewToolResultText("No sessions found."), nil
		}

		var output strings.Builder
		output.WriteString(fmt.Sprintf("Recent sessions (limit %d):\n\n", limit))

		for i, session := range sessions {
			project := session["project"].(string)
			firstQuery := session["firstQuery"].(string)
			msgCount := session["messages"].(int)
			tool := session["tool"].(string)

			output.WriteString(fmt.Sprintf("[%d] %s\n", i+1, project))
			output.WriteString(fmt.Sprintf("    First query: %s\n", shorten(firstQuery, 80)))
			output.WriteString(fmt.Sprintf("    Messages: %d\n", msgCount))
			output.WriteString(fmt.Sprintf("    Tool: %s\n\n", tool))
		}

		return mcp.NewToolResultText(output.String()), nil
	})

	toolsTool := mcp.NewTool("mnemo_tools",
		mcp.WithDescription("List detected AI coding tools on this system"),
	)

	s.AddTool(toolsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tools := detectTools()

		var output strings.Builder
		output.WriteString("Detected AI coding tools:\n\n")

		for i, tool := range tools {
			status := "✗"
			if tool.Installed {
				status = "✓"
			}
			output.WriteString(fmt.Sprintf("[%d] %s %s\n", i+1, status, tool.Name))
			output.WriteString(fmt.Sprintf("    Path: %s\n\n", tool.Path))
		}

		detected := 0
		for _, tool := range tools {
			if tool.Installed {
				detected++
			}
		}
		output.WriteString(fmt.Sprintf("Detected: %d/%d tools\n", detected, len(tools)))

		return mcp.NewToolResultText(output.String()), nil
	})

	log.Printf("mnemo MCP server starting...")

	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

type Tool struct {
	Name      string
	Path      string
	Installed bool
}

func detectTools() []Tool {
	home, _ := os.UserHomeDir()
	tools := []Tool{
		{Name: "Claude Code", Path: filepath.Join(home, ".claude", "projects")},
		{Name: "Opencode", Path: filepath.Join(home, ".local", "share", "opencode")},
		{Name: "Cursor", Path: filepath.Join(appSupportDir(home, "Cursor"), "User", "globalStorage")},
		{Name: "Gemini CLI", Path: filepath.Join(home, ".gemini")},
		{Name: "Windsurf", Path: appSupportDir(home, "Windsurf")},
		{Name: "Aider", Path: filepath.Join(home, ".aider.chat.history.md")},
		{Name: "GitHub Copilot", Path: filepath.Join(home, ".config", "github-copilot")},
		{Name: "Amp", Path: filepath.Join(home, ".amp")},
	}

	for i := range tools {
		if _, err := os.Stat(tools[i].Path); err == nil {
			tools[i].Installed = true
		}
	}

	return tools
}

func shorten(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatRelativeShort returns a compact relative time for MCP context.
func formatRelativeShort(t time.Time) string {
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
