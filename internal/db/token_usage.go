// token_usage.go tracks per-request token consumption and cost. Records are
// inserted per API call and the parent session's aggregate totals are updated
// atomically. Also provides stats aggregation by provider, tool, and model.
package db

import (
	"fmt"
	"log"
	"time"
)

// TokenUsage represents a single API request's token consumption and cost.
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

// TokenStats holds aggregate token usage across multiple API requests.
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

// GetTokenStats returns aggregate token usage across all providers, optionally
// filtered to the last N days (0 means all time).
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

// GetTokenStatsByProvider returns aggregate token usage grouped by provider,
// optionally filtered to the last N days (0 means all time).
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
			log.Printf("GetTokenStatsByProvider: rows.Scan error: %v", err)
			continue
		}
		result[provider] = stats
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetTokenStatsByProvider iteration error: %w", err)
	}

	return result, nil
}

// UsageStats holds aggregate token usage across all sessions.
type UsageStats struct {
	TotalInputTokens  int
	TotalOutputTokens int
	TotalCacheRead    int
	TotalCacheWrite   int
	TotalTokens       int
	TotalCostUSD      float64
	SessionCount      int
}

// GetUsageStats returns aggregate token usage computed from the sessions table.
func GetUsageStats() (UsageStats, error) {
	var s UsageStats
	err := db.QueryRow(`
		SELECT
			COALESCE(SUM(total_input_tokens), 0),
			COALESCE(SUM(total_output_tokens), 0),
			COALESCE(SUM(total_cache_read), 0),
			COALESCE(SUM(total_cache_write), 0),
			COALESCE(SUM(total_cost_usd), 0),
			COUNT(*)
		FROM sessions
	`).Scan(&s.TotalInputTokens, &s.TotalOutputTokens, &s.TotalCacheRead, &s.TotalCacheWrite, &s.TotalCostUSD, &s.SessionCount)
	if err != nil {
		return s, err
	}
	s.TotalTokens = s.TotalInputTokens + s.TotalOutputTokens
	return s, nil
}

// ToolUsageSummary holds aggregate token usage for a single tool.
type ToolUsageSummary struct {
	Sessions     int
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CostUSD      float64
}

// GetUsageStatsByTool returns aggregate token usage grouped by AI tool (e.g. claude-code, opencode).
func GetUsageStatsByTool() (map[string]ToolUsageSummary, error) {
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

	result := make(map[string]ToolUsageSummary)
	for rows.Next() {
		var tool string
		var s ToolUsageSummary
		if err := rows.Scan(&tool, &s.Sessions, &s.InputTokens, &s.OutputTokens, &s.CostUSD); err != nil {
			log.Printf("GetUsageStatsByTool: rows.Scan error: %v", err)
			continue
		}
		s.TotalTokens = s.InputTokens + s.OutputTokens
		result[tool] = s
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetUsageStatsByTool iteration error: %w", err)
	}

	return result, nil
}

// ModelUsageSummary holds aggregate token usage for a single model.
type ModelUsageSummary struct {
	Sessions     int
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CostUSD      float64
}

// GetUsageStatsByModel returns aggregate token usage grouped by model name.
func GetUsageStatsByModel() (map[string]ModelUsageSummary, error) {
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

	result := make(map[string]ModelUsageSummary)
	for rows.Next() {
		var model string
		var s ModelUsageSummary
		if err := rows.Scan(&model, &s.Sessions, &s.InputTokens, &s.OutputTokens, &s.CostUSD); err != nil {
			log.Printf("GetUsageStatsByModel: rows.Scan error: %v", err)
			continue
		}
		s.TotalTokens = s.InputTokens + s.OutputTokens
		result[model] = s
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetUsageStatsByModel iteration error: %w", err)
	}

	return result, nil
}

// SetAPICredential records that a provider's API key is configured and valid.
func SetAPICredential(provider string) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO api_credentials (provider, created_at, is_valid)
		VALUES (?, CURRENT_TIMESTAMP, 1)
	`, provider)
	return err
}

// GetAPICredentials returns the list of providers with valid API keys.
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
			log.Printf("GetAPICredentials: rows.Scan error: %v", err)
			continue
		}
		providers = append(providers, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetAPICredentials iteration error: %w", err)
	}
	return providers, nil
}
