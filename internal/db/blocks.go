// blocks.go implements 5-hour usage block analysis for rate limit tracking.
// Groups token usage entries into time-bounded blocks matching Claude's rate
// limit reset window, with per-block statistics and projections.
package db

import (
	"fmt"
	"log"
	"time"
)

// SessionDurationHours is Claude's rate limit reset window
const SessionDurationHours = 5

// SessionBlock represents a 5-hour usage window (matching Claude's rate limit reset)
type SessionBlock struct {
	ID            string
	StartTime     time.Time
	EndTime       time.Time // StartTime + 5 hours
	ActualEndTime time.Time // Last activity within block
	IsActive      bool      // Currently within this block's time window
	IsGap         bool      // No activity during this period
	InputTokens   int
	OutputTokens  int
	CacheRead     int
	CacheWrite    int
	CostUSD       float64
	Models        []string
	Providers     []string
	SessionCount  int
	MessageCount  int
	ResetTime     *time.Time // From Claude error messages (if available)
}

// TotalTokens returns sum of all token types
func (b SessionBlock) TotalTokens() int {
	return b.InputTokens + b.OutputTokens + b.CacheRead + b.CacheWrite
}

// DurationMinutes returns minutes of actual activity
func (b SessionBlock) DurationMinutes() float64 {
	if b.ActualEndTime.IsZero() || b.StartTime.IsZero() {
		return 0
	}
	return b.ActualEndTime.Sub(b.StartTime).Minutes()
}

// TokensPerMinute calculates burn rate
func (b SessionBlock) TokensPerMinute() float64 {
	mins := b.DurationMinutes()
	if mins <= 0 {
		return 0
	}
	return float64(b.TotalTokens()) / mins
}

// CostPerHour calculates hourly burn rate
func (b SessionBlock) CostPerHour() float64 {
	mins := b.DurationMinutes()
	if mins <= 0 {
		return 0
	}
	return (b.CostUSD / mins) * 60
}

// ProjectedTokens estimates tokens at end of 5-hour window
func (b SessionBlock) ProjectedTokens() int {
	if !b.IsActive {
		return b.TotalTokens()
	}
	rate := b.TokensPerMinute()
	remainingMins := time.Until(b.EndTime).Minutes()
	if remainingMins < 0 {
		remainingMins = 0
	}
	return b.TotalTokens() + int(rate*remainingMins)
}

// ProjectedCost estimates cost at end of 5-hour window
func (b SessionBlock) ProjectedCost() float64 {
	if !b.IsActive {
		return b.CostUSD
	}
	rate := b.CostPerHour() / 60 // per minute
	remainingMins := time.Until(b.EndTime).Minutes()
	if remainingMins < 0 {
		remainingMins = 0
	}
	return b.CostUSD + (rate * remainingMins)
}

