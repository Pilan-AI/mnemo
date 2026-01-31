//go:build ignore

package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleConfigure(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var params struct {
		Mode string `json:"mode"`
	}

	if err := json.Unmarshal(req.Arguments.RawMessage, &params); err != nil {
		return nil, fmt.Errorf("failed to parse parameters: %w", err)
	}

	var mode InjectionMode
	switch params.Mode {
	case "auto":
		mode = InjectionModeAuto
	case "on":
		mode = InjectionModeOn
	case "off":
		mode = InjectionModeOff
	default:
		return nil, fmt.Errorf("invalid mode: %s (must be auto/on/off)", params.Mode)
	}

	if err := s.saveInjectionMode(mode); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	message := fmt.Sprintf("✓ mnemo context injection set to: %s", params.Mode)
	return mcp.NewToolResultText(message)
}

func (s *Server) registerConfigureTool() {
	s.handler.RegisterTool(mcp.Tool{
		Name:        "mnemo_configure",
		Description: "Configure mnemo context injection behavior",
		InputSchema: mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]mcp.PropertySchema{
				"mode": {
					Type:        mcp.TypeString,
					Description: "Injection mode: 'auto' (default), 'on' (always), 'off' (never)",
					Required:    []string{"mode"},
				},
			},
		},
	})
}
