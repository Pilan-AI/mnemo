package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

// extractVSCodeUserContent extracts user content from VS Code-style chat bubbles
// (used by Windsurf and other VS Code-based IDEs)
func extractVSCodeUserContent(initText, richText, rawText string) string {
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
		if info, err := os.Stat(dbPath); err == nil && skipOldFile(info) {
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
				content = extractVSCodeUserContent(bubble.InitText, bubble.RichText, bubble.RawText)
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
