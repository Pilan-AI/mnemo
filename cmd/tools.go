package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type AITool struct {
	Name     string
	Path     string
	Format   string
	Detected bool
}

var supportedTools = []AITool{
	{Name: "Claude Code", Path: "~/.claude/projects", Format: "jsonl"},
	{Name: "Opencode", Path: "~/.opencode", Format: "jsonl"},
	{Name: "Cursor", Path: "~/.cursor", Format: "sqlite"},
	{Name: "Gemini CLI", Path: "~/.gemini", Format: "json"},
	{Name: "Windsurf", Path: "~/.windsurf", Format: "json"},
	{Name: "Aider", Path: "~/.aider.chat.history.md", Format: "markdown"},
	{Name: "GitHub Copilot", Path: "~/.config/github-copilot", Format: "json"},
	{Name: "Roo Code", Path: "~/.roo", Format: "json"},
	{Name: "Kilo Code", Path: "~/.kilo", Format: "json"},
	{Name: "Amp", Path: "~/.amp", Format: "json"},
	{Name: "Cline", Path: "~/.cline", Format: "json"},
}

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "List detected AI coding tools",
	Long:  "Scan your system for AI coding assistants and show which ones have conversation history available.",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()
		
		fmt.Println("Scanning for AI coding tools...")
		fmt.Println()
		
		detected := 0
		for _, tool := range supportedTools {
			path := expandPath(tool.Path, home)
			exists := pathExists(path)
			
			status := "✗"
			if exists {
				status = "✓"
				detected++
			}
			
			fmt.Printf("  %s  %-20s %s\n", status, tool.Name, tool.Path)
		}
		
		fmt.Println()
		fmt.Printf("Detected: %d/%d tools\n", detected, len(supportedTools))
		
		if detected > 0 {
			fmt.Println()
			fmt.Println("Run 'mnemo index' to index all detected tools.")
		}
	},
}

func expandPath(path, home string) string {
	if len(path) > 0 && path[0] == '~' {
		return filepath.Join(home, path[1:])
	}
	return path
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func init() {
	rootCmd.AddCommand(toolsCmd)
}
