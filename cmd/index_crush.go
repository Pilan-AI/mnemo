package cmd

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

// indexCrush indexes Crush CLI sessions from ~/.crush/crush.db
// Crush has the cleanest SQLite schema of all tools with proper sessions/messages tables
func indexCrush(dbPath string) (int, int) {
	crushDB, err := db.OpenReadOnlySQLite(dbPath)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = crushDB.Close() }()

	rows, err := crushDB.Query(`
		SELECT s.id, s.title, m.role, m.parts, m.created_at, m.model, m.provider
		FROM sessions s
		JOIN messages m ON m.session_id = s.id
		ORDER BY s.id, m.created_at
	`)
	if err != nil {
		return 0, 0
	}
	defer func() { _ = rows.Close() }()

	sessionMsgCounts := make(map[string]int)
	sessionFirstMsg := make(map[string]string)
	sessionModels := make(map[string]string)
	sessionProviders := make(map[string]string)
	totalMessages := 0

	for rows.Next() {
		var sessionID, title, role, partsJSON string
		var model, provider sql.NullString
		var createdAt int64

		if err := rows.Scan(&sessionID, &title, &role, &partsJSON, &createdAt, &model, &provider); err != nil {
			continue
		}

		var parts []map[string]interface{}
		if err := json.Unmarshal([]byte(partsJSON), &parts); err != nil {
			continue
		}

		var content string
		for _, part := range parts {
			partType, _ := part["type"].(string)
			if partType == "text" {
				if data, ok := part["data"].(map[string]interface{}); ok {
					if text, ok := data["text"].(string); ok {
						content += text
					}
				}
			}
		}

		if content == "" {
			continue
		}

		projectName := title
		if projectName == "" {
			projectName = "crush"
		}

		modelStr := ""
		if model.Valid {
			modelStr = model.String
		}
		providerStr := ""
		if provider.Valid {
			providerStr = provider.String
		}

		if sessionModels[sessionID] == "" && modelStr != "" {
			sessionModels[sessionID] = modelStr
		}
		if sessionProviders[sessionID] == "" && providerStr != "" {
			sessionProviders[sessionID] = providerStr
		}

		timestamp := time.UnixMilli(createdAt)

		err := db.InsertMessage(db.Message{
			SessionID: sessionID,
			Project:   projectName,
			Role:      role,
			Content:   content,
			Timestamp: timestamp,
			Tool:      "crush",
			Model:     modelStr,
			Provider:  providerStr,
		})
		if err != nil {
			indexErrors++
			continue
		}

		sessionMsgCounts[sessionID]++
		if role == "user" && sessionFirstMsg[sessionID] == "" {
			sessionFirstMsg[sessionID] = truncate(content, 200)
		}
		totalMessages++
	}

	// Fetch session-level token/cost data from Crush's sessions table
	type crushSessionStats struct {
		promptTokens     int
		completionTokens int
		cost             float64
	}
	sessionStats := make(map[string]crushSessionStats)
	statsRows, statsErr := crushDB.Query(`SELECT id, prompt_tokens, completion_tokens, cost FROM sessions`)
	if statsErr == nil {
		defer func() { _ = statsRows.Close() }()
		for statsRows.Next() {
			var sid string
			var pt, ct int
			var c float64
			if err := statsRows.Scan(&sid, &pt, &ct, &c); err == nil {
				sessionStats[sid] = crushSessionStats{promptTokens: pt, completionTokens: ct, cost: c}
			}
		}
	}

	for sessionID, msgCount := range sessionMsgCounts {
		stats := sessionStats[sessionID]
		_ = db.InsertSession(db.Session{
			ID:                sessionID,
			Project:           "crush",
			FirstQuery:        sessionFirstMsg[sessionID],
			MessageCount:      msgCount,
			Tool:              "crush",
			FilePath:          dbPath,
			Model:             sessionModels[sessionID],
			Provider:          sessionProviders[sessionID],
			TotalInputTokens:  stats.promptTokens,
			TotalOutputTokens: stats.completionTokens,
			TotalCostUSD:      stats.cost,
		})
	}

	return len(sessionMsgCounts), totalMessages
}

func scanCrushPerProject(home string, sessions, messages int) (int, int) {
	enabledProjects, err := db.GetEnabledProjects()
	if err != nil || len(enabledProjects) == 0 {
		projectsPath := filepath.Join(home, "Projects")
		if pathExists(projectsPath) {
			_ = filepath.Walk(projectsPath, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.Name() == "crush.db" && strings.Contains(path, ".crush") {
					s, m := indexCrush(path)
					sessions += s
					messages += m
				}
				return nil
			})
		}
		return sessions, messages
	}

	scannedPaths := make(map[string]bool)
	for _, project := range enabledProjects {
		crushDB := filepath.Join(project.Path, ".crush", "crush.db")
		if scannedPaths[crushDB] {
			continue
		}
		scannedPaths[crushDB] = true

		if pathExists(crushDB) {
			s, m := indexCrush(crushDB)
			sessions += s
			messages += m
		}
	}

	return sessions, messages
}
