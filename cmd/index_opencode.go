// index_opencode.go indexes OpenCode sessions from either SQLite (1.2.0+) or JSON (pre-1.2.0).
// SQLite: ~/.local/share/opencode/opencode.db
// JSON: ~/.local/share/opencode/storage/message/<session-id>/msg_*.json
package cmd

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
	_ "github.com/mattn/go-sqlite3"
)

// indexOpencode walks the OpenCode sessions and indexes each session.
// Supports both SQLite (1.2.0+) and JSON (pre-1.2.0) formats.
// Returns total (sessions, messages) indexed.
func indexOpencode(basePath string) (int, int) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, 0
	}

	// Try SQLite first (OpenCode 1.2.0+)
	dbPath := filepath.Join(home, ".local/share/opencode/opencode.db")
	if pathExists(dbPath) {
		return indexOpenCodeSQLite(dbPath)
	}

	// Fall back to JSON format (OpenCode <1.2.0)
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

// indexOpenCodeSQLite indexes sessions from OpenCode 1.2.0+ SQLite database.
// Uses a collect-then-process pattern: reads all session metadata into memory,
// closes the outer cursor, then indexes each session individually. This avoids
// holding a rows cursor open while making nested queries on the same connection.
// Returns total (sessions, messages) indexed.
func indexOpenCodeSQLite(dbPath string) (int, int) {
	sqliteDB, err := db.OpenReadOnlySQLite(dbPath)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = sqliteDB.Close() }()

	type openCodeSession struct {
		id        string
		directory string
		title     string
		version   string
		updated   int64
	}

	rows, err := sqliteDB.Query(`
		SELECT id, directory, title, version, time_updated
		FROM session
		ORDER BY time_created DESC
	`)
	if err != nil {
		return 0, 0
	}

	var sessions []openCodeSession
	for rows.Next() {
		var s openCodeSession
		if err := rows.Scan(&s.id, &s.directory, &s.title, &s.version, &s.updated); err != nil {
			continue
		}
		if isSessionUnchanged(s.id, time.UnixMilli(s.updated)) {
			continue
		}
		sessions = append(sessions, s)
	}
	rows.Close()

	totalSessions := 0
	totalMessages := 0

	for _, s := range sessions {
		numS, numM := indexOpenCodeSQLiteSession(sqliteDB, s.id, s.directory, s.title, s.version)
		totalSessions += numS
		totalMessages += numM
	}

	return totalSessions, totalMessages
}

// indexOpenCodeSQLiteSession indexes a single session from SQLite.
func indexOpenCodeSQLiteSession(sqliteDB *sql.DB, sessionID, directory, title, version string) (int, int) {
	// Get all messages for this session
	msgRows, err := sqliteDB.Query(`
		SELECT id, time_created, data
		FROM message
		WHERE session_id = ?
		ORDER BY time_created ASC
	`, sessionID)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = msgRows.Close() }()

	projectName := ""
	if directory != "" {
		projectName = filepath.Base(directory)
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

	for msgRows.Next() {
		var msgID string
		var timeCreated int64
		var dataJSON string
		if err := msgRows.Scan(&msgID, &timeCreated, &dataJSON); err != nil {
			continue
		}

		var msgData map[string]interface{}
		if err := json.Unmarshal([]byte(dataJSON), &msgData); err != nil {
			continue
		}

		role, _ := msgData["role"].(string)
		if role == "" || (role != "user" && role != "assistant") {
			continue
		}

		content := getOpenCodeMessageContent(sqliteDB, msgID)
		if content == "" {
			continue
		}

		// Extract model info
		var model, provider string
		if modelMap, ok := msgData["model"].(map[string]interface{}); ok {
			model, _ = modelMap["modelID"].(string)
			provider, _ = modelMap["providerID"].(string)
		}
		if model == "" {
			if modelID, ok := msgData["modelID"].(string); ok {
				model = modelID
			}
		}
		if provider == "" {
			if providerID, ok := msgData["providerID"].(string); ok {
				provider = providerID
			}
		}

		if sessionModel == "" && model != "" {
			sessionModel = model
		}
		if sessionProvider == "" && provider != "" {
			sessionProvider = provider
		}

		// Extract timestamps
		msgTime := time.UnixMilli(timeCreated)
		if sessionStartTime.IsZero() || msgTime.Before(sessionStartTime) {
			sessionStartTime = msgTime
		}
		if msgTime.After(sessionEndTime) {
			sessionEndTime = msgTime
		}

		// Extract tokens
		var inputTokens, outputTokens, cacheRead, cacheWrite, reasoning int
		if tokensMap, ok := msgData["tokens"].(map[string]interface{}); ok {
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
		if v, ok := msgData["cost"].(float64); ok {
			costUSD = v
		}

		totalInputTokens += inputTokens
		totalOutputTokens += outputTokens
		totalCacheRead += cacheRead
		totalCacheWrite += cacheWrite
		totalReasoningTokens += reasoning
		totalCostUSD += costUSD

		// Track first user message for session title
		if role == "user" && firstUserMsg == "" && !isSystemDirective(content) {
			firstUserMsg = truncate(sanitizeContent(content), 200)
		}

		// Insert message
		msgDate := msgTime.Format("2006-01-02")
		if err := db.TxInsertMessage(tx, db.Message{
			SessionID:        sessionID,
			Project:          projectName,
			Role:             role,
			Content:          content,
			Timestamp:        msgTime,
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

	// Create session record
	if msgCount > 0 {
		if projectName == "" {
			projectName = "opencode"
		}

		sessionQuery := firstUserMsg
		if sessionQuery == "" {
			sessionQuery = title
		}

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
			FilePath:             directory,
			Model:                sessionModel,
			Provider:             sessionProvider,
			TotalInputTokens:     totalInputTokens,
			TotalOutputTokens:    totalOutputTokens,
			TotalCacheRead:       totalCacheRead,
			TotalCacheWrite:      totalCacheWrite,
			TotalReasoningTokens: totalReasoningTokens,
			TotalCostUSD:         totalCostUSD,
			CLIVersion:           version,
			WorkingDirectory:     directory,
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

// getOpenCodeMessageContent queries the part table for a message's text content.
// OpenCode 1.2.0+ stores message content in a separate part table rather than
// inline in the message data.
func getOpenCodeMessageContent(sqliteDB *sql.DB, messageID string) string {
	partRows, err := sqliteDB.Query(`
		SELECT data
		FROM part
		WHERE message_id = ?
		ORDER BY time_created ASC
	`, messageID)
	if err != nil {
		return ""
	}
	defer func() { _ = partRows.Close() }()

	var contentParts []string
	for partRows.Next() {
		var partDataJSON string
		if err := partRows.Scan(&partDataJSON); err != nil {
			continue
		}
		var partData map[string]interface{}
		if err := json.Unmarshal([]byte(partDataJSON), &partData); err != nil {
			continue
		}
		partType, _ := partData["type"].(string)
		if partType == "text" {
			if text, ok := partData["text"].(string); ok && text != "" {
				contentParts = append(contentParts, text)
			}
		}
	}

	return strings.Join(contentParts, "\n")
}
