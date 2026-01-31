package cmd

import (
	"fmt"
	"os"

	"github.com/Pilan-AI/mnemo/internal/tui"
	"github.com/spf13/cobra"
)

var onboardingCmd = &cobra.Command{
	Use:   "onboarding",
	Short: "Run the visual onboarding experience",
	Long:  "Display the mnemo onboarding TUI - a beautiful introduction to mnemo's features.",
	Run: func(cmd *cobra.Command, args []string) {
		stats := tui.Stats{
			Sessions:   0,
			Messages:   0,
			Projects:   0,
			Days:       0,
			TopProject: "",
			TopCount:   0,
		}

		discoveries := []tui.Discovery{}

		err := tui.RunOnboarding(func() (tui.Stats, []tui.Discovery) {
			return stats, discoveries
		})

		if err != nil {
			fmt.Printf("Error running onboarding: %v\n", err)
			fmt.Println("\nTo get started with mnemo:")
			fmt.Println("  1. Run: mnemo index")
			fmt.Println("  2. Search: mnemo search \"your query\"")
			fmt.Println("  3. Recent: mnemo recent")
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(onboardingCmd)
}
