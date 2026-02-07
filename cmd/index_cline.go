package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

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
		if info, err := taskDir.Info(); err == nil && skipOldFile(info) {
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
		Ts      int64    `json:"ts"`
		Type    string   `json:"type"`
		Say     string   `json:"say"`
		Ask     string   `json:"ask"`
		Text    string   `json:"text"`
		Images  []string `json:"images"`
		Partial *bool    `json:"partial"`
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

	// Track pending API request tokens to attach to the next assistant message
	var pendingInputTokens, pendingOutputTokens, pendingCacheRead, pendingCacheWrite int
	var pendingCost float64

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
				pendingInputTokens += apiReq.TokensIn
				pendingOutputTokens += apiReq.TokensOut
				pendingCacheRead += apiReq.CacheReads
				pendingCacheWrite += apiReq.CacheWrites
				pendingCost += apiReq.Cost
				if sessionProvider == "" && apiReq.InferenceProvider != "" {
					sessionProvider = apiReq.InferenceProvider
				}
			}
			continue
		}

		if msg.Text == "" {
			continue
		}

		if msg.Say == "api_req_finished" || msg.Say == "checkpoint_saved" || msg.Say == "reasoning" {
			continue
		}

		var role string
		if msg.Type == "say" && (msg.Say == "text" || msg.Say == "user_feedback") {
			if msg.Images != nil {
				role = "user"
			} else if msg.Partial != nil {
				role = "assistant"
			} else {
				role = "user"
			}
		} else if msg.Type == "say" && msg.Say == "completion_result" {
			role = "assistant"
		} else if msg.Type == "say" && msg.Say == "" {
			role = "assistant"
		} else {
			continue
		}

		if role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(msg.Text, 200)
		}

		timestamp := time.UnixMilli(msg.Ts)

		// Attach pending API request tokens to assistant messages
		var msgInput, msgOutput, msgCacheRead, msgCacheWrite int
		var msgCost float64
		if role == "assistant" && pendingInputTokens > 0 {
			msgInput = pendingInputTokens
			msgOutput = pendingOutputTokens
			msgCacheRead = pendingCacheRead
			msgCacheWrite = pendingCacheWrite
			msgCost = pendingCost
			pendingInputTokens = 0
			pendingOutputTokens = 0
			pendingCacheRead = 0
			pendingCacheWrite = 0
			pendingCost = 0
		}

		err := db.InsertMessage(db.Message{
			SessionID:        taskID,
			Project:          projectName,
			Role:             role,
			Content:          msg.Text,
			Timestamp:        timestamp,
			Tool:             toolName,
			Provider:         sessionProvider,
			InputTokens:      msgInput,
			OutputTokens:     msgOutput,
			CacheReadTokens:  msgCacheRead,
			CacheWriteTokens: msgCacheWrite,
			CostUSD:          msgCost,
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
