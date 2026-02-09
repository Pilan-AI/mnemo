// index_gemini.go indexes Gemini CLI sessions stored as JSON files.
// Supports both old format (flat message arrays) and new format (structured
// with metadata, exchanges, and token usage). Sessions live under ~/.gemini/sessions/.
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

// indexGeminiCLI walks the Gemini sessions directory and indexes each JSON file,
// auto-detecting old vs new format. Returns total (sessions, messages) indexed.
func indexGeminiCLI(sessionsPath string) (int, int) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, 0
	}
	totalSessions := 0
	totalMessages := 0

	// Old format: ~/.gemini/sessions/*.json
	if pathExists(sessionsPath) {
		sessionFiles, err := os.ReadDir(sessionsPath)
		if err == nil {
			for _, sessionFile := range sessionFiles {
				if sessionFile.IsDir() || !strings.HasSuffix(sessionFile.Name(), ".json") {
					continue
				}
				if info, err := sessionFile.Info(); err == nil && skipOldFile(info) {
					continue
				}
				sessionFilePath := filepath.Join(sessionsPath, sessionFile.Name())
				s, m := indexGeminiSessionOld(sessionFilePath)
				totalSessions += s
				totalMessages += m
			}
		}
	}

	// New format: ~/.gemini/tmp/{projectHash}/chats/session-*.json
	tmpPath := filepath.Join(home, ".gemini", "tmp")
	if pathExists(tmpPath) {
		projectDirs, err := os.ReadDir(tmpPath)
		if err == nil {
			for _, projectDir := range projectDirs {
				if !projectDir.IsDir() {
					continue
				}
				chatsPath := filepath.Join(tmpPath, projectDir.Name(), "chats")
				if !pathExists(chatsPath) {
					continue
				}
				chatFiles, err := os.ReadDir(chatsPath)
				if err != nil {
					continue
				}
				for _, chatFile := range chatFiles {
					if chatFile.IsDir() || !strings.HasPrefix(chatFile.Name(), "session-") {
						continue
					}
					sessionFilePath := filepath.Join(chatsPath, chatFile.Name())
					s, m := indexGeminiSessionNew(sessionFilePath)
					totalSessions += s
					totalMessages += m
				}
			}
		}
	}

	return totalSessions, totalMessages
}

