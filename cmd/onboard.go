package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
	"github.com/Pilan-AI/mnemo/internal/tui"
	"github.com/spf13/cobra"
)

type ProjectStats struct {
	Name        string
	Sessions    int
	Messages    int
	FirstSeen   time.Time
	LastSeen    time.Time
	TopTopics   []string
	Discoveries []string
}

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "First-time setup with stunning visual experience",
	Long:  "Discover your AI coding history with a beautiful terminal experience that feels like magic.",
	Run: func(cmd *cobra.Command, args []string) {
		// Run the beautiful TUI onboarding
		err := tui.RunOnboarding(func() (tui.Stats, []tui.Discovery) {
			return runIndexing()
		})

		if err != nil {
			// Fallback to simple output
			runSimpleOnboarding()
		}
	},
}

func runIndexing() (tui.Stats, []tui.Discovery) {
	home, _ := os.UserHomeDir()

	// Initialize database
	if err := db.InitDB(); err != nil {
		return tui.Stats{}, nil
	}
	defer db.CloseDB()

	projectStats := make(map[string]*ProjectStats)
	discoveries := make([]tui.Discovery, 0)

	totalSessions := 0
	totalMessages := 0

	// Index Claude Code locations
	claudePaths := []string{
		filepath.Join(home, ".claude", "projects"),
		filepath.Join(home, ".claude", "transcripts"),
		filepath.Join(home, ".claude-backup"),
	}

	for _, claudePath := range claudePaths {
		if pathExists(claudePath) {
			s, m, d := indexWithDiscoveriesNew(claudePath, "claude", projectStats)
			totalSessions += s
			totalMessages += m
			discoveries = append(discoveries, d...)
		}
	}

	// Index Opencode
	opencodePath := filepath.Join(home, ".opencode")
	if pathExists(opencodePath) {
		s, m, d := indexWithDiscoveriesNew(filepath.Join(opencodePath, "sessions"), "opencode", projectStats)
		totalSessions += s
		totalMessages += m
		discoveries = append(discoveries, d...)
	}

	// Calculate stats
	var earliest, latest time.Time
	for _, ps := range projectStats {
		if earliest.IsZero() || ps.FirstSeen.Before(earliest) {
			earliest = ps.FirstSeen
		}
		if latest.IsZero() || ps.LastSeen.After(latest) {
			latest = ps.LastSeen
		}
	}

	days := 1
	if !earliest.IsZero() && !latest.IsZero() {
		days = int(latest.Sub(earliest).Hours()/24) + 1
	}

	// Find top project
	var topProject string
	var topCount int
	for name, ps := range projectStats {
		if ps.Messages > topCount {
			topProject = name
			topCount = ps.Messages
		}
	}

	// Sort discoveries by message count (top 5)
	sort.Slice(discoveries, func(i, j int) bool {
		return discoveries[i].Messages > discoveries[j].Messages
	})
	if len(discoveries) > 5 {
		discoveries = discoveries[:5]
	}

	return tui.Stats{
		Sessions:   totalSessions,
		Messages:   totalMessages,
		Projects:   len(projectStats),
		Days:       days,
		TopProject: topProject,
		TopCount:   topCount,
	}, discoveries
}

func indexWithDiscoveriesNew(basePath, tool string, stats map[string]*ProjectStats) (int, int, []tui.Discovery) {
	sessions := 0
	messages := 0
	discoveries := make([]tui.Discovery, 0)

	filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".jsonl") {
			return nil
		}

		s, m, projectName, _ := indexSessionWithStatsNew(path, tool)
		sessions += s
		messages += m

		if projectName != "" {
			if _, exists := stats[projectName]; !exists {
				stats[projectName] = &ProjectStats{
					Name:      projectName,
					FirstSeen: info.ModTime(),
					LastSeen:  info.ModTime(),
				}
			}
			ps := stats[projectName]
			ps.Sessions++
			ps.Messages += m
			if info.ModTime().Before(ps.FirstSeen) {
				ps.FirstSeen = info.ModTime()
			}
			if info.ModTime().After(ps.LastSeen) {
				ps.LastSeen = info.ModTime()
			}

			// Generate discovery for significant projects
			if ps.Sessions == 1 && m > 30 {
				icon := getProjectIcon(projectName)
				discoveries = append(discoveries, tui.Discovery{
					Project:  projectName,
					Messages: m,
					Icon:     icon,
				})
			}
		}

		return nil
	})

	return sessions, messages, discoveries
}

