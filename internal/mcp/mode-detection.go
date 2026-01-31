//go:build ignore

package mcp

import (
	"os"
	"path/filepath"
)

// ModeDetector determines which hook should be used based on active agent
type ModeDetector struct{}

// DetectActiveTool detects which AI tool is currently active
// Returns the tool name and whether a hook is available
func (d *ModeDetector) DetectActiveTool() (string, bool) {
	envTools := os.Getenv("OPENCODE_ACTIVE_TOOL")

	if envTools != "" {
		return "opencode", true
	}

	envTools = os.Getenv("CLAUDE_CODE_ACTIVE_TOOL")

	if envTools != "" {
		return "claude-code", true
	}

	return "unknown", false
}

// GetHookType returns the appropriate hook type for the active tool
func (d *ModeDetector) GetHookType(tool string) string {
	switch tool {
	case "opencode":
		return "opencode-hook"
	case "claude-code":
		return "claude-code-hook"
	default:
		return "unknown-hook"
	}
}

// ShouldUseHook checks if the active tool has a supported hook
func (d *ModeDetector) ShouldUseHook() bool {
	_, hasHook := d.DetectActiveTool()
	return hasHook
}

// GetActiveAgent returns the name of the active agent
func (d *ModeDetector) GetActiveAgent() string {
	envAgent := os.Getenv("OMC_AGENT")

	if envAgent != "" {
		return envAgent
	}

	return "unknown"
}

// CheckHookAvailability checks if a hook file exists for the active tool
func (d *ModeDetector) CheckHookAvailability(tool string) bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	hookPath := filepath.Join(homeDir, ".mnemo", "hooks", tool+".ts")

	if _, err := os.Stat(hookPath); err == nil {
		return true
	}

	return false
}

// GetRecommendedHookForTool returns the recommended hook for a given tool
func (d *ModeDetector) GetRecommendedHookForTool(tool string) string {
	if d.CheckHookAvailability(tool) {
		return d.GetHookType(tool)
	}

	return ""
}
