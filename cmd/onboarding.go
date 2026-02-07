package cmd

import (
	"github.com/spf13/cobra"
)

var onboardingCmd = &cobra.Command{
	Use:   "onboarding",
	Short: "Run the first-run setup experience",
	Long:  "Scan for AI tools, index recent history, and install plugins.",
	Run: func(cmd *cobra.Command, args []string) {
		runOnboarding()
	},
}

func init() {
	rootCmd.AddCommand(onboardingCmd)
}
