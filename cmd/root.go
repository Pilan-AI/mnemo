package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mnemo",
	Short: "Memory for AI-assisted development",
	Long: `mnemo - Memory for AI-assisted development

"Plan panni pannanum, plan panni pannala na ippudi dhan agum"
                                        — Pokkiri (2007)

Don't be the Lochak-Mochak engineer. Be the one who ships.

Your AI coding sessions — indexed, searchable, never forgotten.`,
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
