package db

import (
	"fmt"
	"time"
)

type TokenUsage struct {
	SessionID        string
	Model            string
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	TotalTokens      int
	CostUSD          float64
	Provider         string
	Timestamp        time.Time
}

type TokenStats struct {
	TotalInputTokens  int
	TotalOutputTokens int
	TotalCacheRead    int
	TotalCacheWrite   int
	TotalTokens       int
	TotalCostUSD      float64
	SessionCount      int
}

// InsertTokenUsage records a single API request's token usage and updates
// the parent session's aggregate totals.
func InsertTokenUsage(usage TokenUsage) error {
	_, err := db.Exec(`
		INSERT INTO token_usage (
			session_id, model, input_tokens, output_tokens,
			cache_read_tokens, cache_write_tokens, total_tokens,
			cost_usd, provider, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		usage.SessionID, usage.Model, usage.InputTokens, usage.OutputTokens,
		usage.CacheReadTokens, usage.CacheWriteTokens, usage.TotalTokens,
		usage.CostUSD, usage.Provider, usage.Timestamp,
	)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		UPDATE sessions
		SET total_input_tokens = total_input_tokens + ?,
		    total_output_tokens = total_output_tokens + ?,
		    total_cache_read = total_cache_read + ?,
		    total_cache_write = total_cache_write + ?,
		    total_cost_usd = total_cost_usd + ?,
		    model = COALESCE(NULLIF(?, ''), model),
		    provider = COALESCE(NULLIF(?, ''), provider)
		WHERE id = ?
	`, usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens, usage.CacheWriteTokens,
		usage.CostUSD, usage.Model, usage.Provider, usage.SessionID)

	return err
}

func GetTokenStats(days int) (TokenStats, error) {
	var stats TokenStats

	query := `
		SELECT
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cache_read_tokens), 0),
			COALESCE(SUM(cache_write_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(cost_usd), 0),
			COUNT(DISTINCT session_id)
		FROM token_usage
	`

	if days > 0 {
		query += fmt.Sprintf(" WHERE timestamp >= datetime('now', '-%d days')", days)
	}

	err := db.QueryRow(query).Scan(
		&stats.TotalInputTokens,
		&stats.TotalOutputTokens,
		&stats.TotalCacheRead,
		&stats.TotalCacheWrite,
		&stats.TotalTokens,
		&stats.TotalCostUSD,
		&stats.SessionCount,
	)

	return stats, err
}

func GetTokenStatsByProvider(days int) (map[string]TokenStats, error) {
	query := `
		SELECT
			provider,
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(cache_read_tokens), 0),
			COALESCE(SUM(cache_write_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(cost_usd), 0),
			COUNT(DISTINCT session_id)
		FROM token_usage
	`

	if days > 0 {
		query += fmt.Sprintf(" WHERE timestamp >= datetime('now', '-%d days')", days)
	}
	query += " GROUP BY provider"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]TokenStats)
	for rows.Next() {
		var provider string
		var stats TokenStats
		err := rows.Scan(&provider, &stats.TotalInputTokens, &stats.TotalOutputTokens,
			&stats.TotalCacheRead, &stats.TotalCacheWrite, &stats.TotalTokens,
			&stats.TotalCostUSD, &stats.SessionCount)
		if err != nil {
			continue
		}
		result[provider] = stats
	}

	return result, nil
}

func GetUsageStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var totalInput, totalOutput, totalCacheRead, totalCacheWrite int
	var totalCost float64
	var sessionCount int

	err := db.QueryRow(`
		SELECT
			COALESCE(SUM(total_input_tokens), 0),
			COALESCE(SUM(total_output_tokens), 0),
			COALESCE(SUM(total_cache_read), 0),
			COALESCE(SUM(total_cache_write), 0),
			COALESCE(SUM(total_cost_usd), 0),
			COUNT(*)
		FROM sessions
	`).Scan(&totalInput, &totalOutput, &totalCacheRead, &totalCacheWrite, &totalCost, &sessionCount)
	if err != nil {
		return nil, err
	}

	stats["totalInputTokens"] = totalInput
	stats["totalOutputTokens"] = totalOutput
	stats["totalCacheReadTokens"] = totalCacheRead
	stats["totalCacheWriteTokens"] = totalCacheWrite
	stats["totalCostUSD"] = totalCost
	stats["sessionCount"] = sessionCount
	stats["totalTokens"] = totalInput + totalOutput

	return stats, nil
}

func GetUsageStatsByTool() (map[string]map[string]interface{}, error) {
	rows, err := db.Query(`
		SELECT
			tool,
			COUNT(*) as sessions,
			COALESCE(SUM(total_input_tokens), 0),
			COALESCE(SUM(total_output_tokens), 0),
			COALESCE(SUM(total_cost_usd), 0)
		FROM sessions
		GROUP BY tool
		ORDER BY sessions DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]map[string]interface{})
	for rows.Next() {
		var tool string
		var sessions, inputTokens, outputTokens int
		var cost float64
		if err := rows.Scan(&tool, &sessions, &inputTokens, &outputTokens, &cost); err != nil {
			continue
		}
		result[tool] = map[string]interface{}{
			"sessions":     sessions,
			"inputTokens":  inputTokens,
			"outputTokens": outputTokens,
			"totalTokens":  inputTokens + outputTokens,
			"costUSD":      cost,
		}
	}

	return result, nil
}

func GetUsageStatsByModel() (map[string]map[string]interface{}, error) {
	rows, err := db.Query(`
		SELECT
			COALESCE(NULLIF(model, ''), 'unknown') as model,
			COUNT(*) as sessions,
			COALESCE(SUM(total_input_tokens), 0),
			COALESCE(SUM(total_output_tokens), 0),
			COALESCE(SUM(total_cost_usd), 0)
		FROM sessions
		WHERE model != ''
		GROUP BY model
		ORDER BY sessions DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]map[string]interface{})
	for rows.Next() {
		var model string
		var sessions, inputTokens, outputTokens int
		var cost float64
		if err := rows.Scan(&model, &sessions, &inputTokens, &outputTokens, &cost); err != nil {
			continue
		}
		result[model] = map[string]interface{}{
			"sessions":     sessions,
			"inputTokens":  inputTokens,
			"outputTokens": outputTokens,
			"totalTokens":  inputTokens + outputTokens,
			"costUSD":      cost,
		}
	}

	return result, nil
}

func SetAPICredential(provider string) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO api_credentials (provider, created_at, is_valid)
		VALUES (?, CURRENT_TIMESTAMP, 1)
	`, provider)
	return err
}

func GetAPICredentials() ([]string, error) {
	rows, err := db.Query(`SELECT provider FROM api_credentials WHERE is_valid = 1`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var providers []string
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			continue
		}
		providers = append(providers, provider)
	}
	return providers, nil
}
