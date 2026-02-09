// index_opencode.go indexes OpenCode sessions stored as JSON files.
// Sessions live under ~/.local/share/opencode/sessions/<session-id>/.
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

// indexOpencode walks the OpenCode sessions directory and indexes each session.
// Returns total (sessions, messages) indexed.
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
		if info, err := sessionDir.Info(); err == nil && skipOldFile(info) {
			continue
		}

		sessionID := sessionDir.Name()
		// Incremental: skip if session file hasn't changed
		if info, err := sessionDir.Info(); err == nil && isSessionUnchanged(sessionID, info.ModTime()) {
			continue
		}
		s, m := indexOpenCodeSession(storagePath, sessionID)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

// indexOpenCodeSession parses a single OpenCode session directory containing
// session.json (metadata) and messages.json (conversation), inserting atomically.
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

	sessionMeta := findOpenCodeSessionMeta(storagePath, sessionID)

	projectName := ""
	var sessionWorkingDir string
	var cliVersion string
	if sessionMeta != nil {
		if sessionMeta.Directory != "" {
			projectName = filepath.Base(sessionMeta.Directory)
			sessionWorkingDir = sessionMeta.Directory
		}
		cliVersion = sessionMeta.Version
	}

	var firstUserMsg string
	var sessionModel, sessionProvider string
	var sessionStartTime, sessionEndTime time.Time
	var totalInputTokens, totalOutputTokens, totalCacheRead, totalCacheWrite, totalReasoningTokens int
	var totalCostUSD float64
	msgCount := 0

	tx, err := db.BeginTx()
	if err != nil {
		indexErrors++
		return 0, 0
	}
	defer func() { _ = tx.Rollback() }()

	if err := db.TxDeleteSessionMessages(tx, sessionID); err != nil {
		indexErrors++
		return 0, 0
	}

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

		msgDate := msgTimestamp.Format("2006-01-02")

		totalInputTokens += inputTokens
		totalOutputTokens += outputTokens
		totalCacheRead += cacheRead
		totalCacheWrite += cacheWrite
		totalReasoningTokens += reasoning
		totalCostUSD += costUSD

		if sessionStartTime.IsZero() || msgTimestamp.Before(sessionStartTime) {
			sessionStartTime = msgTimestamp
		}
		if msgTimestamp.After(sessionEndTime) {
			sessionEndTime = msgTimestamp
		}

		if role == "user" && firstUserMsg == "" && !isSystemDirective(content) {
			firstUserMsg = truncate(sanitizeContent(content), 200)
		}

		if err := db.TxInsertMessage(tx, db.Message{
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
			ReasoningTokens:  reasoning,
			CostUSD:          costUSD,
			Date:             msgDate,
		}); err != nil {
			indexErrors++
			continue
		}

		msgCount++
	}

	if msgCount > 0 {
		if projectName == "" {
			projectName = "opencode"
		}

		sessionQuery := firstUserMsg
		if sessionQuery == "" && sessionMeta != nil && sessionMeta.Title != "" {
			sessionQuery = sessionMeta.Title
		}

		sessionPath := filepath.Join(storagePath, "session")
		sessionDate := ""
		if !sessionStartTime.IsZero() {
			sessionDate = sessionStartTime.Format("2006-01-02")
		}
		if err := db.TxInsertSession(tx, db.Session{
			ID:                   sessionID,
			Project:              projectName,
			FirstQuery:           sessionQuery,
			MessageCount:         msgCount,
			Tool:                 "opencode",
			FilePath:             sessionPath,
			Model:                sessionModel,
			Provider:             sessionProvider,
			TotalInputTokens:     totalInputTokens,
			TotalOutputTokens:    totalOutputTokens,
			TotalCacheRead:       totalCacheRead,
			TotalCacheWrite:      totalCacheWrite,
			TotalReasoningTokens: totalReasoningTokens,
			TotalCostUSD:         totalCostUSD,
			CLIVersion:           cliVersion,
			WorkingDirectory:     sessionWorkingDir,
			StartTime:            sessionStartTime,
			EndTime:              sessionEndTime,
			Date:                 sessionDate,
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
