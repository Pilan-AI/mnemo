// index_claude.go indexes Claude Code sessions stored as JSONL files.
// Session data lives under ~/.claude/projects/<project-hash>/<session>.jsonl.
package cmd

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

// indexClaudeCode walks the Claude Code projects directory and indexes each
// JSONL session file. Returns total (sessions, messages) indexed.
func indexClaudeCode(basePath string) (int, int) {
	sessions := 0
	messages := 0

	if err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			indexErrors++
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".jsonl") {
			if skipOldFile(info) {
				return nil
			}
			sessionID := strings.TrimSuffix(info.Name(), ".jsonl")
			if isSessionUnchanged(sessionID, info.ModTime()) {
				return nil
			}
			s, m := indexJSONLSession(path, "claude")
			sessions += s
			messages += m
		}

		return nil
	}); err != nil {
		indexErrors++
	}

	return sessions, messages
}

// indexJSONLSession parses a single JSONL session file (one JSON object per line)
// and inserts all messages atomically within a transaction.
func indexJSONLSession(path, tool string) (int, int) {
	file, err := os.Open(path)
	if err != nil {
		indexErrors++
		return 0, 0
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		indexErrors++
		return 0, 0
	}
	if info.Size() == 0 {
		return 0, 0
	}

	var firstUserMsg string
	sessionID := filepath.Base(path)
	sessionID = strings.TrimSuffix(sessionID, ".jsonl")
	projectName := extractProjectName(path)
	msgCount := 0

	var sessionCwd, sessionGitBranch, sessionVersion string
	var sessionModel, sessionProvider string
	var sessionStartTime, sessionEndTime time.Time
	var totalInputTokens, totalOutputTokens int

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

	// Stream JSONL line by line to avoid loading entire file into memory
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // Up to 10MB per line

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		entryType, _ := entry["type"].(string)
		if entryType != "user" && entryType != "assistant" {
			continue
		}

		var content string
		var role string
		var msgModel string
		var msgInputTokens, msgOutputTokens int

		uuid, _ := entry["uuid"].(string)
		parentUuid, _ := entry["parentUuid"].(string)
		cwd, _ := entry["cwd"].(string)
		gitBranch, _ := entry["gitBranch"].(string)
		version, _ := entry["version"].(string)

		timestamp := time.Now()
		if tsStr, ok := entry["timestamp"].(string); ok && tsStr != "" {
			if parsed, err := time.Parse(time.RFC3339, tsStr); err == nil {
				timestamp = parsed
			}
		}

		if sessionCwd == "" && cwd != "" {
			sessionCwd = cwd
		}
		if sessionGitBranch == "" && gitBranch != "" {
			sessionGitBranch = gitBranch
		}
		if sessionVersion == "" && version != "" {
			sessionVersion = version
		}
		if sessionStartTime.IsZero() {
			sessionStartTime = timestamp
		}
		sessionEndTime = timestamp

		if msg, ok := entry["message"].(map[string]interface{}); ok {
			role, _ = msg["role"].(string)

			if m, ok := msg["model"].(string); ok && m != "" {
				msgModel = m
				if sessionModel == "" {
					sessionModel = m
				}
			}

			if usage, ok := msg["usage"].(map[string]interface{}); ok {
				if v, ok := usage["input_tokens"].(float64); ok {
					msgInputTokens = int(v)
				}
				if v, ok := usage["output_tokens"].(float64); ok {
					msgOutputTokens = int(v)
				}
			}

			switch c := msg["content"].(type) {
			case string:
				content = c
			case []interface{}:
				for _, item := range c {
					if block, ok := item.(map[string]interface{}); ok {
						if text, ok := block["text"].(string); ok {
							content += text + " "
						}
					}
				}
			}
		} else {
			role = entryType
			if c, ok := entry["content"].(string); ok {
				content = c
			}
		}

		if content == "" {
			continue
		}

		totalInputTokens += msgInputTokens
		totalOutputTokens += msgOutputTokens

		if role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(content, 200)
		}

		msgProvider := inferProviderFromModel(msgModel)
		if sessionProvider == "" && msgProvider != "" {
			sessionProvider = msgProvider
		}

		err := db.TxInsertMessage(tx, db.Message{
			SessionID:        sessionID,
			Project:          projectName,
			Role:             role,
			Content:          content,
			Timestamp:        timestamp,
			Tool:             tool,
			Model:            msgModel,
			Provider:         msgProvider,
			InputTokens:      msgInputTokens,
			OutputTokens:     msgOutputTokens,
			MessageUUID:      uuid,
			ParentUUID:       parentUuid,
			WorkingDirectory: cwd,
		})
		if err != nil {
			indexErrors++
			continue
		}
		msgCount++
	}

	if err := scanner.Err(); err != nil {
		indexErrors++
	}

	if msgCount > 0 {
		if err := db.TxInsertSession(tx, db.Session{
			ID:                sessionID,
			Project:           projectName,
			FirstQuery:        firstUserMsg,
			MessageCount:      msgCount,
			Tool:              tool,
			FilePath:          path,
			Model:             sessionModel,
			Provider:          sessionProvider,
			TotalInputTokens:  totalInputTokens,
			TotalOutputTokens: totalOutputTokens,
			CLIVersion:        sessionVersion,
			GitBranch:         sessionGitBranch,
			WorkingDirectory:  sessionCwd,
			StartTime:         sessionStartTime,
			EndTime:           sessionEndTime,
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