func getProjectIcon(project string) string {
	projectLower := strings.ToLower(project)

	// Match project types to icons
	switch {
	case strings.Contains(projectLower, "intelligence") || strings.Contains(projectLower, "ai"):
		return "🧠"
	case strings.Contains(projectLower, "forge") || strings.Contains(projectLower, "build"):
		return "🔥"
	case strings.Contains(projectLower, "tui") || strings.Contains(projectLower, "ui"):
		return "🖥️"
	case strings.Contains(projectLower, "api") || strings.Contains(projectLower, "server"):
		return "🌐"
	case strings.Contains(projectLower, "cli") || strings.Contains(projectLower, "tool"):
		return "⚡"
	case strings.Contains(projectLower, "test"):
		return "🧪"
	case strings.Contains(projectLower, "doc"):
		return "📚"
	case strings.Contains(projectLower, "mnemo") || strings.Contains(projectLower, "memory"):
		return "💾"
	default:
		return "📁"
	}
}

func indexSessionWithStatsNew(path, tool string) (int, int, string, []string) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, "", nil
	}
	defer file.Close()

	data, _ := io.ReadAll(file)
	lines := strings.Split(string(data), "\n")

	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	projectName := extractProjectName(path)
	msgCount := 0
	topics := make([]string, 0)
	var firstUserMsg string

	for _, line := range lines {
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		entryType, _ := entry["type"].(string)
		if entryType != "user" && entryType != "assistant" {
			continue
		}

		var content string
		var role string

		if msg, ok := entry["message"].(map[string]interface{}); ok {
			role, _ = msg["role"].(string)
			switch c := msg["content"].(type) {
			case string:
				content = c
			case []interface{}:
				for _, item := range c {
					if block, ok := item.(map[string]interface{}); ok {
						if text, ok := block["text"].(string); ok {
							content += text + " "
						}
					}
				}
			}
		} else {
			role = entryType
			if c, ok := entry["content"].(string); ok {
				content = c
			}
		}

		if content == "" {
			continue
		}

		if role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(content, 200)
		}

		if role == "user" && len(content) > 50 {
			topics = append(topics, extractTopics(content)...)
		}

		err := db.InsertMessage(db.Message{
			SessionID: sessionID,
			Project:   projectName,
			Role:      role,
			Content:   content,
			Timestamp: time.Now(),
			Tool:      tool,
		})
		if err == nil {
			msgCount++
		}
	}

	if msgCount > 0 {
		db.InsertSession(sessionID, projectName, firstUserMsg, path, tool, msgCount)
		return 1, msgCount, projectName, topics
	}

	return 0, 0, "", nil
}

func extractTopics(content string) []string {
	topics := make([]string, 0)
	keywords := []string{
		"authentication", "database", "API", "frontend", "backend",
		"bug", "feature", "refactor", "test", "deploy", "docker",
		"kubernetes", "AWS", "performance", "security", "UI", "UX",
	}

	contentLower := strings.ToLower(content)
	for _, keyword := range keywords {
		if strings.Contains(contentLower, strings.ToLower(keyword)) {
			topics = append(topics, keyword)
		}
	}
	return topics
}

// Fallback simple onboarding for non-TTY environments
func runSimpleOnboarding() {
	home, _ := os.UserHomeDir()

	println("\n  MNEMO - Your AI coding memory\n")
	println("  Scanning your AI coding history...\n")

	if err := db.InitDB(); err != nil {
		println("  Error:", err.Error())
		return
	}
	defer db.CloseDB()

	totalSessions := 0
	totalMessages := 0

	claudePaths := []string{
		filepath.Join(home, ".claude", "projects"),
		filepath.Join(home, ".claude", "transcripts"),
		filepath.Join(home, ".claude-backup"),
	}

	for _, claudePath := range claudePaths {
		if pathExists(claudePath) {
			s, m := indexClaudeCode(claudePath)
			totalSessions += s
			totalMessages += m
		}
	}

	println("  ✓ Indexed", totalSessions, "sessions with", totalMessages, "messages")
	println("\n  Your memory is ready.")
	println("  Try: mnemo search \"<anything>\"\n")
}

func init() {
	rootCmd.AddCommand(onboardCmd)
}
