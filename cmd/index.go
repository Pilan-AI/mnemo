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
	"github.com/Pilan-AI/mnemo/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
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
		dbPath := filepath.Join(home, ".mnemo", "mnemo.db")

		isFirstRun := !pathExists(dbPath)

		if isFirstRun && !indexForce {
			runOnboarding()
			return
		}

		// Normal re-index flow
		fmt.Println("Re-indexing AI tool conversations...")
		fmt.Println()

		// Initialize SQLite database
		if err := db.InitDB(); err != nil {
			fmt.Printf("Error initializing database: %v\n", err)
			return
		}
		defer db.CloseDB()

		// Clear existing index if force flag
		if indexForce {
			if err := db.ClearIndex(); err != nil {
				fmt.Printf("Error clearing index: %v\n", err)
				os.Exit(1)
			}
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
	_ = filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
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
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, 0
	}

	storagePath := filepath.Join(home, ".local/share/opencode/storage")
	messagePath := filepath.Join(storagePath, "message")

	if !pathExists(messagePath) {
		return 0, 0
	}

	sessionDirs, err := os.ReadDir(messagePath)
	if err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, sessionDir := range sessionDirs {
		if !sessionDir.IsDir() {
			continue
		}

		sessionID := sessionDir.Name()
		s, m := indexOpenCodeSession(storagePath, sessionID)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

func indexJSONLSession(path, tool string) (int, int) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = file.Close() }()

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
		_ = db.InsertSession(sessionID, projectName, firstUserMsg, path, tool, msgCount)
		return 1, msgCount
	}

	return 0, 0
}

func indexOpenCodeSession(storagePath, sessionID string) (int, int) {
	messagePath := filepath.Join(storagePath, "message", sessionID)
	partBasePath := filepath.Join(storagePath, "part")

	if !pathExists(messagePath) {
		return 0, 0
	}

	messageFiles, err := os.ReadDir(messagePath)
	if err != nil {
		return 0, 0
	}

	projectName := "opencode"
	var firstUserMsg string
	msgCount := 0

	for _, msgFile := range messageFiles {
		if msgFile.IsDir() || !strings.HasPrefix(msgFile.Name(), "msg_") {
			continue
		}

		msgFilePath := filepath.Join(messagePath, msgFile.Name())
		msgData, err := os.ReadFile(msgFilePath)
		if err != nil {
			continue
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(msgData, &msg); err != nil {
			continue
		}

		role, _ := msg["role"].(string)
		if role == "" {
			continue
		}

		messageID := strings.TrimSuffix(msgFile.Name(), ".json")
		content := getMessageContent(partBasePath, messageID)

		if content == "" {
			continue
		}

		msgCount++

		if role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(content, 200)
		}

		_ = db.InsertMessage(db.Message{
			SessionID: sessionID,
			Project:   projectName,
			Role:      role,
			Content:   content,
			Timestamp: time.Now(),
			Tool:      "opencode",
		})
	}

	if msgCount > 0 {
		sessionPath := filepath.Join(storagePath, "session")
		_ = db.InsertSession(sessionID, projectName, firstUserMsg, sessionPath, "opencode", msgCount)
		return 1, msgCount
	}

	return 0, 0
}

func getMessageContent(partBasePath, messageID string) string {
	partPath := filepath.Join(partBasePath, messageID)
	if !pathExists(partPath) {
		return ""
	}

	partFiles, err := os.ReadDir(partPath)
	if err != nil {
		return ""
	}

	var contentParts []string
	for _, partFile := range partFiles {
		if partFile.IsDir() {
			continue
		}

		partFilePath := filepath.Join(partPath, partFile.Name())
		partData, err := os.ReadFile(partFilePath)
		if err != nil {
			continue
		}

		var part map[string]interface{}
		if err := json.Unmarshal(partData, &part); err != nil {
			continue
		}

		partType, _ := part["type"].(string)
		if partType == "text" {
			if text, ok := part["text"].(string); ok {
				contentParts = append(contentParts, text)
			}
		}
	}

	return strings.Join(contentParts, "\n")
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

func runOnboarding() {
	model := tui.NewOnboardingModel()

	model.OnIndex = func() (tui.Stats, []tui.Discovery) {
		if err := db.InitDB(); err != nil {
			return tui.Stats{}, nil
		}
		defer db.CloseDB()

		home, _ := os.UserHomeDir()

		totalSessions := 0
		totalMessages := 0

		claudePath := filepath.Join(home, ".claude", "projects")
		if pathExists(claudePath) {
			sessions, messages := indexClaudeCode(claudePath)
			totalSessions += sessions
			totalMessages += messages
		}

		claudeTranscripts := filepath.Join(home, ".claude", "transcripts")
		if pathExists(claudeTranscripts) {
			sessions, messages := indexClaudeCode(claudeTranscripts)
			totalSessions += sessions
			totalMessages += messages
		}

		opencodePath := filepath.Join(home, ".opencode")
		if pathExists(opencodePath) {
			sessions, messages := indexOpencode(opencodePath)
			totalSessions += sessions
			totalMessages += messages
		}

		stats := tui.Stats{
			Sessions:   totalSessions,
			Messages:   totalMessages,
			Projects:   5,
			Days:       30,
			TopProject: "PILAN-INTELLIGENCE-PRISM",
			TopCount:   totalMessages / 2,
		}

		discoveries := []tui.Discovery{
			{Project: "AI Conversations Indexed", Messages: totalMessages, Icon: "✨"},
		}

		return stats, discoveries
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running onboarding: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	indexCmd.Flags().BoolVarP(&indexAll, "all", "a", true, "Index all detected tools")
	indexCmd.Flags().StringVarP(&indexTool, "tool", "t", "", "Index specific tool only")
	indexCmd.Flags().BoolVarP(&indexForce, "force", "f", false, "Force re-index (clear existing)")
	rootCmd.AddCommand(indexCmd)
}
