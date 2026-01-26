package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

var recentDays int

var recentCmd = &cobra.Command{
	Use:   "recent",
	Short: "Show recent AI coding sessions",
	Long:  "List your most recent AI coding conversations across all tools.",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()
		mnemoDir := filepath.Join(home, ".mnemo")
		
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
		
		// Sort by indexed time (most recent first)
		sort.Slice(index, func(i, j int) bool {
			ti, _ := time.Parse(time.RFC3339, index[i]["indexed"].(string))
			tj, _ := time.Parse(time.RFC3339, index[j]["indexed"].(string))
			return ti.After(tj)
		})
		
		// Filter by days if specified
		cutoff := time.Now().AddDate(0, 0, -recentDays)
		
		fmt.Printf("Recent sessions (last %d days):\n\n", recentDays)
		
		count := 0
		for _, entry := range index {
			if recentDays > 0 {
				indexed, _ := time.Parse(time.RFC3339, entry["indexed"].(string))
				if indexed.Before(cutoff) {
					continue
				}
			}
			
			project := entry["project"].(string)
			messages := int(entry["messages"].(float64))
			firstQuery := ""
			if fq, ok := entry["firstQuery"].(string); ok {
				firstQuery = fq
			}
			
			fmt.Printf("• %s (%d messages)\n", project, messages)
			if firstQuery != "" {
				fmt.Printf("  └─ %s\n", truncate(firstQuery, 70))
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
