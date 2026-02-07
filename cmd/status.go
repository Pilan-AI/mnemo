package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show mnemo database stats and background index status",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()
		mnemoDir := filepath.Join(home, ".mnemo")
		dbPath := filepath.Join(mnemoDir, "mnemo.db")

		if !pathExists(dbPath) {
			fmt.Println("  No database found. Run 'mnemo index' first.")
			return
		}

		if err := db.InitDB(); err != nil {
			fmt.Printf("  Error opening database: %v\n", err)
			return
		}
		defer db.CloseDB()

		// DB stats
		var sessions, messages, tools int
		if row := db.GetDB().QueryRow("SELECT COUNT(*) FROM sessions"); row != nil {
			_ = row.Scan(&sessions)
		}
		if row := db.GetDB().QueryRow("SELECT COUNT(*) FROM messages"); row != nil {
			_ = row.Scan(&messages)
		}
		if row := db.GetDB().QueryRow("SELECT COUNT(DISTINCT tool) FROM sessions"); row != nil {
			_ = row.Scan(&tools)
		}

		fmt.Println()
		fmt.Printf("  Database: %s\n", dbPath)
		fmt.Printf("  Sessions: %d | Messages: %d | Tools: %d\n", sessions, messages, tools)

		// Background index status
		indexingPath := filepath.Join(mnemoDir, ".indexing")
		data, err := os.ReadFile(indexingPath)
		if err != nil {
			fmt.Printf("  Background index: not running\n")
		} else {
			var info struct {
				PID     int    `json:"pid"`
				Started string `json:"started"`
			}
			if json.Unmarshal(data, &info) == nil && info.PID > 0 {
				// Check if process is still alive
				proc, err := os.FindProcess(info.PID)
				alive := false
				if err == nil {
					// Signal 0 checks if process exists without killing it
					alive = proc.Signal(syscall.Signal(0)) == nil
				}

				if alive {
					started, _ := time.Parse(time.RFC3339, info.Started)
					ago := time.Since(started).Round(time.Second)
					fmt.Printf("  Background index: running (started %s ago)\n", ago)
				} else {
					// Process is dead, clean up stale file
					_ = os.Remove(indexingPath)
					fmt.Printf("  Background index: completed\n")
				}
			} else {
				_ = os.Remove(indexingPath)
				fmt.Printf("  Background index: not running\n")
			}
		}
		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
