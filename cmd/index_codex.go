package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

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
				if skipOldFile(info) {
					return nil
				}
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
	var sessionTotalInput, sessionTotalOutput, sessionTotalReasoning int
	var messages []struct {
		role            string
		content         string
		timestamp       time.Time
		model           string
		inputTokens     int
		outputTokens    int
		reasoningTokens int
	}

	// pendingTokens tracks last_token_usage from the most recent token_count event,
	// to be attached to the preceding assistant message when we finalize.
	var pendingInputTokens, pendingOutputTokens, pendingReasoningTokens int
	var hasPendingTokens bool

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

				if msgType == "token_count" {
					// Extract per-turn tokens from last_token_usage
					if info, ok := payload["info"].(map[string]interface{}); ok {
						if lastUsage, ok := info["last_token_usage"].(map[string]interface{}); ok {
							pendingInputTokens = 0
							pendingOutputTokens = 0
							pendingReasoningTokens = 0
							if v, ok := lastUsage["input_tokens"].(float64); ok {
								pendingInputTokens = int(v)
							}
							if v, ok := lastUsage["output_tokens"].(float64); ok {
								pendingOutputTokens = int(v)
							}
							if v, ok := lastUsage["reasoning_output_tokens"].(float64); ok {
								pendingReasoningTokens = int(v)
							}
							hasPendingTokens = true
						}
						// Extract session-level totals (last one wins)
						if totalUsage, ok := info["total_token_usage"].(map[string]interface{}); ok {
							if v, ok := totalUsage["input_tokens"].(float64); ok {
								sessionTotalInput = int(v)
							}
							if v, ok := totalUsage["output_tokens"].(float64); ok {
								sessionTotalOutput = int(v)
							}
							if v, ok := totalUsage["reasoning_output_tokens"].(float64); ok {
								sessionTotalReasoning = int(v)
							}
						}
					}
					continue
				}

				message, _ := payload["message"].(string)

				if message == "" {
					continue
				}

				var role string
				switch msgType {
				case "user_message":
					role = "user"
				case "agent_message":
					role = "assistant"
				default:
					continue
				}

				messages = append(messages, struct {
					role            string
					content         string
					timestamp       time.Time
					model           string
					inputTokens     int
					outputTokens    int
					reasoningTokens int
				}{role: role, content: message, timestamp: timestamp, model: currentModel})

				// If we have pending tokens and this is an assistant message,
				// they belong to this response. Attach them.
				if role == "assistant" && hasPendingTokens {
					messages[len(messages)-1].inputTokens = pendingInputTokens
					messages[len(messages)-1].outputTokens = pendingOutputTokens
					messages[len(messages)-1].reasoningTokens = pendingReasoningTokens
					hasPendingTokens = false
				}
			}
		}
	}

	// token_count events come AFTER the agent_message, so check if any
	// trailing tokens should be attached to the last assistant message.
	if hasPendingTokens {
		for i := len(messages) - 1; i >= 0; i-- {
			if messages[i].role == "assistant" && messages[i].inputTokens == 0 {
				messages[i].inputTokens = pendingInputTokens
				messages[i].outputTokens = pendingOutputTokens
				messages[i].reasoningTokens = pendingReasoningTokens
				break
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

	// Infer provider from model name if not already set
	if sessionProvider == "" && sessionModel != "" {
		if strings.Contains(sessionModel, "gpt") || strings.Contains(sessionModel, "o1") || strings.Contains(sessionModel, "o3") || strings.Contains(sessionModel, "codex") {
			sessionProvider = "openai"
		} else if strings.Contains(sessionModel, "claude") {
			sessionProvider = "anthropic"
		} else if strings.Contains(sessionModel, "gemini") || strings.Contains(sessionModel, "gemma") {
			sessionProvider = "google"
		}
	}

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
			InputTokens:      msg.inputTokens,
			OutputTokens:     msg.outputTokens,
			ReasoningTokens:  msg.reasoningTokens,
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
			ID:                   sessionID,
			Project:              "codex",
			FirstQuery:           firstUserMsg,
			MessageCount:         msgCount,
			Tool:                 "codex",
			FilePath:             sessionPath,
			Model:                sessionModel,
			Provider:             sessionProvider,
			TotalInputTokens:     sessionTotalInput,
			TotalOutputTokens:    sessionTotalOutput,
			TotalReasoningTokens: sessionTotalReasoning,
			CLIVersion:           cliVersion,
			WorkingDirectory:     cwd,
		})
		return 1, msgCount
	}

	return 0, 0
}
