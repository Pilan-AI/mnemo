// index_cursor.go indexes Cursor IDE sessions from its SQLite state database.
// Cursor stores composer conversations in state.vscdb under the globalStorage
// directory. Each composer session is indexed atomically using a closure pattern.
package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

// indexCursorGlobalStorage opens the Cursor state.vscdb database, queries
// composer conversations, and indexes each session atomically.
// Returns total (sessions, messages) indexed.
func indexCursorGlobalStorage(dbPath string) (int, int) {
	sqliteDB, err := db.OpenReadOnlySQLite(dbPath)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = sqliteDB.Close() }()

	rows, err := sqliteDB.Query("SELECT key, value FROM cursorDiskKV WHERE key LIKE 'composerData:%'")
	if err != nil {
		return 0, 0
	}
	defer func() { _ = rows.Close() }()

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

		var sessionStartTime, sessionEndTime time.Time

		if composer.CreatedAt > 0 {
			sessionStartTime = time.UnixMilli(composer.CreatedAt)
		}
		if composer.LastUpdated > 0 {
			sessionEndTime = time.UnixMilli(composer.LastUpdated)
		}

		s, m := func() (int, int) {
			tx, txErr := db.BeginTx()
			if txErr != nil {
				indexErrors++
				return 0, 0
			}
			defer func() { _ = tx.Rollback() }()

			if err := db.TxDeleteSessionMessages(tx, sessionID); err != nil {
				indexErrors++
				return 0, 0
			}

			localMsgCount := 0
			localInputTokens := 0
			localOutputTokens := 0
			localFirstUserMsg := ""

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

				if role == "user" && localFirstUserMsg == "" {
					localFirstUserMsg = truncate(bubbleData.Text, 200)
				}

				inputTokens := bubbleData.TokenCount.InputTokens
				outputTokens := bubbleData.TokenCount.OutputTokens
				localInputTokens += inputTokens
				localOutputTokens += outputTokens

				timestamp := sessionStartTime
				if timestamp.IsZero() {
					timestamp = time.Now()
				}

				err := db.TxInsertMessage(tx, db.Message{
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
				localMsgCount++
			}

			if localMsgCount > 0 {
				sessionDate := ""
				if !sessionStartTime.IsZero() {
					sessionDate = sessionStartTime.Format("2006-01-02")
				}
				if err := db.TxInsertSession(tx, db.Session{
					ID:                sessionID,
					Project:           projectName,
					FirstQuery:        localFirstUserMsg,
					MessageCount:      localMsgCount,
					Tool:              "cursor",
					FilePath:          dbPath,
					Provider:          "cursor",
					TotalInputTokens:  localInputTokens,
					TotalOutputTokens: localOutputTokens,
					StartTime:         sessionStartTime,
					EndTime:           sessionEndTime,
					Date:              sessionDate,
				}); err != nil {
					indexErrors++
					return 0, localMsgCount
				}
				if err := tx.Commit(); err != nil {
					indexErrors++
					return 0, localMsgCount
				}
				return 1, localMsgCount
			}
			return 0, 0
		}()
		totalSessions += s
		totalMessages += m
	}
	if err := rows.Err(); err != nil {
		indexErrors++
	}

	return totalSessions, totalMessages
}
