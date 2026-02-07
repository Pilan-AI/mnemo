package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Build-time variables, overridden via -ldflags during goreleaser builds.
var (
	Version   = "1.0.0"
	BuildDate = "2026-01-26"
	GitCommit = "dev"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("mnemo %s\n", Version)
		fmt.Printf("Build date: %s\n", BuildDate)
		fmt.Printf("Git commit: %s\n", GitCommit)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
