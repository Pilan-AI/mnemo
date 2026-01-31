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
}

func init() {
	rootCmd.AddCommand(indexCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(recentCmd)
	rootCmd.AddCommand(contextCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(toolsCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(configureCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
