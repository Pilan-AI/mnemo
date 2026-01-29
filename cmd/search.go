package cmd

import (
	"fmt"
	"strings"

	"github.com/Pilan-AI/mnemo/internal/db"
	"github.com/spf13/cobra"
)

var (
	searchLimit int
	searchDays  int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across all indexed conversations",
	Long:  "Full-text search across all your AI coding sessions using SQLite FTS5.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.Join(args, " ")

		fmt.Printf("Searching for: %s\n", query)
		fmt.Println()

		// Initialize database
		if err := db.InitDB(); err != nil {
			fmt.Printf("Error opening database: %v\n", err)
			fmt.Println("Run 'mnemo index' first to build the search index.")
			return
		}
		defer db.CloseDB()

		// Perform FTS5 search
		results, err := db.Search(query, searchLimit)
		if err != nil {
			fmt.Printf("Search error: %v\n", err)
			return
		}

		if len(results) == 0 {
			fmt.Println("No results found.")
			fmt.Println()
			fmt.Println("Tips:")
			fmt.Println("  - Try different keywords")
			fmt.Println("  - Run 'mnemo index --force' to rebuild the index")
			return
		}

		fmt.Printf("Found %d results:\n\n", len(results))

		for i, r := range results {
			fmt.Printf("[%d] %s\n", i+1, r.Project)
			fmt.Printf("    Role: %s\n", r.Role)

			// Show snippet with highlights
			snippet := r.Snippet
			snippet = strings.ReplaceAll(snippet, ">>>", "\033[1;33m") // Bold yellow
			snippet = strings.ReplaceAll(snippet, "<<<", "\033[0m")    // Reset
			fmt.Printf("    Match: %s\n", snippet)
			fmt.Println()
		}
	},
}

func init() {
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 10, "Maximum results to show")
	searchCmd.Flags().IntVarP(&searchDays, "days", "d", 0, "Limit to last N days")
	rootCmd.AddCommand(searchCmd)
}
