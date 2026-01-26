package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	searchLimit int
	searchDays  int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search across all indexed conversations",
	Long:  "Full-text search across all your AI coding sessions.",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := strings.Join(args, " ")
		home, _ := os.UserHomeDir()
		mnemoDir := filepath.Join(home, ".mnemo")
		
		fmt.Printf("Searching for: %s\n", query)
		fmt.Println()
		
		// Load Claude index
		indexPath := filepath.Join(mnemoDir, "claude-index.json")
		if !pathExists(indexPath) {
			fmt.Println("No index found. Run 'mnemo index' first.")
			return
		}
		
		data, err := os.ReadFile(indexPath)
		if err != nil {
			fmt.Printf("Error reading index: %v\n", err)
			return
		}
		
		var index []map[string]interface{}
		json.Unmarshal(data, &index)
		
		results := searchIndex(index, query)
		
		if len(results) == 0 {
			fmt.Println("No results found.")
			return
		}
		
		fmt.Printf("Found %d results:\n\n", len(results))
		
		for i, result := range results {
			if i >= searchLimit {
				fmt.Printf("\n... and %d more. Use --limit to show more.\n", len(results)-searchLimit)
				break
			}
			
			project := result["project"].(string)
			firstQuery := ""
			if fq, ok := result["firstQuery"].(string); ok {
				firstQuery = fq
			}
			messages := int(result["messages"].(float64))
			
			fmt.Printf("[%d] %s\n", i+1, project)
			fmt.Printf("    Messages: %d\n", messages)
			if firstQuery != "" {
				fmt.Printf("    Query: %s\n", truncate(firstQuery, 80))
			}
			fmt.Println()
		}
	},
}

func searchIndex(index []map[string]interface{}, query string) []map[string]interface{} {
	var results []map[string]interface{}
	queryLower := strings.ToLower(query)
	
	for _, entry := range index {
		// Search in project name
		if project, ok := entry["project"].(string); ok {
			if strings.Contains(strings.ToLower(project), queryLower) {
				results = append(results, entry)
				continue
			}
		}
		
		// Search in first query
		if firstQuery, ok := entry["firstQuery"].(string); ok {
			if strings.Contains(strings.ToLower(firstQuery), queryLower) {
				results = append(results, entry)
				continue
			}
		}
		
		// TODO: Full content search with SQLite FTS5
	}
	
	return results
}

func init() {
	searchCmd.Flags().IntVarP(&searchLimit, "limit", "l", 10, "Maximum results to show")
	searchCmd.Flags().IntVarP(&searchDays, "days", "d", 0, "Limit to last N days")
	rootCmd.AddCommand(searchCmd)
}
