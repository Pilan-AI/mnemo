package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
	"github.com/spf13/cobra"
)

var (
	indexAll    bool
	indexTool   string
	indexForce  bool
	indexErrors int
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index AI tool conversation history",
	Long:  "Scan and index conversation history from all detected AI coding tools.",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()

		fmt.Println("Indexing AI tool conversations...")
		fmt.Println()

		// Initialize SQLite database
		if err := db.InitDB(); err != nil {
			fmt.Printf("Error initializing database: %v\n", err)
			return
		}
		defer db.CloseDB()

		// Clear existing index if force flag
		if indexForce {
			db.ClearIndex()
		}

		totalSessions := 0
		totalMessages := 0

		// Index Claude Code - main directory
		claudePath := filepath.Join(home, ".claude", "projects")
		if pathExists(claudePath) {
			sessions, messages := indexClaudeCode(claudePath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Claude Code (projects): %d sessions, %d messages\n", sessions, messages)
		}

		// Index Claude Code - transcripts directory
		claudeTranscripts := filepath.Join(home, ".claude", "transcripts")
		if pathExists(claudeTranscripts) {
			sessions, messages := indexClaudeCode(claudeTranscripts)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Claude Code (transcripts): %d sessions, %d messages\n", sessions, messages)
		}

		// Index Claude Code - backup directory (for users who reinstalled)
		claudeBackup := filepath.Join(home, ".claude-backup")
		if pathExists(claudeBackup) {
			sessions, messages := indexClaudeCode(claudeBackup)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Claude Code (backup): %d sessions, %d messages\n", sessions, messages)
		}

		// Index Opencode
		opencodePath := filepath.Join(home, ".opencode")
		if pathExists(opencodePath) {
			sessions, messages := indexOpencode(opencodePath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Opencode: %d sessions, %d messages\n", sessions, messages)
		}

		fmt.Println()
		fmt.Printf("Total: %d sessions, %d messages indexed\n", totalSessions, totalMessages)
		if indexErrors > 0 {
			fmt.Printf("  (Skipped %d messages due to errors)\n", indexErrors)
		}
		fmt.Printf("Index saved to: ~/.mnemo/mnemo.db\n")

		// Warn if fewer than 100 sessions found
		if totalSessions < 100 && totalSessions > 0 {
			fmt.Println()
			fmt.Println("⚠️  Only", totalSessions, "sessions found. This seems low.")
			fmt.Println("   If you have more sessions, check if they're in a different location:")
			fmt.Println("   - ~/.claude-backup/ (if you reinstalled Claude)")
			fmt.Println("   - Different user account")
			fmt.Println("   - External drive")
			fmt.Println()
			fmt.Println("   Use 'mnemo index --path /custom/path' to index additional locations.")
		}

		fmt.Println()
		fmt.Println("Run 'mnemo search <query>' to search your history.")
	},
}

func indexClaudeCode(basePath string) (int, int) {
	sessions := 0
	messages := 0

	// Recursively walk through all directories to find .jsonl files
	filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip directories, only process .jsonl files
		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".jsonl") {
			s, m := indexJSONLSession(path, "claude")
			sessions += s
			messages += m
		}

		return nil
	})

	return sessions, messages
}

func indexOpencode(basePath string) (int, int) {
	sessions := 0
	messages := 0

	// Walk through sessions directory
	sessionsPath := filepath.Join(basePath, "sessions")
	if !pathExists(sessionsPath) {
		return 0, 0
	}

	entries, err := os.ReadDir(sessionsPath)
	if err != nil {
		return 0, 0
	}

	for _, entry := range entries {
		if entry.IsDir() {
			sessionPath := filepath.Join(sessionsPath, entry.Name(), "session.json")
			if pathExists(sessionPath) {
				s, m := indexOpenCodeSession(sessionPath)
				sessions += s
				messages += m
			}
		}
	}

	return sessions, messages
}

