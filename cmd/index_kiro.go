// index_kiro.go indexes Kiro IDE sessions stored as JSON files.
// Sessions live under ~/Library/Application Support/Kiro/workspace-sessions/.
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

// indexKiro walks the Kiro workspace-sessions directory and indexes each
// session's JSON file. Returns total (sessions, messages) indexed.
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
			if info, err := os.Stat(sessionFile); err == nil && skipOldFile(info) {
				continue
			}
			fileSessionID := strings.TrimSuffix(filepath.Base(sessionFile), ".json")
			if info, err := os.Stat(sessionFile); err == nil && isSessionUnchanged(fileSessionID, info.ModTime()) {
				continue
			}

			s, m := indexKiroSession(sessionFile)
			totalSessions += s
			totalMessages += m
		}
	}

	return totalSessions, totalMessages
}

// indexKiroSession parses a single Kiro workspace session JSON file and
// inserts all messages atomically within a transaction.
func indexKiroSession(sessionPath string) (int, int) {
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return 0, 0
	}

	var session struct {
		SessionID     string `json:"sessionId"`
		Title         string `json:"title"`
		WorkspacePath string `json:"workspacePath"`
		SelectedModel string `json:"selectedModel"`
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

	// Extract model and infer provider
	sessionModel := session.SelectedModel
	sessionProvider := ""
	if sessionModel != "" {
		sessionProvider = inferProviderFromModel(sessionModel)
	}

	if session.SessionID == "" {
		return 0, 0
	}

	// Use file modification time as fallback timestamp (more accurate than time.Now())
	fileModTime := time.Now()
	if info, err := os.Stat(sessionPath); err == nil {
		fileModTime = info.ModTime()
	}

	tx, err := db.BeginTx()
	if err != nil {
		indexErrors++
		return 0, 0
	}
	defer func() { _ = tx.Rollback() }()

	if err := db.TxDeleteSessionMessages(tx, session.SessionID); err != nil {
		indexErrors++
		return 0, 0
	}

	var firstUserMsg string
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

		if role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(content, 200)
		}

		err := db.TxInsertMessage(tx, db.Message{
			SessionID: session.SessionID,
			Role:      role,
			Content:   content,
			Project:   projectName,
			Tool:      "kiro",
			Timestamp: fileModTime,
			Model:     sessionModel,
			Provider:  sessionProvider,
		})
		if err != nil {
			indexErrors++
			continue
		}
		messagesIndexed++
	}

	if messagesIndexed > 0 {
		sessionQuery := firstUserMsg
		if sessionQuery == "" {
			sessionQuery = session.Title
		}
		if err := db.TxInsertSession(tx, db.Session{
			ID:           session.SessionID,
			Project:      projectName,
			FirstQuery:   sessionQuery,
			MessageCount: messagesIndexed,
			Tool:         "kiro",
			FilePath:     sessionPath,
			Model:        sessionModel,
			Provider:     sessionProvider,
			StartTime:    fileModTime,
			EndTime:      fileModTime,
		}); err != nil {
			indexErrors++
			return 0, messagesIndexed
		}
		if err := tx.Commit(); err != nil {
			indexErrors++
			return 0, messagesIndexed
		}
		return 1, messagesIndexed
	}

	return 0, 0
}