// indexGeminiSessionOld parses the legacy Gemini JSON format (flat message arrays).
func indexGeminiSessionOld(sessionPath string) (int, int) {
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

	// Incremental check using the actual session ID from JSON
	if info, err := os.Stat(sessionPath); err == nil && isSessionUnchanged(session.ID, info.ModTime()) {
		return 0, 0
	}

	projectName := filepath.Base(session.ProjectPath)
	if projectName == "" || projectName == "." {
		projectName = "gemini"
	}

	var firstUserMsg string
	msgCount := 0

	if session.ID == "" {
		return 0, 0
	}

	tx, err := db.BeginTx()
	if err != nil {
		indexErrors++
		return 0, 0
	}
	defer func() { _ = tx.Rollback() }()

	if err := db.TxDeleteSessionMessages(tx, session.ID); err != nil {
		indexErrors++
		return 0, 0
	}

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

		err := db.TxInsertMessage(tx, db.Message{
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
		if err := db.TxInsertSessionSimple(tx, session.ID, projectName, firstUserMsg, sessionPath, "gemini", msgCount); err != nil {
			indexErrors++
			return 0, msgCount
		}
		if err := tx.Commit(); err != nil {
			indexErrors++
			return 0, msgCount
		}
		return 1, msgCount
	}

	return 0, 0
}

// indexGeminiSessionNew parses the structured Gemini JSON format with metadata,
// exchanges, and token usage fields.
func indexGeminiSessionNew(sessionPath string) (int, int) {
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return 0, 0
	}

	var session struct {
		SessionID   string `json:"sessionId"`
		ProjectHash string `json:"projectHash"`
		StartTime   string `json:"startTime"`
		LastUpdated string `json:"lastUpdated"`
		Messages    []struct {
			ID        string `json:"id"`
			Timestamp string `json:"timestamp"`
			Type      string `json:"type"`
			Content   string `json:"content"`
			Model     string `json:"model"`
			Tokens    struct {
				Input    int `json:"input"`
				Output   int `json:"output"`
				Cached   int `json:"cached"`
				Thoughts int `json:"thoughts"`
				Total    int `json:"total"`
			} `json:"tokens"`
		} `json:"messages"`
	}

	if err := json.Unmarshal(data, &session); err != nil {
		return 0, 0
	}

	if len(session.Messages) == 0 {
		return 0, 0
	}

	// Incremental check using the actual session ID from JSON
	if info, err := os.Stat(sessionPath); err == nil && isSessionUnchanged(session.SessionID, info.ModTime()) {
		return 0, 0
	}

	hashSuffix := session.ProjectHash
	if len(hashSuffix) > 8 {
		hashSuffix = hashSuffix[:8]
	}
	projectName := "gemini-" + hashSuffix
	var firstUserMsg string
	var sessionModel string
	var totalInputTokens, totalOutputTokens int
	var sessionStartTime, sessionEndTime time.Time
	msgCount := 0

	if session.SessionID == "" {
		return 0, 0
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

	for _, msg := range session.Messages {
		if msg.Content == "" {
			continue
		}

		role := "user"
		if msg.Type == "gemini" {
			role = "assistant"
			if sessionModel == "" && msg.Model != "" {
				sessionModel = msg.Model
			}
		}

		if role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(msg.Content, 200)
		}

		timestamp := time.Now()
		if msg.Timestamp != "" {
			if parsed, err := time.Parse(time.RFC3339, msg.Timestamp); err == nil {
				timestamp = parsed
			}
		}

		if sessionStartTime.IsZero() || timestamp.Before(sessionStartTime) {
			sessionStartTime = timestamp
		}
		if timestamp.After(sessionEndTime) {
			sessionEndTime = timestamp
		}

		inputTokens := msg.Tokens.Input
		outputTokens := msg.Tokens.Output + msg.Tokens.Thoughts
		totalInputTokens += inputTokens
		totalOutputTokens += outputTokens

		err := db.TxInsertMessage(tx, db.Message{
			SessionID:       session.SessionID,
			Project:         projectName,
			Role:            role,
			Content:         msg.Content,
			Timestamp:       timestamp,
			Tool:            "gemini",
			Model:           msg.Model,
			Provider:        "google",
			InputTokens:     inputTokens,
			OutputTokens:    outputTokens,
			CacheReadTokens: msg.Tokens.Cached,
			ReasoningTokens: msg.Tokens.Thoughts,
			Date:            timestamp.Format("2006-01-02"),
		})
		if err != nil {
			indexErrors++
			continue
		}
		msgCount++
	}

	if msgCount > 0 {
		sessionDate := ""
		if !sessionStartTime.IsZero() {
			sessionDate = sessionStartTime.Format("2006-01-02")
		}
		if err := db.TxInsertSession(tx, db.Session{
			ID:                session.SessionID,
			Project:           projectName,
			FirstQuery:        firstUserMsg,
			MessageCount:      msgCount,
			Tool:              "gemini",
			FilePath:          sessionPath,
			Model:             sessionModel,
			Provider:          "google",
			TotalInputTokens:  totalInputTokens,
			TotalOutputTokens: totalOutputTokens,
			StartTime:         sessionStartTime,
			EndTime:           sessionEndTime,
			Date:              sessionDate,
		}); err != nil {
			indexErrors++
			return 0, msgCount
		}
		if err := tx.Commit(); err != nil {
			indexErrors++
			return 0, msgCount
		}
		return 1, msgCount
	}

	return 0, 0
}
