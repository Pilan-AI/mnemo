// Package cmd implements all mnemo CLI commands using cobra.
//
// Command tree:
//
//	mnemo index       — Scan and index AI tool session history
//	mnemo search      — Full-text search across all indexed sessions
//	mnemo recent      — List recent sessions
//	mnemo context     — Generate markdown context summary for a project
//	mnemo blocks      — Show 5-hour usage windows (Claude rate limit tracking)
//	mnemo projects    — Manage tracked project directories
//	mnemo add         — Index documentation/notes as a knowledge source
//	mnemo tools       — Detect installed AI coding tools
//	mnemo serve       — Start MCP server for Claude Desktop/Code integration
//	mnemo install     — Install plugins for Claude Code/Desktop and OpenCode
//	mnemo onboarding  — Run the interactive first-run experience
//	mnemo configure   — Set context injection mode (off/helper/assistant)
//	mnemo version     — Print version info
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Set mnemo injection mode (off/helper/assistant)",
	Long: `Configure mnemo's context injection behavior.

Available modes:
  off       - No auto-injection
  helper    - Keyword-based filtering (code/debug only)
  assistant - Inject context for every message`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Usage: mnemo configure <mode>")
			fmt.Println("Available modes: off, helper, assistant")
			return
		}
		mode := strings.ToLower(args[0])

		homeDir, _ := os.UserHomeDir()
		mnemoDir := filepath.Join(homeDir, ".mnemo")

		if err := os.MkdirAll(mnemoDir, 0755); err != nil {
			fmt.Printf("Error creating config directory: %v\n", err)
			os.Exit(1)
		}

		validModes := map[string]bool{
			"off":       true,
			"helper":    true,
			"assistant": true,
		}

		if !validModes[mode] {
			fmt.Printf("Invalid mode: %s\n", mode)
			fmt.Println("Available modes: off, helper, assistant")
			os.Exit(1)
		}

		configPath := filepath.Join(mnemoDir, "config.json")
		config := fmt.Sprintf(`{
  "injection_mode": "%s"
 }`, mode)

		if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
			fmt.Printf("Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ mnemo injection mode set to: %s\n", mode)
		fmt.Println("\nRestart your AI tool (Claude Code / OpenCode) to apply changes.")
	},
}

var rootCmd = &cobra.Command{
	Use:   "mnemo",
	Short: "Memory for AI-assisted development",
	Long: `mnemo - Memory for AI-assisted development

The faintest ink is more powerful than the strongest memory.
Index your past to build your future.

Your AI coding sessions — indexed, searchable, never forgotten.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Skip onboarding for commands that don't need it
		name := cmd.Name()
		if name == "onboarding" || name == "version" || name == "help" || name == "completion" || name == "status" {
			return
		}

		// First run: trigger onboarding if no database exists
		home, _ := os.UserHomeDir()
		dbPath := filepath.Join(home, ".mnemo", "mnemo.db")
		if !pathExists(dbPath) {
			runOnboarding()
		}
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(configureCmd)
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
