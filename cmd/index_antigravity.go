// index_antigravity.go indexes Antigravity IDE sessions stored as JSONL files.
// Sessions live under ~/.gemini/antigravity/code_tracker/<project>/*.jsonl.
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

// indexAntigravityCodeTracker walks the Antigravity code_tracker directory
// and indexes each JSONL session file. Returns total (sessions, messages) indexed.
func indexAntigravityCodeTracker(codeTrackerPath string) (int, int) {
	jsonlFiles, err := filepath.Glob(filepath.Join(codeTrackerPath, "*", "*.jsonl"))
	if err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, jsonlPath := range jsonlFiles {
		if info, err := os.Stat(jsonlPath); err == nil && skipOldFile(info) {
			continue
		}
		fileSessionID := strings.TrimSuffix(filepath.Base(jsonlPath), ".jsonl")
		if info, err := os.Stat(jsonlPath); err == nil && isSessionUnchanged(fileSessionID, info.ModTime()) {
			continue
		}
		s, m := indexAntigravitySession(jsonlPath)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

// indexAntigravitySession parses a single Antigravity JSONL file and
// inserts all messages atomically within a transaction.
func indexAntigravitySession(jsonlPath string) (int, int) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = file.Close() }()

	sessionID := filepath.Base(jsonlPath)
	sessionID = strings.TrimSuffix(sessionID, ".jsonl")

	parentDir := filepath.Dir(jsonlPath)
	projectName := filepath.Base(parentDir)
	if projectName == "no_repo" {
		projectName = "antigravity"
	}

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

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	messagesIndexed := 0
	var firstQuery string

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Antigravity JSONL files have a binary/protobuf-style prefix before JSON
		// Strip everything before the first '{' to get valid JSON
		jsonStart := strings.Index(line, "{")
		if jsonStart == -1 {
			continue
		}
		jsonLine := line[jsonStart:]

		var event struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			Content   string `json:"content"`
		}

		if err := json.Unmarshal([]byte(jsonLine), &event); err != nil {
			continue
		}

		if event.Type != "user" && event.Type != "assistant" {
			continue
		}

		if event.Content == "" {
			continue
		}

		role := event.Type
		timestamp := time.Now()
		if event.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339, event.Timestamp); err == nil {
				timestamp = t
			}
		}

		if firstQuery == "" && role == "user" {
			if len(event.Content) > 100 {
				firstQuery = event.Content[:100] + "..."
			} else {
				firstQuery = event.Content
			}
		}

		err := db.TxInsertMessage(tx, db.Message{
			SessionID: sessionID,
			Role:      role,
			Content:   event.Content,
			Project:   projectName,
			Tool:      "antigravity",
			Timestamp: timestamp,
		})
		if err != nil {
			indexErrors++
			continue
		}
		messagesIndexed++
	}

	if err := scanner.Err(); err != nil {
		indexErrors++
	}

	if messagesIndexed > 0 {
		if err := db.TxInsertSessionSimple(tx, sessionID, projectName, firstQuery, jsonlPath, "antigravity", messagesIndexed); err != nil {
			indexErrors++
			return 0, messagesIndexed
		}
		if err := tx.Commit(); err != nil {
			indexErrors++
			return 0, messagesIndexed
		}
		return 1, messagesIndexed
	}

	return 0, 0
}
