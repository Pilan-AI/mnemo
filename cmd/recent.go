package cmd

import (
	"fmt"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
	"github.com/spf13/cobra"
)

var recentDays int

var recentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Show recent AI coding sessions",
	Long:  "List your most recent AI coding conversations across all tools.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := db.InitDB(); err != nil {
			fmt.Printf("Error opening database: %v\n", err)
			fmt.Println("Run 'mnemo index' first to build the search index.")
			return
		}
		defer db.CloseDB()

		sessions, err := db.GetRecentSessions(50)
		if err != nil {
			fmt.Printf("Error getting sessions: %v\n", err)
			return
		}

		if len(sessions) == 0 {
			fmt.Println("No sessions found. Run 'mnemo index' first.")
			return
		}

		cutoff := time.Now().AddDate(0, 0, -recentDays)

		fmt.Printf("Recent sessions (last %d days):\n\n", recentDays)

		count := 0
		for _, session := range sessions {
			indexedAt, ok := session["indexedAt"].(time.Time)
			if !ok {
				continue
			}

			if recentDays > 0 && indexedAt.Before(cutoff) {
				continue
			}

			project := session["project"].(string)
			messages := session["messages"].(int)
			tool := session["tool"].(string)
			firstQuery := ""
			if fq, ok := session["firstQuery"].(string); ok {
				firstQuery = fq
			}

			fmt.Printf("[%s] %s (%d messages)\n", tool, project, messages)
			if firstQuery != "" {
				fmt.Printf("       %s\n", truncate(firstQuery, 70))
			}

			count++
			if count >= 20 {
				fmt.Printf("\n... showing top 20. Use 'mnemo search' for more.\n")
				break
			}
		}

		if count == 0 {
			fmt.Println("No sessions found in the specified time range.")
		}
	},
}

func init() {
	recentCmd.Flags().IntVarP(&recentDays, "days", "d", 7, "Show sessions from last N days")
	rootCmd.AddCommand(recentCmd)
}