func indexJSONLSession(path, tool string) (int, int) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	info, _ := file.Stat()
	if info.Size() == 0 {
		return 0, 0
	}

	// Read file content
	data, _ := io.ReadAll(file)
	lines := strings.Split(string(data), "\n")

	var firstUserMsg string
	sessionID := filepath.Base(path)
	sessionID = strings.TrimSuffix(sessionID, ".jsonl")
	projectName := extractProjectName(path)
	msgCount := 0

	for _, line := range lines {
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Skip non-message entries
		entryType, _ := entry["type"].(string)
		if entryType != "user" && entryType != "assistant" {
			continue
		}

		// Extract message content
		var content string
		var role string

		// Try new format first: {"type":"user","message":{"role":"user","content":"..."}}
		if msg, ok := entry["message"].(map[string]interface{}); ok {
			role, _ = msg["role"].(string)

			// Handle different content formats
			switch c := msg["content"].(type) {
			case string:
				content = c
			case []interface{}:
				// Claude's content array format
				for _, item := range c {
					if block, ok := item.(map[string]interface{}); ok {
						if text, ok := block["text"].(string); ok {
							content += text + " "
						}
					}
				}
			}
		} else {
			// Try old format: {"type":"user","content":"..."}
			role = entryType // "user" or "assistant"
			if c, ok := entry["content"].(string); ok {
				content = c
			}
		}

		if content == "" {
			continue
		}

		// Capture first user message
		if role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(content, 200)
		}

		// Insert into database
		err := db.InsertMessage(db.Message{
			SessionID: sessionID,
			Project:   projectName,
			Role:      role,
			Content:   content,
			Timestamp: time.Now(),
			Tool:      tool,
		})
		if err != nil {
			indexErrors++
			continue
		}
		msgCount++
	}

	// Insert session metadata
	if msgCount > 0 {
		db.InsertSession(sessionID, projectName, firstUserMsg, path, tool, msgCount)
		return 1, msgCount
	}

	return 0, 0
}

func indexOpenCodeSession(path string) (int, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0
	}

	var session map[string]interface{}
	if err := json.Unmarshal(data, &session); err != nil {
		return 0, 0
	}

	sessionID, _ := session["id"].(string)
	if sessionID == "" {
		sessionID = filepath.Base(filepath.Dir(path))
	}

	projectName := "opencode"
	if cwd, ok := session["cwd"].(string); ok {
		projectName = filepath.Base(cwd)
	}

	messages, ok := session["messages"].([]interface{})
	if !ok {
		return 0, 0
	}

	var firstUserMsg string
	msgCount := 0

	for _, m := range messages {
		msg, ok := m.(map[string]interface{})
		if !ok {
			continue
		}

		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		if content == "" {
			continue
		}

		msgCount++

		if role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(content, 200)
		}

		db.InsertMessage(db.Message{
			SessionID: sessionID,
			Project:   projectName,
			Role:      role,
			Content:   content,
			Timestamp: time.Now(),
			Tool:      "opencode",
		})
	}

	if msgCount > 0 {
		db.InsertSession(sessionID, projectName, firstUserMsg, path, "opencode", msgCount)
		return 1, msgCount
	}

	return 0, 0
}

func extractProjectName(path string) string {
	// Extract from path like: -Users-raghu-Projects-PILAN-INTELLIGENCE-PRISM
	dir := filepath.Dir(path)
	parts := strings.Split(dir, string(os.PathSeparator))

	for _, part := range parts {
		if strings.HasPrefix(part, "-") && strings.Contains(part, "-") {
			// Convert: -Volumes-UMBRA-BACKUP-PERSONAL-FORGE -> PERSONAL-FORGE
			segments := strings.Split(part, "-")
			if len(segments) > 2 {
				// Find last meaningful segment
				for i := len(segments) - 1; i >= 0; i-- {
					if segments[i] != "" && segments[i] != "Users" && segments[i] != "Volumes" {
						return strings.Join(segments[max(1, i-1):], "-")
					}
				}
			}
			return part
		}
	}

	return filepath.Base(dir)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func init() {
	indexCmd.Flags().BoolVarP(&indexAll, "all", "a", true, "Index all detected tools")
	indexCmd.Flags().StringVarP(&indexTool, "tool", "t", "", "Index specific tool only")
	indexCmd.Flags().BoolVarP(&indexForce, "force", "f", false, "Force re-index (clear existing)")
	rootCmd.AddCommand(indexCmd)
}