// RemainingTime returns time left in block
func (b SessionBlock) RemainingTime() time.Duration {
	if !b.IsActive {
		return 0
	}
	remaining := time.Until(b.EndTime)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// UsageEntry represents a single token usage record for block identification
type UsageEntry struct {
	SessionID        string
	Timestamp        time.Time
	Model            string
	Provider         string
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	CostUSD          float64
}

// GetUsageEntries fetches token usage records for block identification.
// Falls back to the sessions table if the token_usage table is empty,
// which happens when data was indexed from session history.
func GetUsageEntries(days int) ([]UsageEntry, error) {
	query := `
		SELECT
			session_id, timestamp, model, provider,
			input_tokens, output_tokens, cache_read_tokens, cache_write_tokens,
			cost_usd
		FROM token_usage
	`
	if days > 0 {
		query += fmt.Sprintf(" WHERE timestamp >= datetime('now', '-%d days')", days)
	}
	query += " ORDER BY timestamp ASC"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []UsageEntry
	for rows.Next() {
		var e UsageEntry
		err := rows.Scan(
			&e.SessionID, &e.Timestamp, &e.Model, &e.Provider,
			&e.InputTokens, &e.OutputTokens, &e.CacheReadTokens, &e.CacheWriteTokens,
			&e.CostUSD,
		)
		if err != nil {
			log.Printf("getUsageEntries: rows.Scan error: %v", err)
			continue
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetUsageEntries iteration error: %w", err)
	}

	if len(entries) == 0 {
		return getUsageEntriesFromSessions(days)
	}

	return entries, nil
}

func getUsageEntriesFromSessions(days int) ([]UsageEntry, error) {
	query := `
		SELECT
			id,
			CASE
				WHEN start_time IS NOT NULL AND start_time NOT LIKE '0001%' THEN start_time
				ELSE indexed_at
			END as ts,
			model, provider,
			total_input_tokens, total_output_tokens,
			total_cache_read, total_cache_write,
			total_cost_usd
		FROM sessions
		WHERE total_input_tokens > 0 OR total_output_tokens > 0
	`
	if days > 0 {
		query += fmt.Sprintf(` AND CASE
			WHEN start_time IS NOT NULL AND start_time NOT LIKE '0001%%' THEN start_time
			ELSE indexed_at
		END >= datetime('now', '-%d days')`, days)
	}
	query += " ORDER BY ts ASC"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []UsageEntry
	for rows.Next() {
		var e UsageEntry
		var tsStr string
		err := rows.Scan(
			&e.SessionID, &tsStr, &e.Model, &e.Provider,
			&e.InputTokens, &e.OutputTokens, &e.CacheReadTokens, &e.CacheWriteTokens,
			&e.CostUSD,
		)
		if err != nil {
			log.Printf("getUsageEntriesFromSessions: rows.Scan error: %v", err)
			continue
		}
		if ts, parseErr := time.Parse("2006-01-02 15:04:05", tsStr); parseErr == nil {
			e.Timestamp = ts
		} else if ts, parseErr := time.Parse(time.RFC3339, tsStr); parseErr == nil {
			e.Timestamp = ts
		} else {
			continue // Skip entries with unparseable timestamps
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("getUsageEntriesFromSessions iteration error: %w", err)
	}

	return entries, nil
}

// floorToHour rounds a time down to the nearest hour
func floorToHour(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

// IdentifySessionBlocks groups usage entries into 5-hour windows
// This mirrors ccusage's _session-blocks.ts logic
func IdentifySessionBlocks(entries []UsageEntry) []SessionBlock {
	if len(entries) == 0 {
		return nil
	}

	var blocks []SessionBlock
	var currentBlock *SessionBlock
	modelSet := make(map[string]bool)
	providerSet := make(map[string]bool)
	sessionSet := make(map[string]bool)

	for _, entry := range entries {
		// Start a new block if:
		// 1. No current block exists
		// 2. 5 hours have elapsed since block start
		// 3. 5+ hour gap between entries
		needNewBlock := currentBlock == nil

		if currentBlock != nil {
			hoursSinceBlockStart := entry.Timestamp.Sub(currentBlock.StartTime).Hours()
			if hoursSinceBlockStart >= float64(SessionDurationHours) {
				needNewBlock = true
			}
		}

		if needNewBlock {
			// Save current block if exists
			if currentBlock != nil {
				currentBlock.Models = mapKeys(modelSet)
				currentBlock.Providers = mapKeys(providerSet)
				currentBlock.SessionCount = len(sessionSet)
				blocks = append(blocks, *currentBlock)
			}

			// Start new block
			blockStart := floorToHour(entry.Timestamp)
			currentBlock = &SessionBlock{
				ID:            fmt.Sprintf("block-%d", blockStart.Unix()),
				StartTime:     blockStart,
				EndTime:       blockStart.Add(time.Duration(SessionDurationHours) * time.Hour),
				ActualEndTime: entry.Timestamp,
				IsActive:      false,
				IsGap:         false,
			}
			modelSet = make(map[string]bool)
			providerSet = make(map[string]bool)
			sessionSet = make(map[string]bool)
		}

		// Accumulate into current block
		currentBlock.InputTokens += entry.InputTokens
		currentBlock.OutputTokens += entry.OutputTokens
		currentBlock.CacheRead += entry.CacheReadTokens
		currentBlock.CacheWrite += entry.CacheWriteTokens
		currentBlock.CostUSD += entry.CostUSD
		currentBlock.ActualEndTime = entry.Timestamp
		currentBlock.MessageCount++

		if entry.Model != "" {
			modelSet[entry.Model] = true
		}
		if entry.Provider != "" {
			providerSet[entry.Provider] = true
		}
		if entry.SessionID != "" {
			sessionSet[entry.SessionID] = true
		}
	}

	// Don't forget the last block
	if currentBlock != nil {
		currentBlock.Models = mapKeys(modelSet)
		currentBlock.Providers = mapKeys(providerSet)
		currentBlock.SessionCount = len(sessionSet)
		blocks = append(blocks, *currentBlock)
	}

	// Mark the active block (current time falls within block window)
	now := time.Now()
	for i := range blocks {
		if now.After(blocks[i].StartTime) && now.Before(blocks[i].EndTime) {
			blocks[i].IsActive = true
		}
	}

	return blocks
}

// mapKeys extracts keys from a map[string]bool
func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// GetSessionBlocks returns identified 5-hour blocks for a time range
func GetSessionBlocks(days int) ([]SessionBlock, error) {
	entries, err := GetUsageEntries(days)
	if err != nil {
		return nil, err
	}
	return IdentifySessionBlocks(entries), nil
}

// GetActiveBlock returns the current 5-hour block (if any activity exists)
func GetActiveBlock() (*SessionBlock, error) {
	blocks, err := GetSessionBlocks(1) // Last 24 hours should be enough
	if err != nil {
		return nil, err
	}

	for i := range blocks {
		if blocks[i].IsActive {
			return &blocks[i], nil
		}
	}

	return nil, nil // No active block
}

// GetRecentBlocks returns the last N blocks
func GetRecentBlocks(count int) ([]SessionBlock, error) {
	blocks, err := GetSessionBlocks(7) // Last week
	if err != nil {
		return nil, err
	}

	if len(blocks) <= count {
		return blocks, nil
	}

	// Return last N blocks
	return blocks[len(blocks)-count:], nil
}

// ToolUsageStats represents token usage from a specific tool
type ToolUsageStats struct {
	Tool         string
	Provider     string
	InputTokens  int
	OutputTokens int
	CacheRead    int
	CacheWrite   int
	TotalTokens  int
	SessionCount int
}

// GetUsageByToolInWindow returns token usage grouped by tool for a provider
// within a specific time window (e.g., current 5-hour block)
func GetUsageByToolInWindow(provider string, windowStart, windowEnd time.Time) ([]ToolUsageStats, error) {
	query := `
		SELECT
			tool,
			provider,
			COALESCE(SUM(total_input_tokens), 0),
			COALESCE(SUM(total_output_tokens), 0),
			COALESCE(SUM(total_cache_read), 0),
			COALESCE(SUM(total_cache_write), 0),
			COUNT(DISTINCT id)
		FROM sessions
		WHERE provider = ?
		AND (
			(start_time IS NOT NULL AND start_time NOT LIKE '0001%' AND start_time >= ? AND start_time <= ?)
			OR
			(start_time IS NULL OR start_time LIKE '0001%') AND indexed_at >= ? AND indexed_at <= ?
		)
		GROUP BY tool
		ORDER BY SUM(total_input_tokens) + SUM(total_output_tokens) DESC
	`

	rows, err := db.Query(query, provider, windowStart, windowEnd, windowStart, windowEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage by tool: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []ToolUsageStats
	for rows.Next() {
		var s ToolUsageStats
		err := rows.Scan(
			&s.Tool, &s.Provider,
			&s.InputTokens, &s.OutputTokens,
			&s.CacheRead, &s.CacheWrite,
			&s.SessionCount,
		)
		if err != nil {
			log.Printf("GetUsageByToolAndProvider: rows.Scan error: %v", err)
			continue
		}
		s.TotalTokens = s.InputTokens + s.OutputTokens + s.CacheRead + s.CacheWrite
		results = append(results, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetUsageByToolInWindow iteration error: %w", err)
	}

	return results, nil
}

// GetUsageByToolForActiveBlock returns per-tool usage for the current 5-hour block
func GetUsageByToolForActiveBlock(provider string) ([]ToolUsageStats, *SessionBlock, error) {
	block, err := GetActiveBlock()
	if err != nil {
		return nil, nil, err
	}
	if block == nil {
		return nil, nil, nil
	}

	stats, err := GetUsageByToolInWindow(provider, block.StartTime, block.EndTime)
	if err != nil {
		return nil, block, err
	}

	return stats, block, nil
}
