package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

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
			Model:     sessionModel,
			Provider:  sessionProvider,
		})
		if err == nil {
			messagesIndexed++
		}
	}

	if messagesIndexed > 0 {
		_ = db.InsertSession(db.Session{
			ID:           session.SessionID,
			Project:      projectName,
			FirstQuery:   session.Title,
			MessageCount: messagesIndexed,
			Tool:         "kiro",
			FilePath:     sessionPath,
			Model:        sessionModel,
			Provider:     sessionProvider,
		})
		return 1, messagesIndexed
	}

	return 0, 0
}
