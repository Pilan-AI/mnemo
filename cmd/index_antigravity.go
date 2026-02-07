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
		s, m := indexAntigravitySession(jsonlPath)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

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

		err := db.InsertMessage(db.Message{
			SessionID: sessionID,
			Role:      role,
			Content:   event.Content,
			Project:   projectName,
			Tool:      "antigravity",
			Timestamp: timestamp,
		})
		if err == nil {
			messagesIndexed++
		}
	}

	if messagesIndexed > 0 {
		_ = db.InsertSessionSimple(sessionID, projectName, firstQuery, jsonlPath, "antigravity", messagesIndexed)
		return 1, messagesIndexed
	}

	return 0, 0
}
