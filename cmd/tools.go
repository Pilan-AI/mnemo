// tools.go detects installed AI coding tools on the local system and provides
// path helper functions for locating tool-specific session directories.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

// AITool describes a supported AI coding assistant and the filesystem path
// where it stores conversation history.
type AITool struct {
	Name     string
	Path     string
	Format   string
	Detected bool
}

// appSupportDir returns the OS-specific application support directory for an app.
// macOS: ~/Library/Application Support/{app}
// Linux: ~/.config/{app}
// Windows: %APPDATA%/{app}
func appSupportDir(home, app string) string {
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", app)
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, app)
		}
		return filepath.Join(home, "AppData", "Roaming", app)
	default: // linux, freebsd, etc.
		return filepath.Join(home, ".config", app)
	}
}

// getSupportedTools returns the full list of AI tools with their expected paths.
func getSupportedTools(home string) []AITool {
	return []AITool{
		{Name: "Claude Code", Path: filepath.Join(home, ".claude", "projects"), Format: "jsonl"},
		{Name: "Opencode", Path: filepath.Join(home, ".local", "share", "opencode"), Format: "json"},
		{Name: "Cursor", Path: filepath.Join(appSupportDir(home, "Cursor"), "User", "globalStorage"), Format: "sqlite"},
		{Name: "Gemini CLI", Path: filepath.Join(home, ".gemini", "sessions"), Format: "json"},
		{Name: "Windsurf", Path: appSupportDir(home, "Windsurf"), Format: "sqlite"},
		{Name: "Aider", Path: filepath.Join(home, ".aider.chat.history.md"), Format: "markdown"},
		{Name: "GitHub Copilot", Path: filepath.Join(home, ".config", "github-copilot"), Format: "json"},
		{Name: "Crush", Path: filepath.Join(home, ".crush", "crush.db"), Format: "sqlite"},
		{Name: "Amp", Path: filepath.Join(home, ".local", "share", "amp"), Format: "jsonl"},
		{Name: "Codex", Path: filepath.Join(home, ".codex"), Format: "jsonl"},
		{Name: "Antigravity", Path: appSupportDir(home, "Antigravity"), Format: "sqlite"},
		{Name: "Kiro", Path: appSupportDir(home, "Kiro"), Format: "sqlite"},
	}
}

var vscodeExtensions = []struct {
	Name  string
	ExtID string
}{
	{Name: "Kilo Code", ExtID: "kilocode.kilo-code"},
	{Name: "Cline", ExtID: "saoudrizwan.claude-dev"},
	{Name: "Roo Code", ExtID: "rooveterinaryinc.roo-cline"},
}

var vscodeIDEs = []string{
	"Code", "Code - Insiders", "Cursor", "Windsurf", "VSCodium", "Antigravity", "Kiro", "Trae",
}

// vscodeExtTasksPath returns the path to a VS Code extension's tasks directory for a given IDE.
func vscodeExtTasksPath(home, ide, extID string) string {
	return filepath.Join(appSupportDir(home, ide), "User", "globalStorage", extID, "tasks")
}

var toolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "List detected AI coding tools",
	Long:  "Scan your system for AI coding assistants and show which ones have conversation history available.",
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("Error: cannot determine home directory: %v\n", err)
			return
		}
		tools := getSupportedTools(home)

		fmt.Println("Scanning for AI coding tools...")
		fmt.Println()

		detected := 0
		for _, tool := range tools {
			exists := pathExists(tool.Path)

			status := "✗"
			if exists {
				status = "✓"
				detected++
			}

			fmt.Printf("  %s  %-20s %s\n", status, tool.Name, tool.Path)
		}

		fmt.Println()
		fmt.Println("VS Code Extensions (scanning all IDEs):")
		for _, ext := range vscodeExtensions {
			foundIn := []string{}
			for _, ide := range vscodeIDEs {
				tasksPath := vscodeExtTasksPath(home, ide, ext.ExtID)
				if pathExists(tasksPath) {
					foundIn = append(foundIn, ide)
				}
			}

			status := "✗"
			location := "(not found)"
			if len(foundIn) > 0 {
				status = "✓"
				detected++
				location = fmt.Sprintf("(%s)", joinStrings(foundIn, ", "))
			}

			fmt.Printf("  %s  %-20s %s\n", status, ext.Name, location)
		}

		fmt.Println()
		fmt.Printf("Detected: %d tools\n", detected)

		if detected > 0 {
			fmt.Println()
			fmt.Println("Run 'mnemo index' to index all detected tools.")
		}
	},
}

// pathExists returns true if the given filesystem path exists.
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// joinStrings concatenates non-empty strings with the given separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func init() {
	rootCmd.AddCommand(toolsCmd)
}
