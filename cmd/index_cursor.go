package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

func indexCursorGlobalStorage(dbPath string) (int, int) {
	sqliteDB, err := db.OpenReadOnlySQLite(dbPath)
	if err != nil {
		return 0, 0
	}
	defer sqliteDB.Close()

	rows, err := sqliteDB.Query("SELECT key, value FROM cursorDiskKV WHERE key LIKE 'composerData:%'")
	if err != nil {
		return 0, 0
	}
	defer rows.Close()

	type ComposerData struct {
		Version      int    `json:"_v"`
		ComposerID   string `json:"composerId"`
		Name         string `json:"name"`
		CreatedAt    int64  `json:"createdAt"`
		LastUpdated  int64  `json:"lastUpdatedAt"`
		UnifiedMode  string `json:"unifiedMode"`
		Conversation []struct {
			BubbleID string `json:"bubbleId"`
			Type     int    `json:"type"`
		} `json:"fullConversationHeadersOnly"`
	}

	totalSessions := 0
	totalMessages := 0

	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}

		var composer ComposerData
		if err := json.Unmarshal(value, &composer); err != nil {
			continue
		}

		if len(composer.Conversation) == 0 {
			continue
		}

		sessionID := composer.ComposerID
		projectName := composer.Name
		if projectName == "" {
			projectName = "cursor-" + sessionID[:8]
		}

		var firstUserMsg string
		var sessionStartTime, sessionEndTime time.Time
		var totalInputTokens, totalOutputTokens int
		msgCount := 0

		if composer.CreatedAt > 0 {
			sessionStartTime = time.UnixMilli(composer.CreatedAt)
		}
		if composer.LastUpdated > 0 {
			sessionEndTime = time.UnixMilli(composer.LastUpdated)
		}

		for _, bubble := range composer.Conversation {
			bubbleKey := fmt.Sprintf("bubbleId:%s:%s", sessionID, bubble.BubbleID)
			var bubbleValue []byte
			row := sqliteDB.QueryRow("SELECT value FROM cursorDiskKV WHERE key=?", bubbleKey)
			if err := row.Scan(&bubbleValue); err != nil {
				continue
			}

			var bubbleData struct {
				Type       int    `json:"type"`
				Text       string `json:"text"`
				TokenCount struct {
					InputTokens  int `json:"inputTokens"`
					OutputTokens int `json:"outputTokens"`
				} `json:"tokenCount"`
			}

			if err := json.Unmarshal(bubbleValue, &bubbleData); err != nil {
				continue
			}

			if bubbleData.Text == "" {
				continue
			}

			role := "user"
			if bubbleData.Type == 2 {
				role = "assistant"
			}

			if role == "user" && firstUserMsg == "" {
				firstUserMsg = truncate(bubbleData.Text, 200)
			}

			inputTokens := bubbleData.TokenCount.InputTokens
			outputTokens := bubbleData.TokenCount.OutputTokens
			totalInputTokens += inputTokens
			totalOutputTokens += outputTokens

			timestamp := sessionStartTime
			if timestamp.IsZero() {
				timestamp = time.Now()
			}

			err := db.InsertMessage(db.Message{
				SessionID:    sessionID,
				Project:      projectName,
				Role:         role,
				Content:      bubbleData.Text,
				Timestamp:    timestamp,
				Tool:         "cursor",
				Provider:     "cursor",
				InputTokens:  inputTokens,
				OutputTokens: outputTokens,
				Date:         timestamp.Format("2006-01-02"),
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
			_ = db.InsertSession(db.Session{
				ID:                sessionID,
				Project:           projectName,
				FirstQuery:        firstUserMsg,
				MessageCount:      msgCount,
				Tool:              "cursor",
				FilePath:          dbPath,
				Provider:          "cursor",
				TotalInputTokens:  totalInputTokens,
				TotalOutputTokens: totalOutputTokens,
				StartTime:         sessionStartTime,
				EndTime:           sessionEndTime,
				Date:              sessionDate,
			})
			totalSessions++
			totalMessages += msgCount
		}
	}

	return totalSessions, totalMessages
}
