//go:build ignore

package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	Version = "1.0.0"
)

type Server struct {
	db      *sql.DB
	handler *server.MCPServer
}

func (s *Server) handleInject(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params struct {
		Query string `json:"query"`
		Mode  string `json:"mode,omitempty"`
	}

	if err := json.Unmarshal(req.Arguments.RawMessage, &params); err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	mode := s.loadModeForLogging()
	if params.Mode != "" {
		mode = params.Mode
	}

	slog.Info("Mnemo injection triggered", "mode", mode, "query", params.Query)

	results, err := s.db.SearchSessions(params.Query, 10)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	if len(results) == 0 {
		slog.Info("No results found for query", "query", params.Query)
		return mcp.NewToolResultText("")
	}

	var context string
	for i, r := range results {
		if i > 0 {
			context += "\n\n"
		}
		context += fmt.Sprintf("[%s] %s (from %s)", r.ID, r.Snippet, r.Timestamp)
	}

	fullContext := fmt.Sprintf("## Mnemo Context\n\n%s\n---", context)

	slog.Info("Context injection successful", "query", params.Query, "results", len(results))

	return mcp.NewToolResultText(fullContext)
}
