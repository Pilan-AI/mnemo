package cmd

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

func indexClaudeCode(basePath string) (int, int) {
	sessions := 0
	messages := 0

	// Recursively walk through all directories to find .jsonl files
	_ = filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Skip directories, only process .jsonl files
		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), ".jsonl") {
			if skipOldFile(info) {
				return nil
			}
			s, m := indexJSONLSession(path, "claude")
			sessions += s
			messages += m
		}

		return nil
	})

	return sessions, messages
}

func indexJSONLSession(path, tool string) (int, int) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = file.Close() }()

	info, _ := file.Stat()
	if info.Size() == 0 {
		return 0, 0
	}

	// Read file content
	data, _ := io.ReadAll(file)
	lines := strings.Split(string(data), "\n")

	var firstUserMsg string
	sessionID := filepath.Base(path)
	sessionID = strings.TrimSuffix(sessionID, ".jsonl")
	projectName := extractProjectName(path)
	msgCount := 0

	var sessionCwd, sessionGitBranch, sessionVersion string
	var sessionModel, sessionProvider string
	var sessionStartTime, sessionEndTime time.Time
	var totalInputTokens, totalOutputTokens int

	for _, line := range lines {
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		// Skip non-message entries
		entryType, _ := entry["type"].(string)
		if entryType != "user" && entryType != "assistant" {
			continue
		}

		// Extract message content
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

		// Try new format first: {"type":"user","message":{"role":"user","content":"...","model":"...","usage":{...}}}
		if msg, ok := entry["message"].(map[string]interface{}); ok {
			role, _ = msg["role"].(string)

			// Extract model
			if m, ok := msg["model"].(string); ok && m != "" {
				msgModel = m
				if sessionModel == "" {
					sessionModel = m
				}
			}

			// Extract token usage
			if usage, ok := msg["usage"].(map[string]interface{}); ok {
				if v, ok := usage["input_tokens"].(float64); ok {
					msgInputTokens = int(v)
				}
				if v, ok := usage["output_tokens"].(float64); ok {
					msgOutputTokens = int(v)
				}
			}

			// Handle different content formats
			switch c := msg["content"].(type) {
			case string:
				content = c
			case []interface{}:
				// Claude's content array format
				for _, item := range c {
					if block, ok := item.(map[string]interface{}); ok {
						if text, ok := block["text"].(string); ok {
							content += text + " "
						}
					}
				}
			}
		} else {
			// Try old format: {"type":"user","content":"..."}
			role = entryType // "user" or "assistant"
			if c, ok := entry["content"].(string); ok {
				content = c
			}
		}

		if content == "" {
			continue
		}

		totalInputTokens += msgInputTokens
		totalOutputTokens += msgOutputTokens

		// Capture first user message
		if role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(content, 200)
		}

		// Determine provider from model name
		msgProvider := ""
		if msgModel != "" {
			if strings.Contains(msgModel, "claude") {
				msgProvider = "anthropic"
			} else if strings.Contains(msgModel, "gpt") || strings.Contains(msgModel, "o1") || strings.Contains(msgModel, "o3") {
				msgProvider = "openai"
			} else if strings.Contains(msgModel, "gemini") || strings.Contains(msgModel, "gemma") {
				msgProvider = "google"
			}
			if sessionProvider == "" && msgProvider != "" {
				sessionProvider = msgProvider
			}
		}

		err := db.InsertMessage(db.Message{
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

	if msgCount > 0 {
		_ = db.InsertSession(db.Session{
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
		})
		return 1, msgCount
	}

	return 0, 0
}
