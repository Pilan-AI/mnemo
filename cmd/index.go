package cmd

import (
	"bufio"
	"database/sql"
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
	indexPath   string
	indexFormat string
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

		// Handle custom path indexing
		if indexPath != "" {
			if indexFormat == "" {
				fmt.Println("Error: --format is required when using --path")
				fmt.Println("Supported formats: claude, opencode, codex, amp, gemini, cline, kiro, antigravity, crush")
				os.Exit(1)
			}

			expandedPath := indexPath
			if strings.HasPrefix(expandedPath, "~") {
				expandedPath = filepath.Join(home, expandedPath[1:])
			}

			if !pathExists(expandedPath) {
				fmt.Printf("Error: Path does not exist: %s\n", expandedPath)
				os.Exit(1)
			}

			fmt.Printf("Indexing custom path: %s (format: %s)\n", expandedPath, indexFormat)
			sessions, messages := indexCustomPath(expandedPath, indexFormat)
			fmt.Printf("  ✓ Custom (%s): %d sessions, %d messages\n", indexFormat, sessions, messages)
			fmt.Printf("\nIndex saved to: ~/.mnemo/mnemo.db\n")
			return
		}

		totalSessions := 0
		totalMessages := 0

		// Index Claude Code - main directory (legacy path)
		claudePath := filepath.Join(home, ".claude", "projects")
		if pathExists(claudePath) {
			sessions, messages := indexClaudeCode(claudePath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Claude Code (projects): %d sessions, %d messages\n", sessions, messages)
		}

		// Index Claude Code - XDG path (Claude Code v1.0.30+)
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome == "" {
			xdgConfigHome = filepath.Join(home, ".config")
		}
		claudeXDGPath := filepath.Join(xdgConfigHome, "claude", "projects")
		if pathExists(claudeXDGPath) && claudeXDGPath != claudePath {
			sessions, messages := indexClaudeCode(claudeXDGPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Claude Code (XDG): %d sessions, %d messages\n", sessions, messages)
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
		opencodePath := filepath.Join(home, ".local", "share", "opencode")
		if pathExists(opencodePath) {
			sessions, messages := indexOpencode(opencodePath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Opencode: %d sessions, %d messages\n", sessions, messages)
		}

		// Index Gemini CLI
		geminiSessionsPath := filepath.Join(home, ".gemini", "sessions")
		if pathExists(geminiSessionsPath) {
			sessions, messages := indexGeminiCLI(geminiSessionsPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Gemini CLI: %d sessions, %d messages\n", sessions, messages)
		}

		// Index Cursor
		cursorPath := filepath.Join(home, "Library", "Application Support", "Cursor", "User", "workspaceStorage")
		if pathExists(cursorPath) {
			sessions, messages := indexCursor(cursorPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Cursor: %d sessions, %d messages\n", sessions, messages)
		}

		// Index VS Code extensions (Kilo Code, Cline, Roo Code) across ALL IDEs
		vscodeIDEs := []string{"Code", "Code - Insiders", "Cursor", "Windsurf", "VSCodium", "Antigravity", "Kiro", "Trae"}
		clineExtensions := []struct {
			ExtID    string
			ToolName string
		}{
			{"kilocode.kilo-code", "kilo-code"},
			{"saoudrizwan.claude-dev", "cline"},
			{"rooveterinaryinc.roo-cline", "roo-code"},
		}

		for _, ext := range clineExtensions {
			extSessions := 0
			extMessages := 0
			for _, ide := range vscodeIDEs {
				tasksPath := filepath.Join(home, "Library", "Application Support", ide, "User", "globalStorage", ext.ExtID, "tasks")
				if pathExists(tasksPath) {
					s, m := indexClineFamily(tasksPath, ext.ToolName)
					extSessions += s
					extMessages += m
				}
			}
			if extSessions > 0 {
				totalSessions += extSessions
				totalMessages += extMessages
				fmt.Printf("  ✓ %s: %d sessions, %d messages\n", ext.ToolName, extSessions, extMessages)
			}
		}

		// Index Crush CLI - main directory
		crushPath := filepath.Join(home, ".crush", "crush.db")
		crushSessions := 0
		crushMessages := 0
		if pathExists(crushPath) {
			s, m := indexCrush(crushPath)
			crushSessions += s
			crushMessages += m
		}

		crushSessions, crushMessages = scanCrushPerProject(home, crushSessions, crushMessages)
		if crushSessions > 0 {
			totalSessions += crushSessions
			totalMessages += crushMessages
			fmt.Printf("  ✓ Crush: %d sessions, %d messages\n", crushSessions, crushMessages)
		}

		// Index Antigravity (code_tracker JSONL files in ~/.gemini/antigravity/)
		antigravityPath := filepath.Join(home, ".gemini", "antigravity", "code_tracker", "active")
		if pathExists(antigravityPath) {
			sessions, messages := indexAntigravityCodeTracker(antigravityPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Antigravity: %d sessions, %d messages\n", sessions, messages)
		}

		// Index Kiro (stores JSON files in globalStorage/kiro.kiroagent/workspace-sessions/)
		kiroPath := filepath.Join(home, "Library", "Application Support", "Kiro", "User", "globalStorage", "kiro.kiroagent", "workspace-sessions")
		if pathExists(kiroPath) {
			sessions, messages := indexKiro(kiroPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Kiro: %d sessions, %d messages\n", sessions, messages)
		}

		// Index Amp CLI
		ampPath := filepath.Join(home, ".local", "share", "amp")
		if pathExists(ampPath) {
			sessions, messages := indexAmp(ampPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Amp: %d sessions, %d messages\n", sessions, messages)
		}

		// Index Codex CLI
		codexPath := filepath.Join(home, ".codex")
		if pathExists(codexPath) {
			sessions, messages := indexCodex(codexPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Codex: %d sessions, %d messages\n", sessions, messages)
		}

		fmt.Println()
		fmt.Printf("Total: %d sessions, %d messages indexed\n", totalSessions, totalMessages)
		if indexErrors > 0 {
			fmt.Printf("  (Skipped %d messages due to errors)\n", indexErrors)
		}
		fmt.Printf("Index saved to: ~/.mnemo/mnemo.db\n")

		if err := populateProjectsFromSessions(); err != nil {
			fmt.Printf("  (Warning: could not populate projects: %v)\n", err)
		} else {
			if err := db.ClassifyProjects(); err == nil {
				active, inactive, _ := db.GetProjectsForOnboarding()
				if len(active)+len(inactive) > 0 {
					fmt.Printf("  → %d active projects, %d inactive projects discovered\n", len(active), len(inactive))
				}
			}
		}

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

func indexGeminiCLI(sessionsPath string) (int, int) {
	sessionFiles, err := os.ReadDir(sessionsPath)
	if err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, sessionFile := range sessionFiles {
		if sessionFile.IsDir() || !strings.HasSuffix(sessionFile.Name(), ".json") {
			continue
		}

		sessionFilePath := filepath.Join(sessionsPath, sessionFile.Name())
		s, m := indexGeminiSession(sessionFilePath)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

func indexGeminiSession(sessionPath string) (int, int) {
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return 0, 0
	}

	var session struct {
		ID          string `json:"id"`
		ProjectPath string `json:"projectPath"`
		Messages    []struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			Timestamp string `json:"timestamp"`
		} `json:"messages"`
		CreatedAt    string `json:"createdAt"`
		LastActivity string `json:"lastActivity"`
	}

	if err := json.Unmarshal(data, &session); err != nil {
		return 0, 0
	}

	if len(session.Messages) == 0 {
		return 0, 0
	}

	projectName := filepath.Base(session.ProjectPath)
	if projectName == "" || projectName == "." {
		projectName = "gemini"
	}

	var firstUserMsg string
	msgCount := 0

	for _, msg := range session.Messages {
		if msg.Content == "" {
			continue
		}

		if msg.Role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(msg.Content, 200)
		}

		timestamp := time.Now()
		if msg.Timestamp != "" {
			if parsed, err := time.Parse(time.RFC3339, msg.Timestamp); err == nil {
				timestamp = parsed
			}
		}

		err := db.InsertMessage(db.Message{
			SessionID: session.ID,
			Project:   projectName,
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: timestamp,
			Tool:      "gemini",
		})
		if err != nil {
			indexErrors++
			continue
		}
		msgCount++
	}

	if msgCount > 0 {
		_ = db.InsertSessionSimple(session.ID, projectName, firstUserMsg, sessionPath, "gemini", msgCount)
		return 1, msgCount
	}

	return 0, 0
}

func indexCursor(workspaceStoragePath string) (int, int) {
	workspaceDirs, err := os.ReadDir(workspaceStoragePath)
	if err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, wsDir := range workspaceDirs {
		if !wsDir.IsDir() {
			continue
		}

		dbPath := filepath.Join(workspaceStoragePath, wsDir.Name(), "state.vscdb")
		if !pathExists(dbPath) {
			continue
		}

		s, m := indexCursorWorkspace(dbPath, wsDir.Name())
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

func indexCursorWorkspace(dbPath, workspaceID string) (int, int) {
	sqliteDB, err := db.OpenReadOnlySQLite(dbPath)
	if err != nil {
		return 0, 0
	}
	defer sqliteDB.Close()

	var chatDataJSON string
	row := sqliteDB.QueryRow("SELECT value FROM ItemTable WHERE key='workbench.panel.aichat.view.aichat.chatdata'")
	if err := row.Scan(&chatDataJSON); err != nil {
		return 0, 0
	}

	if chatDataJSON == "" {
		return 0, 0
	}

	var chatData struct {
		Tabs []struct {
			TabID     string `json:"tabId"`
			ChatTitle string `json:"chatTitle"`
			Bubbles   []struct {
				Type     string `json:"type"`
				ID       string `json:"id"`
				RawText  string `json:"rawText"`
				Text     string `json:"text"`
				InitText string `json:"initText"`
				RichText string `json:"richText"`
			} `json:"bubbles"`
		} `json:"tabs"`
	}

	if err := json.Unmarshal([]byte(chatDataJSON), &chatData); err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, tab := range chatData.Tabs {
		if len(tab.Bubbles) == 0 {
			continue
		}

		sessionID := tab.TabID
		if sessionID == "" {
			continue
		}

		projectName := tab.ChatTitle
		if projectName == "" {
			projectName = "cursor"
		}

		var firstUserMsg string
		msgCount := 0

		for _, bubble := range tab.Bubbles {
			var role, content string

			if bubble.Type == "user" {
				role = "user"
				content = extractCursorUserContent(bubble.InitText, bubble.RichText, bubble.RawText)
			} else if bubble.Type == "ai" {
				role = "assistant"
				content = bubble.Text
				if content == "" {
					content = bubble.RawText
				}
			} else {
				continue
			}

			if content == "" {
				continue
			}

			if role == "user" && firstUserMsg == "" {
				firstUserMsg = truncate(content, 200)
			}

			err := db.InsertMessage(db.Message{
				SessionID: sessionID,
				Project:   projectName,
				Role:      role,
				Content:   content,
				Timestamp: time.Now(),
				Tool:      "cursor",
			})
			if err != nil {
				indexErrors++
				continue
			}
			msgCount++
		}

		if msgCount > 0 {
			_ = db.InsertSessionSimple(sessionID, projectName, firstUserMsg, dbPath, "cursor", msgCount)
			totalSessions++
			totalMessages += msgCount
		}
	}

	return totalSessions, totalMessages
}

func extractCursorUserContent(initText, richText, rawText string) string {
	if rawText != "" {
		return rawText
	}

	textToProcess := initText
	if textToProcess == "" {
		textToProcess = richText
	}
	if textToProcess == "" {
		return ""
	}

	var lexical struct {
		Root struct {
			Children []struct {
				Children []struct {
					Text string `json:"text"`
				} `json:"children"`
			} `json:"children"`
		} `json:"root"`
	}

	if err := json.Unmarshal([]byte(textToProcess), &lexical); err != nil {
		return ""
	}

	var parts []string
	for _, paragraph := range lexical.Root.Children {
		for _, child := range paragraph.Children {
			if child.Text != "" {
				parts = append(parts, child.Text)
			}
		}
	}

	return strings.Join(parts, " ")
}

func indexClineFamily(tasksPath, toolName string) (int, int) {
	taskDirs, err := os.ReadDir(tasksPath)
	if err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, taskDir := range taskDirs {
		if !taskDir.IsDir() {
			continue
		}

		taskPath := filepath.Join(tasksPath, taskDir.Name())
		uiMessagesPath := filepath.Join(taskPath, "ui_messages.json")

		if !pathExists(uiMessagesPath) {
			continue
		}

		s, m := indexClineTask(uiMessagesPath, taskDir.Name(), toolName)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

func indexClineTask(uiMessagesPath, taskID, toolName string) (int, int) {
	data, err := os.ReadFile(uiMessagesPath)
	if err != nil {
		return 0, 0
	}

	var uiMessages []struct {
		Ts   int64  `json:"ts"`
		Type string `json:"type"`
		Say  string `json:"say"`
		Ask  string `json:"ask"`
		Text string `json:"text"`
	}

	if err := json.Unmarshal(data, &uiMessages); err != nil {
		return 0, 0
	}

	if len(uiMessages) == 0 {
		return 0, 0
	}

	projectName := toolName
	var firstUserMsg string
	var totalInputTokens, totalOutputTokens, totalCacheRead, totalCacheWrite int
	var totalCost float64
	var sessionProvider string
	msgCount := 0

	for _, msg := range uiMessages {
		if msg.Say == "api_req_started" && msg.Text != "" {
			var apiReq struct {
				TokensIn          int     `json:"tokensIn"`
				TokensOut         int     `json:"tokensOut"`
				CacheWrites       int     `json:"cacheWrites"`
				CacheReads        int     `json:"cacheReads"`
				Cost              float64 `json:"cost"`
				InferenceProvider string  `json:"inferenceProvider"`
			}
			if err := json.Unmarshal([]byte(msg.Text), &apiReq); err == nil {
				totalInputTokens += apiReq.TokensIn
				totalOutputTokens += apiReq.TokensOut
				totalCacheRead += apiReq.CacheReads
				totalCacheWrite += apiReq.CacheWrites
				totalCost += apiReq.Cost
				if sessionProvider == "" && apiReq.InferenceProvider != "" {
					sessionProvider = apiReq.InferenceProvider
				}
			}
			continue
		}

		if msg.Text == "" {
			continue
		}

		if msg.Say == "api_req_finished" || msg.Say == "checkpoint_saved" {
			continue
		}

		var role string
		if msg.Type == "say" && (msg.Say == "text" || msg.Say == "user_feedback") {
			role = "user"
		} else if msg.Type == "say" && msg.Say == "" {
			role = "assistant"
		} else {
			continue
		}

		if role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(msg.Text, 200)
		}

		timestamp := time.UnixMilli(msg.Ts)

		err := db.InsertMessage(db.Message{
			SessionID: taskID,
			Project:   projectName,
			Role:      role,
			Content:   msg.Text,
			Timestamp: timestamp,
			Tool:      toolName,
			Provider:  sessionProvider,
		})
		if err != nil {
			indexErrors++
			continue
		}
		msgCount++
	}

	if msgCount > 0 {
		_ = db.InsertSession(db.Session{
			ID:                taskID,
			Project:           projectName,
			FirstQuery:        firstUserMsg,
			MessageCount:      msgCount,
			Tool:              toolName,
			FilePath:          uiMessagesPath,
			Provider:          sessionProvider,
			TotalInputTokens:  totalInputTokens,
			TotalOutputTokens: totalOutputTokens,
			TotalCacheRead:    totalCacheRead,
			TotalCacheWrite:   totalCacheWrite,
			TotalCostUSD:      totalCost,
		})
		return 1, msgCount
	}

	return 0, 0
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

	var sessionCwd, sessionGitBranch, sessionVersion string
	var sessionStartTime, sessionEndTime time.Time

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

		uuid, _ := entry["uuid"].(string)
		parentUuid, _ := entry["parentUuid"].(string)
		cwd, _ := entry["cwd"].(string)
		gitBranch, _ := entry["gitBranch"].(string)
		version, _ := entry["version"].(string)

		timestamp := time.Now()
		if tsStr, ok := entry["timestamp"].(string); ok && tsStr != "" {
			if parsed, err := time.Parse(time.RFC3339, tsStr); err == nil {
				timestamp = parsed
			}
		}

		if sessionCwd == "" && cwd != "" {
			sessionCwd = cwd
		}
		if sessionGitBranch == "" && gitBranch != "" {
			sessionGitBranch = gitBranch
		}
		if sessionVersion == "" && version != "" {
			sessionVersion = version
		}
		if sessionStartTime.IsZero() {
			sessionStartTime = timestamp
		}
		sessionEndTime = timestamp

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

		err := db.InsertMessage(db.Message{
			SessionID:        sessionID,
			Project:          projectName,
			Role:             role,
			Content:          content,
			Timestamp:        timestamp,
			Tool:             tool,
			MessageUUID:      uuid,
			ParentUUID:       parentUuid,
			WorkingDirectory: cwd,
		})
		if err != nil {
			indexErrors++
			continue
		}
		msgCount++
	}

	if msgCount > 0 {
		_ = db.InsertSession(db.Session{
			ID:               sessionID,
			Project:          projectName,
			FirstQuery:       firstUserMsg,
			MessageCount:     msgCount,
			Tool:             tool,
			FilePath:         path,
			CLIVersion:       sessionVersion,
			GitBranch:        sessionGitBranch,
			WorkingDirectory: sessionCwd,
			StartTime:        sessionStartTime,
			EndTime:          sessionEndTime,
		})
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

	projectName := ""
	var firstUserMsg string
	var sessionModel, sessionProvider string
	var sessionWorkingDir string
	var sessionStartTime, sessionEndTime time.Time
	var totalInputTokens, totalOutputTokens, totalCacheRead, totalCacheWrite int
	var totalCostUSD float64
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

		var model, provider string
		if modelMap, ok := msg["model"].(map[string]interface{}); ok {
			model, _ = modelMap["modelID"].(string)
			provider, _ = modelMap["providerID"].(string)
		}
		if model == "" {
			model, _ = msg["modelID"].(string)
		}
		if provider == "" {
			provider, _ = msg["providerID"].(string)
		}

		if sessionModel == "" && model != "" {
			sessionModel = model
		}
		if sessionProvider == "" && provider != "" {
			sessionProvider = provider
		}

		if sessionWorkingDir == "" {
			if pathMap, ok := msg["path"].(map[string]interface{}); ok {
				if cwd, ok := pathMap["cwd"].(string); ok && cwd != "" {
					sessionWorkingDir = cwd
					projectName = filepath.Base(cwd)
				} else if root, ok := pathMap["root"].(string); ok && root != "" {
					sessionWorkingDir = root
					projectName = filepath.Base(root)
				}
			}
		}

		var msgTimestamp time.Time
		if timeMap, ok := msg["time"].(map[string]interface{}); ok {
			if created, ok := timeMap["created"].(float64); ok {
				msgTimestamp = time.UnixMilli(int64(created))
			}
		}
		if msgTimestamp.IsZero() {
			msgTimestamp = time.Now()
		}

		var inputTokens, outputTokens, cacheRead, cacheWrite, reasoning int
		if tokensMap, ok := msg["tokens"].(map[string]interface{}); ok {
			if v, ok := tokensMap["input"].(float64); ok {
				inputTokens = int(v)
			}
			if v, ok := tokensMap["output"].(float64); ok {
				outputTokens = int(v)
			}
			if v, ok := tokensMap["reasoning"].(float64); ok {
				reasoning = int(v)
				outputTokens += reasoning
			}
			if cacheMap, ok := tokensMap["cache"].(map[string]interface{}); ok {
				if v, ok := cacheMap["read"].(float64); ok {
					cacheRead = int(v)
				}
				if v, ok := cacheMap["write"].(float64); ok {
					cacheWrite = int(v)
				}
			}
		}

		var costUSD float64
		if v, ok := msg["cost"].(float64); ok {
			costUSD = v
		}

		totalInputTokens += inputTokens
		totalOutputTokens += outputTokens
		totalCacheRead += cacheRead
		totalCacheWrite += cacheWrite
		totalCostUSD += costUSD

		if sessionStartTime.IsZero() || msgTimestamp.Before(sessionStartTime) {
			sessionStartTime = msgTimestamp
		}
		if msgTimestamp.After(sessionEndTime) {
			sessionEndTime = msgTimestamp
		}

		msgCount++

		if role == "user" && firstUserMsg == "" && !isSystemDirective(content) {
			firstUserMsg = truncate(sanitizeContent(content), 200)
		}

		_ = db.InsertMessage(db.Message{
			SessionID:        sessionID,
			Project:          projectName,
			Role:             role,
			Content:          content,
			Timestamp:        msgTimestamp,
			Tool:             "opencode",
			Model:            model,
			Provider:         provider,
			InputTokens:      inputTokens,
			OutputTokens:     outputTokens,
			CacheReadTokens:  cacheRead,
			CacheWriteTokens: cacheWrite,
			CostUSD:          costUSD,
		})
	}

	if msgCount > 0 {
		if projectName == "" {
			projectName = "opencode"
		}
		sessionPath := filepath.Join(storagePath, "session")
		_ = db.InsertSession(db.Session{
			ID:                sessionID,
			Project:           projectName,
			FirstQuery:        firstUserMsg,
			MessageCount:      msgCount,
			Tool:              "opencode",
			FilePath:          sessionPath,
			Model:             sessionModel,
			Provider:          sessionProvider,
			TotalInputTokens:  totalInputTokens,
			TotalOutputTokens: totalOutputTokens,
			TotalCacheRead:    totalCacheRead,
			TotalCacheWrite:   totalCacheWrite,
			TotalCostUSD:      totalCostUSD,
			StartTime:         sessionStartTime,
			EndTime:           sessionEndTime,
		})
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

// indexCrush indexes Crush CLI sessions from ~/.crush/crush.db
// Crush has the cleanest SQLite schema of all tools with proper sessions/messages tables
func indexCrush(dbPath string) (int, int) {
	crushDB, err := db.OpenReadOnlySQLite(dbPath)
	if err != nil {
		return 0, 0
	}
	defer crushDB.Close()

	rows, err := crushDB.Query(`
		SELECT s.id, s.title, m.role, m.parts, m.created_at, m.model, m.provider
		FROM sessions s
		JOIN messages m ON m.session_id = s.id
		ORDER BY s.id, m.created_at
	`)
	if err != nil {
		return 0, 0
	}
	defer rows.Close()

	sessionMsgCounts := make(map[string]int)
	sessionFirstMsg := make(map[string]string)
	sessionModels := make(map[string]string)
	sessionProviders := make(map[string]string)
	totalMessages := 0

	for rows.Next() {
		var sessionID, title, role, partsJSON string
		var model, provider sql.NullString
		var createdAt int64

		if err := rows.Scan(&sessionID, &title, &role, &partsJSON, &createdAt, &model, &provider); err != nil {
			continue
		}

		var parts []map[string]interface{}
		if err := json.Unmarshal([]byte(partsJSON), &parts); err != nil {
			continue
		}

		var content string
		for _, part := range parts {
			partType, _ := part["type"].(string)
			if partType == "text" {
				if data, ok := part["data"].(map[string]interface{}); ok {
					if text, ok := data["text"].(string); ok {
						content += text
					}
				}
			}
		}

		if content == "" {
			continue
		}

		projectName := title
		if projectName == "" {
			projectName = "crush"
		}

		modelStr := ""
		if model.Valid {
			modelStr = model.String
		}
		providerStr := ""
		if provider.Valid {
			providerStr = provider.String
		}

		if sessionModels[sessionID] == "" && modelStr != "" {
			sessionModels[sessionID] = modelStr
		}
		if sessionProviders[sessionID] == "" && providerStr != "" {
			sessionProviders[sessionID] = providerStr
		}

		timestamp := time.UnixMilli(createdAt)

		err := db.InsertMessage(db.Message{
			SessionID: sessionID,
			Project:   projectName,
			Role:      role,
			Content:   content,
			Timestamp: timestamp,
			Tool:      "crush",
			Model:     modelStr,
			Provider:  providerStr,
		})
		if err != nil {
			indexErrors++
			continue
		}

		sessionMsgCounts[sessionID]++
		if role == "user" && sessionFirstMsg[sessionID] == "" {
			sessionFirstMsg[sessionID] = truncate(content, 200)
		}
		totalMessages++
	}

	for sessionID, msgCount := range sessionMsgCounts {
		_ = db.InsertSession(db.Session{
			ID:           sessionID,
			Project:      "crush",
			FirstQuery:   sessionFirstMsg[sessionID],
			MessageCount: msgCount,
			Tool:         "crush",
			FilePath:     dbPath,
			Model:        sessionModels[sessionID],
			Provider:     sessionProviders[sessionID],
		})
	}

	return len(sessionMsgCounts), totalMessages
}

func indexKiro(workspaceSessionsPath string) (int, int) {
	workspaceDirs, err := os.ReadDir(workspaceSessionsPath)
	if err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, wsDir := range workspaceDirs {
		if !wsDir.IsDir() {
			continue
		}

		workspaceDir := filepath.Join(workspaceSessionsPath, wsDir.Name())
		sessionFiles, err := filepath.Glob(filepath.Join(workspaceDir, "*.json"))
		if err != nil {
			continue
		}

		for _, sessionFile := range sessionFiles {
			if filepath.Base(sessionFile) == "sessions.json" {
				continue
			}

			s, m := indexKiroSession(sessionFile)
			totalSessions += s
			totalMessages += m
		}
	}

	return totalSessions, totalMessages
}

func indexKiroSession(sessionPath string) (int, int) {
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return 0, 0
	}

	var session struct {
		SessionID     string `json:"sessionId"`
		Title         string `json:"title"`
		WorkspacePath string `json:"workspacePath"`
		History       []struct {
			Message struct {
				Role    string      `json:"role"`
				Content interface{} `json:"content"`
				ID      string      `json:"id"`
			} `json:"message"`
		} `json:"history"`
	}

	if err := json.Unmarshal(data, &session); err != nil {
		return 0, 0
	}

	if len(session.History) == 0 {
		return 0, 0
	}

	projectName := session.Title
	if projectName == "" {
		projectName = filepath.Base(session.WorkspacePath)
	}
	if projectName == "" {
		projectName = "kiro"
	}

	messagesIndexed := 0
	for _, h := range session.History {
		var content string
		switch c := h.Message.Content.(type) {
		case string:
			content = c
		case []interface{}:
			for _, item := range c {
				if m, ok := item.(map[string]interface{}); ok {
					if t, ok := m["type"].(string); ok && t == "text" {
						if text, ok := m["text"].(string); ok {
							content += text
						}
					}
				}
			}
		}

		if content == "" {
			continue
		}

		role := h.Message.Role
		if role != "user" && role != "assistant" {
			continue
		}

		err := db.InsertMessage(db.Message{
			SessionID: session.SessionID,
			Role:      role,
			Content:   content,
			Project:   projectName,
			Tool:      "kiro",
			Timestamp: time.Now(),
		})
		if err == nil {
			messagesIndexed++
		}
	}

	if messagesIndexed > 0 {
		_ = db.InsertSessionSimple(session.SessionID, projectName, session.Title, sessionPath, "kiro", messagesIndexed)
		return 1, messagesIndexed
	}

	return 0, 0
}

func indexAntigravityCodeTracker(codeTrackerPath string) (int, int) {
	jsonlFiles, err := filepath.Glob(filepath.Join(codeTrackerPath, "*", "*.jsonl"))
	if err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, jsonlPath := range jsonlFiles {
		s, m := indexAntigravitySession(jsonlPath)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

func indexAntigravitySession(jsonlPath string) (int, int) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	sessionID := filepath.Base(jsonlPath)
	sessionID = strings.TrimSuffix(sessionID, ".jsonl")

	parentDir := filepath.Dir(jsonlPath)
	projectName := filepath.Base(parentDir)
	if projectName == "no_repo" {
		projectName = "antigravity"
	}

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	messagesIndexed := 0
	var firstQuery string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Antigravity JSONL files have a binary/protobuf-style prefix before JSON
		// Strip everything before the first '{' to get valid JSON
		jsonStart := strings.Index(line, "{")
		if jsonStart == -1 {
			continue
		}
		jsonLine := line[jsonStart:]

		var event struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			Content   string `json:"content"`
		}

		if err := json.Unmarshal([]byte(jsonLine), &event); err != nil {
			continue
		}

		if event.Type != "user" && event.Type != "assistant" {
			continue
		}

		if event.Content == "" {
			continue
		}

		role := event.Type
		timestamp := time.Now()
		if event.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, event.Timestamp); err == nil {
				timestamp = t
			}
		}

		if firstQuery == "" && role == "user" {
			if len(event.Content) > 100 {
				firstQuery = event.Content[:100] + "..."
			} else {
				firstQuery = event.Content
			}
		}

		err := db.InsertMessage(db.Message{
			SessionID: sessionID,
			Role:      role,
			Content:   event.Content,
			Project:   projectName,
			Tool:      "antigravity",
			Timestamp: timestamp,
		})
		if err == nil {
			messagesIndexed++
		}
	}

	if messagesIndexed > 0 {
		_ = db.InsertSessionSimple(sessionID, projectName, firstQuery, jsonlPath, "antigravity", messagesIndexed)
		return 1, messagesIndexed
	}

	return 0, 0
}

// indexVSCodeAIChat indexes VS Code-based IDEs that use state.vscdb
// These share the same format as Cursor: ItemTable with aichat.chatdata JSON blob
func indexVSCodeAIChat(workspaceStoragePath, toolName string) (int, int) {
	workspaceDirs, err := os.ReadDir(workspaceStoragePath)
	if err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, wsDir := range workspaceDirs {
		if !wsDir.IsDir() {
			continue
		}

		dbPath := filepath.Join(workspaceStoragePath, wsDir.Name(), "state.vscdb")
		if !pathExists(dbPath) {
			continue
		}

		s, m := indexVSCodeWorkspace(dbPath, wsDir.Name(), toolName)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

func indexVSCodeWorkspace(dbPath, workspaceID, toolName string) (int, int) {
	sqliteDB, err := db.OpenReadOnlySQLite(dbPath)
	if err != nil {
		return 0, 0
	}
	defer sqliteDB.Close()

	var chatDataJSON string
	row := sqliteDB.QueryRow("SELECT value FROM ItemTable WHERE key='workbench.panel.aichat.view.aichat.chatdata'")
	if err := row.Scan(&chatDataJSON); err != nil {
		return 0, 0
	}

	if chatDataJSON == "" {
		return 0, 0
	}

	var chatData struct {
		Tabs []struct {
			TabID     string `json:"tabId"`
			ChatTitle string `json:"chatTitle"`
			Bubbles   []struct {
				Type     string `json:"type"`
				ID       string `json:"id"`
				RawText  string `json:"rawText"`
				Text     string `json:"text"`
				InitText string `json:"initText"`
				RichText string `json:"richText"`
			} `json:"bubbles"`
		} `json:"tabs"`
	}

	if err := json.Unmarshal([]byte(chatDataJSON), &chatData); err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, tab := range chatData.Tabs {
		if len(tab.Bubbles) == 0 {
			continue
		}

		sessionID := tab.TabID
		if sessionID == "" {
			continue
		}

		projectName := tab.ChatTitle
		if projectName == "" {
			projectName = toolName
		}

		var firstUserMsg string
		msgCount := 0

		for _, bubble := range tab.Bubbles {
			var role, content string

			if bubble.Type == "user" {
				role = "user"
				content = extractCursorUserContent(bubble.InitText, bubble.RichText, bubble.RawText)
			} else if bubble.Type == "ai" {
				role = "assistant"
				content = bubble.Text
				if content == "" {
					content = bubble.RawText
				}
			} else {
				continue
			}

			if content == "" {
				continue
			}

			if role == "user" && firstUserMsg == "" {
				firstUserMsg = truncate(content, 200)
			}

			err := db.InsertMessage(db.Message{
				SessionID: sessionID,
				Project:   projectName,
				Role:      role,
				Content:   content,
				Timestamp: time.Now(),
				Tool:      toolName,
			})
			if err != nil {
				indexErrors++
				continue
			}
			msgCount++
		}

		if msgCount > 0 {
			_ = db.InsertSessionSimple(sessionID, projectName, firstUserMsg, dbPath, toolName, msgCount)
			totalSessions++
			totalMessages += msgCount
		}
	}

	return totalSessions, totalMessages
}

// indexAmp indexes Amp CLI sessions from ~/.local/share/amp/
// Format: history.jsonl for prompts, threads/ directory for full conversations
func indexAmp(basePath string) (int, int) {
	threadsPath := filepath.Join(basePath, "threads")
	if !pathExists(threadsPath) {
		return 0, 0
	}

	threadFiles, err := os.ReadDir(threadsPath)
	if err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, threadFile := range threadFiles {
		if threadFile.IsDir() || !strings.HasSuffix(threadFile.Name(), ".json") {
			continue
		}

		threadPath := filepath.Join(threadsPath, threadFile.Name())
		s, m := indexAmpThread(threadPath)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

func indexAmpThread(threadPath string) (int, int) {
	data, err := os.ReadFile(threadPath)
	if err != nil {
		return 0, 0
	}

	var thread struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Messages []struct {
			Role      string `json:"role"`
			MessageID int    `json:"messageId"`
			Content   []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Provider string `json:"provider"`
			} `json:"content"`
		} `json:"messages"`
		Created int64 `json:"created"`
	}

	if err := json.Unmarshal(data, &thread); err != nil {
		return 0, 0
	}

	if len(thread.Messages) == 0 {
		return 0, 0
	}

	sessionID := thread.ID
	if sessionID == "" {
		sessionID = strings.TrimSuffix(filepath.Base(threadPath), ".json")
	}

	projectName := "amp"
	var sessionProvider string

	var firstUserMsg string
	msgCount := 0

	for _, msg := range thread.Messages {
		var content string
		var msgProvider string
		for _, c := range msg.Content {
			if c.Type == "text" && c.Text != "" {
				content += c.Text
			}
			if c.Provider != "" && msgProvider == "" {
				msgProvider = c.Provider
			}
		}

		if sessionProvider == "" && msgProvider != "" {
			sessionProvider = msgProvider
		}

		if content == "" {
			continue
		}

		if msg.Role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(content, 200)
		}

		timestamp := time.UnixMilli(thread.Created)

		err := db.InsertMessage(db.Message{
			SessionID: sessionID,
			Project:   projectName,
			Role:      msg.Role,
			Content:   content,
			Timestamp: timestamp,
			Tool:      "amp",
			Provider:  msgProvider,
		})
		if err != nil {
			indexErrors++
			continue
		}
		msgCount++
	}

	if msgCount > 0 {
		_ = db.InsertSession(db.Session{
			ID:           sessionID,
			Project:      projectName,
			FirstQuery:   firstUserMsg,
			MessageCount: msgCount,
			Tool:         "amp",
			FilePath:     threadPath,
			Provider:     sessionProvider,
		})
		return 1, msgCount
	}

	return 0, 0
}

// indexCodex indexes OpenAI Codex CLI sessions from ~/.codex/
// Format: history.jsonl for prompts, sessions/ and archived_sessions/ directories with .jsonl files
func indexCodex(basePath string) (int, int) {
	totalSessions := 0
	totalMessages := 0

	sessionDirs := []string{
		filepath.Join(basePath, "sessions"),
		filepath.Join(basePath, "archived_sessions"),
	}

	foundSessions := false
	for _, sessionsPath := range sessionDirs {
		if !pathExists(sessionsPath) {
			continue
		}
		foundSessions = true

		_ = filepath.Walk(sessionsPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			if info.IsDir() {
				return nil
			}

			if strings.HasSuffix(info.Name(), ".jsonl") {
				s, m := indexCodexSessionJSONL(path)
				totalSessions += s
				totalMessages += m
			}

			return nil
		})
	}

	if !foundSessions {
		historyPath := filepath.Join(basePath, "history.jsonl")
		if pathExists(historyPath) {
			return indexCodexHistory(historyPath)
		}
		return 0, 0
	}

	return totalSessions, totalMessages
}

func indexCodexHistory(historyPath string) (int, int) {
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return 0, 0
	}

	lines := strings.Split(string(data), "\n")
	sessionMessages := make(map[string][]struct {
		role      string
		content   string
		timestamp time.Time
	})

	for _, line := range lines {
		if line == "" {
			continue
		}

		var entry struct {
			SessionID string `json:"session_id"`
			Ts        int64  `json:"ts"`
			Text      string `json:"text"`
		}

		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if entry.Text == "" {
			continue
		}

		sessionID := entry.SessionID
		if sessionID == "" {
			sessionID = "codex-default"
		}

		sessionMessages[sessionID] = append(sessionMessages[sessionID], struct {
			role      string
			content   string
			timestamp time.Time
		}{
			role:      "user",
			content:   entry.Text,
			timestamp: time.UnixMilli(entry.Ts),
		})
	}

	totalSessions := 0
	totalMessages := 0

	for sessionID, messages := range sessionMessages {
		var firstUserMsg string
		msgCount := 0

		for _, msg := range messages {
			if firstUserMsg == "" {
				firstUserMsg = truncate(msg.content, 200)
			}

			err := db.InsertMessage(db.Message{
				SessionID: sessionID,
				Project:   "codex",
				Role:      msg.role,
				Content:   msg.content,
				Timestamp: msg.timestamp,
				Tool:      "codex",
			})
			if err != nil {
				indexErrors++
				continue
			}
			msgCount++
		}

		if msgCount > 0 {
			_ = db.InsertSessionSimple(sessionID, "codex", firstUserMsg, historyPath, "codex", msgCount)
			totalSessions++
			totalMessages += msgCount
		}
	}

	return totalSessions, totalMessages
}

func indexCodexSessionJSONL(sessionPath string) (int, int) {
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return 0, 0
	}

	lines := strings.Split(string(data), "\n")
	var sessionID, cliVersion, cwd, sessionModel, sessionProvider string
	var currentModel string
	var messages []struct {
		role      string
		content   string
		timestamp time.Time
		model     string
	}

	for _, line := range lines {
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		entryType, _ := entry["type"].(string)
		timestampStr, _ := entry["timestamp"].(string)

		timestamp := time.Now()
		if timestampStr != "" {
			if parsed, err := time.Parse(time.RFC3339, timestampStr); err == nil {
				timestamp = parsed
			}
		}

		if entryType == "session_meta" {
			if payload, ok := entry["payload"].(map[string]interface{}); ok {
				sessionID, _ = payload["id"].(string)
				cliVersion, _ = payload["cli_version"].(string)
				cwd, _ = payload["cwd"].(string)
				sessionProvider, _ = payload["model_provider"].(string)
			}
		} else if entryType == "turn_context" {
			if payload, ok := entry["payload"].(map[string]interface{}); ok {
				currentModel, _ = payload["model"].(string)
				if sessionModel == "" && currentModel != "" {
					sessionModel = currentModel
				}
			}
		} else if entryType == "event_msg" {
			if payload, ok := entry["payload"].(map[string]interface{}); ok {
				msgType, _ := payload["type"].(string)
				message, _ := payload["message"].(string)

				if message == "" {
					continue
				}

				var role string
				if msgType == "user_message" {
					role = "user"
				} else if msgType == "agent_message" {
					role = "assistant"
				} else {
					continue
				}

				messages = append(messages, struct {
					role      string
					content   string
					timestamp time.Time
					model     string
				}{role: role, content: message, timestamp: timestamp, model: currentModel})
			}
		}
	}

	if len(messages) == 0 {
		return 0, 0
	}

	if sessionID == "" {
		sessionID = strings.TrimSuffix(filepath.Base(sessionPath), ".jsonl")
	}

	var firstUserMsg string
	msgCount := 0

	for _, msg := range messages {
		if msg.role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(msg.content, 200)
		}

		err := db.InsertMessage(db.Message{
			SessionID:        sessionID,
			Project:          "codex",
			Role:             msg.role,
			Content:          msg.content,
			Timestamp:        msg.timestamp,
			Tool:             "codex",
			Model:            msg.model,
			Provider:         sessionProvider,
			WorkingDirectory: cwd,
		})
		if err != nil {
			indexErrors++
			continue
		}
		msgCount++
	}

	if msgCount > 0 {
		_ = db.InsertSession(db.Session{
			ID:               sessionID,
			Project:          "codex",
			FirstQuery:       firstUserMsg,
			MessageCount:     msgCount,
			Tool:             "codex",
			FilePath:         sessionPath,
			Model:            sessionModel,
			Provider:         sessionProvider,
			CLIVersion:       cliVersion,
			WorkingDirectory: cwd,
		})
		return 1, msgCount
	}

	return 0, 0
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

		opencodePath := filepath.Join(home, ".local", "share", "opencode")
		if pathExists(opencodePath) {
			sessions, messages := indexOpencode(opencodePath)
			totalSessions += sessions
			totalMessages += messages
		}

		geminiSessionsPath := filepath.Join(home, ".gemini", "sessions")
		if pathExists(geminiSessionsPath) {
			sessions, messages := indexGeminiCLI(geminiSessionsPath)
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

func indexCustomPath(customPath, format string) (int, int) {
	switch strings.ToLower(format) {
	case "claude":
		return indexClaudeCode(customPath)
	case "opencode":
		return indexOpencode(customPath)
	case "codex":
		return indexCodex(customPath)
	case "amp":
		return indexAmp(customPath)
	case "gemini":
		return indexGeminiCLI(customPath)
	case "cline", "kilo", "roo":
		return indexClineFamily(customPath, format)
	case "kiro":
		return indexKiro(customPath)
	case "antigravity":
		return indexAntigravityCodeTracker(customPath)
	case "crush":
		return indexCrush(customPath)
	default:
		fmt.Printf("Unknown format: %s\n", format)
		fmt.Println("Supported formats: claude, opencode, codex, amp, gemini, cline, kilo, roo, kiro, antigravity, crush")
		return 0, 0
	}
}

func populateProjectsFromSessions() error {
	home, _ := os.UserHomeDir()

	rows, err := db.GetDB().Query(`
		SELECT DISTINCT working_directory, MAX(COALESCE(start_time, indexed_at)) as last_activity
		FROM sessions
		WHERE working_directory != '' AND working_directory IS NOT NULL
		GROUP BY working_directory
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var workDir string
		var lastActivity time.Time
		if err := rows.Scan(&workDir, &lastActivity); err != nil {
			continue
		}

		workDir = strings.TrimSpace(workDir)
		if workDir == "" {
			continue
		}

		if strings.HasPrefix(workDir, "~") {
			workDir = filepath.Join(home, workDir[1:])
		}

		if !filepath.IsAbs(workDir) {
			continue
		}

		if lastActivity.IsZero() {
			lastActivity = time.Now()
		}

		_ = db.UpsertProject(workDir, lastActivity)
	}

	return nil
}

func scanCrushPerProject(home string, sessions, messages int) (int, int) {
	enabledProjects, err := db.GetEnabledProjects()
	if err != nil || len(enabledProjects) == 0 {
		projectsPath := filepath.Join(home, "Projects")
		if pathExists(projectsPath) {
			_ = filepath.Walk(projectsPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.Name() == "crush.db" && strings.Contains(path, ".crush") {
					s, m := indexCrush(path)
					sessions += s
					messages += m
				}
				return nil
			})
		}
		return sessions, messages
	}

	scannedPaths := make(map[string]bool)
	for _, project := range enabledProjects {
		crushDB := filepath.Join(project.Path, ".crush", "crush.db")
		if scannedPaths[crushDB] {
			continue
		}
		scannedPaths[crushDB] = true

		if pathExists(crushDB) {
			s, m := indexCrush(crushDB)
			sessions += s
			messages += m
		}
	}

	return sessions, messages
}

func init() {
	indexCmd.Flags().BoolVarP(&indexAll, "all", "a", true, "Index all detected tools")
	indexCmd.Flags().StringVarP(&indexTool, "tool", "t", "", "Index specific tool only")
	indexCmd.Flags().BoolVarP(&indexForce, "force", "f", false, "Force re-index (clear existing)")
	indexCmd.Flags().StringVarP(&indexPath, "path", "p", "", "Custom path to index (requires --format)")
	indexCmd.Flags().StringVar(&indexFormat, "format", "", "Format for custom path: claude, opencode, codex, amp, gemini, cline, kiro, antigravity")
	rootCmd.AddCommand(indexCmd)
}
