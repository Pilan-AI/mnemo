package db

import "time"

// Session aggregates all messages from a single AI coding conversation.
// The ID is typically derived from the source tool's session identifier.
type Session struct {
	ID                   string
	Project              string
	FirstQuery           string
	MessageCount         int
	Tool                 string
	FilePath             string
	IndexedAt            time.Time
	Model                string
	Provider             string
	TotalInputTokens     int
	TotalOutputTokens    int
	TotalCacheRead       int
	TotalCacheWrite      int
	TotalReasoningTokens int
	TotalCostUSD         float64
	CLIVersion           string
	GitBranch            string
	WorkingDirectory     string
	StartTime            time.Time
	EndTime              time.Time
	Agent                string
	Date                 string
}

func InsertSession(sess Session) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO sessions (
			id, project, first_query, message_count, tool, file_path, indexed_at,
			model, provider, total_input_tokens, total_output_tokens,
			total_cache_read, total_cache_write, total_reasoning_tokens, total_cost_usd,
			cli_version, git_branch, working_directory, start_time, end_time, agent, date
		) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		sess.ID, sess.Project, sess.FirstQuery, sess.MessageCount, sess.Tool, sess.FilePath,
		sess.Model, sess.Provider, sess.TotalInputTokens, sess.TotalOutputTokens,
		sess.TotalCacheRead, sess.TotalCacheWrite, sess.TotalReasoningTokens, sess.TotalCostUSD,
		sess.CLIVersion, sess.GitBranch, sess.WorkingDirectory, sess.StartTime, sess.EndTime,
		sess.Agent, sess.Date,
	)
	return err
}

func InsertSessionSimple(id, project, firstQuery, filePath, tool string, msgCount int) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO sessions (id, project, first_query, message_count, tool, file_path, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, id, project, firstQuery, msgCount, tool, filePath)
	return err
}

func UpdateSessionTokens(sessionID string, inputTokens, outputTokens, cacheRead, cacheWrite int, costUSD float64, model, provider string) error {
	_, err := db.Exec(`
		UPDATE sessions SET
			total_input_tokens = total_input_tokens + ?,
			total_output_tokens = total_output_tokens + ?,
			total_cache_read = total_cache_read + ?,
			total_cache_write = total_cache_write + ?,
			total_cost_usd = total_cost_usd + ?,
			model = COALESCE(NULLIF(?, ''), model),
			provider = COALESCE(NULLIF(?, ''), provider)
		WHERE id = ?
	`, inputTokens, outputTokens, cacheRead, cacheWrite, costUSD, model, provider, sessionID)
	return err
}

func GetRecentSessions(limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := db.Query(`
		SELECT id, project, first_query, message_count, tool, indexed_at,
		       model, provider, total_input_tokens, total_output_tokens, total_cost_usd
		FROM sessions
		ORDER BY indexed_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sessions []map[string]interface{}
	for rows.Next() {
		var id, project, firstQuery, tool, model, provider string
		var msgCount, inputTokens, outputTokens int
		var costUSD float64
		var indexedAt time.Time

		err := rows.Scan(&id, &project, &firstQuery, &msgCount, &tool, &indexedAt,
			&model, &provider, &inputTokens, &outputTokens, &costUSD)
		if err != nil {
			continue
		}

		sessions = append(sessions, map[string]interface{}{
			"id":           id,
			"project":      project,
			"firstQuery":   firstQuery,
			"messages":     msgCount,
			"tool":         tool,
			"indexedAt":    indexedAt,
			"model":        model,
			"provider":     provider,
			"inputTokens":  inputTokens,
			"outputTokens": outputTokens,
			"costUSD":      costUSD,
		})
	}

	return sessions, nil
}

func GetStats() (int, int, error) {
	var sessionCount, messageCount int

	err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
	if err != nil {
		return 0, 0, err
	}

	err = db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&messageCount)
	if err != nil {
		return 0, 0, err
	}

	return sessionCount, messageCount, nil
}
