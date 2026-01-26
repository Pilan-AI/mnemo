package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	indexAll  bool
	indexTool string
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index AI tool conversation history",
	Long:  "Scan and index conversation history from all detected AI coding tools.",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()
		mnemoDir := filepath.Join(home, ".mnemo")
		
		// Create mnemo directory
		os.MkdirAll(mnemoDir, 0755)
		
		fmt.Println("Indexing AI tool conversations...")
		fmt.Println()
		
		totalSessions := 0
		totalMessages := 0
		
		// Index Claude Code
		claudePath := filepath.Join(home, ".claude", "projects")
		if pathExists(claudePath) {
			sessions, messages := indexClaudeCode(claudePath, mnemoDir)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Claude Code: %d sessions, %d messages\n", sessions, messages)
		}
		
		// Index Opencode
		opencodePath := filepath.Join(home, ".opencode")
		if pathExists(opencodePath) {
			sessions, messages := indexOpencode(opencodePath, mnemoDir)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Opencode: %d sessions, %d messages\n", sessions, messages)
		}
		
		fmt.Println()
		fmt.Printf("Total: %d sessions, %d messages indexed\n", totalSessions, totalMessages)
		fmt.Printf("Index saved to: %s\n", mnemoDir)
		fmt.Println()
		fmt.Println("Run 'mnemo search <query>' to search your history.")
	},
}

func indexClaudeCode(basePath, mnemoDir string) (int, int) {
	sessions := 0
	messages := 0
	
	// Walk through all project directories
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return 0, 0
	}
	
	indexFile := filepath.Join(mnemoDir, "claude-index.json")
	var index []map[string]interface{}
	
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			sessionPath := filepath.Join(basePath, entry.Name())
			sessionData, msgCount := parseJSONLSession(sessionPath)
			if sessionData != nil {
				index = append(index, sessionData)
				sessions++
				messages += msgCount
			}
		}
		
		// Check subdirectories for jsonl files
		if entry.IsDir() {
			subPath := filepath.Join(basePath, entry.Name())
			subEntries, _ := os.ReadDir(subPath)
			for _, subEntry := range subEntries {
				if strings.HasSuffix(subEntry.Name(), ".jsonl") {
					sessionPath := filepath.Join(subPath, subEntry.Name())
					sessionData, msgCount := parseJSONLSession(sessionPath)
					if sessionData != nil {
						index = append(index, sessionData)
						sessions++
						messages += msgCount
					}
				}
			}
		}
	}
	
	// Save index
	indexData, _ := json.MarshalIndent(index, "", "  ")
	os.WriteFile(indexFile, indexData, 0644)
	
	return sessions, messages
}

func indexOpencode(basePath, mnemoDir string) (int, int) {
	// Similar structure to Claude Code
	return 0, 0 // Placeholder
}

func parseJSONLSession(path string) (map[string]interface{}, int) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0
	}
	defer file.Close()
	
	info, _ := file.Stat()
	if info.Size() == 0 {
		return nil, 0
	}
	
	// Read first few lines to extract metadata
	data, _ := io.ReadAll(file)
	lines := strings.Split(string(data), "\n")
	
	var firstUserMsg string
	var topics []string
	msgCount := 0
	
	for _, line := range lines {
		if line == "" {
			continue
		}
		
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		
		msgCount++
		
		// Extract user messages for topic detection
		if entry["type"] == "user" {
			if msg, ok := entry["message"].(map[string]interface{}); ok {
				if content, ok := msg["content"].(string); ok && firstUserMsg == "" {
					firstUserMsg = truncate(content, 200)
				}
			}
		}
	}
	
	// Extract project name from path
	projectName := extractProjectName(path)
	
	return map[string]interface{}{
		"path":       path,
		"project":    projectName,
		"messages":   msgCount,
		"firstQuery": firstUserMsg,
		"topics":     topics,
		"indexed":    time.Now().Format(time.RFC3339),
		"size":       info.Size(),
	}, msgCount
}

func extractProjectName(path string) string {
	// Extract from path like: -Users-raghu-Projects-PILAN-INTELLIGENCE-PRISM
	parts := strings.Split(filepath.Dir(path), string(os.PathSeparator))
	for _, part := range parts {
		if strings.HasPrefix(part, "-Users-") {
			// Convert back: -Users-raghu-Projects-Foo -> Foo
			segments := strings.Split(part, "-")
			if len(segments) > 3 {
				return strings.Join(segments[4:], "-")
			}
		}
	}
	return filepath.Base(filepath.Dir(path))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func init() {
	indexCmd.Flags().BoolVarP(&indexAll, "all", "a", true, "Index all detected tools")
	indexCmd.Flags().StringVarP(&indexTool, "tool", "t", "", "Index specific tool only")
	rootCmd.AddCommand(indexCmd)
}
