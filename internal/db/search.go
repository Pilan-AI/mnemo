package db

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// SearchResult holds a single FTS5 search match with BM25 ranking.
// Snippet contains the matched text with >>> and <<< delimiters for highlighting.
type SearchResult struct {
	SessionID string
	Project   string
	Role      string
	Content   string
	Snippet   string
	Rank      float64
	Tool      string
	Model     string
	Provider  string
}

// SessionMatch holds a session-level search result with aggregated scoring.
// This is the primary return type for the redesigned search system.
type SessionMatch struct {
	SessionID    string
	Project      string
	FirstQuery   string
	MessageCount int
	Tool         string
	StartTime    time.Time
	MatchCount   int
	BestRank     float64
	FinalScore   float64
	Snippet      string
	SnippetRole  string
}

// sanitizeFTS5Query strips FTS5 special characters to prevent query syntax errors.
// Without this, user queries containing ?, *, parentheses, etc. would cause
// "fts5: syntax error" from SQLite.
func sanitizeFTS5Query(query string) string {
	specialChars := []string{"?", "*", "(", ")", "^", ":", "+", "-", "\"", "'"}

	result := query
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, " ")
	}

	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}

	result = strings.TrimSpace(result)

	if result == "" {
		return "error"
	}

	return result
}

// Search performs a full-text search using FTS5 with BM25 ranking.
// Results include highlighted snippets with >>> <<< delimiters.
func Search(query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	safeQuery := sanitizeFTS5Query(query)

	rows, err := db.Query(`
		SELECT
			m.session_id,
			m.project,
			m.role,
			m.content,
			snippet(messages_fts, 0, '>>>', '<<<', '...', 256) as snippet,
			bm25(messages_fts) as rank,
			m.tool,
			m.model,
			m.provider
		FROM messages_fts
		JOIN messages m ON messages_fts.rowid = m.id
		WHERE messages_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, safeQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		err := rows.Scan(&r.SessionID, &r.Project, &r.Role, &r.Content, &r.Snippet, &r.Rank, &r.Tool, &r.Model, &r.Provider)
		if err != nil {
			continue
		}
		results = append(results, r)
	}

	return results, nil
}

// SearchGrouped performs a session-level search with intelligent ranking.
// Fetches message-level FTS5 results, groups by session in Go, then
// enriches with session metadata. Ranked by BM25 * temporal_decay * density_bonus.
func SearchGrouped(query string, limit int) ([]SessionMatch, error) {
	if limit <= 0 {
		limit = 5
	}

	safeQuery := sanitizeFTS5Query(query)

	// Fetch message-level results with higher limit for session grouping
	fetchLimit := limit * 10
	if fetchLimit < 50 {
		fetchLimit = 50
	}

	rows, err := db.Query(`
		SELECT m.session_id, m.role,
			   snippet(messages_fts, 0, '>>>', '<<<', '...', 64) as snippet,
			   bm25(messages_fts) as rank
		FROM messages_fts
		JOIN messages m ON messages_fts.rowid = m.id
		WHERE messages_fts MATCH ?
		ORDER BY rank
		LIMIT ?
	`, safeQuery, fetchLimit)
	if err != nil {
		return nil, fmt.Errorf("grouped search failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Group by session in Go
	type sessionData struct {
		matchCount      int
		bestRank        float64
		userHits        int
		bestSnippet     string
		bestSnippetRole string
		bestSnippetRank float64
		bestSnippetUser bool
	}
	sessions := make(map[string]*sessionData)
	var order []string

	for rows.Next() {
		var sessionID, role, snippet string
		var rank float64
		if err := rows.Scan(&sessionID, &role, &snippet, &rank); err != nil {
			continue
		}

		sd, exists := sessions[sessionID]
		if !exists {
			sd = &sessionData{bestRank: rank, bestSnippetRank: 999}
			sessions[sessionID] = sd
			order = append(order, sessionID)
		}
		sd.matchCount++
		if rank < sd.bestRank {
			sd.bestRank = rank
		}
		if role == "user" {
			sd.userHits++
		}

		// Prefer user messages for snippet, then best rank
		isUser := role == "user"
		if (isUser && !sd.bestSnippetUser) || (isUser == sd.bestSnippetUser && rank < sd.bestSnippetRank) {
			sd.bestSnippet = snippet
			sd.bestSnippetRole = role
			sd.bestSnippetRank = rank
			sd.bestSnippetUser = isUser
		}
	}

	// Build SessionMatch results with session metadata
	var matches []SessionMatch
	for _, sid := range order {
		sd := sessions[sid]
		sm := SessionMatch{
			SessionID:   sid,
			MatchCount:  sd.matchCount,
			BestRank:    sd.bestRank,
			Snippet:     sd.bestSnippet,
			SnippetRole: sd.bestSnippetRole,
		}

		// Fetch session metadata (scan time as string due to mixed timestamp formats)
		var timeStr string
		err := db.QueryRow(`
			SELECT project, COALESCE(first_query, ''), message_count, tool,
				   COALESCE(start_time, indexed_at, '')
			FROM sessions WHERE id = ?
		`, sid).Scan(&sm.Project, &sm.FirstQuery, &sm.MessageCount, &sm.Tool, &timeStr)
		if err != nil {
			continue
		}
		sm.StartTime = parseFlexibleTime(timeStr)

		// Composite scoring: BM25 + density bonus + temporal decay + user-match bonus
		daysOld := time.Since(sm.StartTime).Hours() / 24
		temporalDecay := math.Exp(-0.03 * daysOld) // 1.0 for today, ~0.4 at 30 days
		densityBonus := float64(sd.matchCount) * 0.05
		userBonus := float64(sd.userHits) * 0.1

		sm.FinalScore = (sd.bestRank - densityBonus - userBonus) * temporalDecay

		matches = append(matches, sm)
	}

	// Sort by FinalScore ascending (more negative = more relevant)
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].FinalScore < matches[j-1].FinalScore; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}

	if len(matches) > limit {
		matches = matches[:limit]
	}

	return matches, nil
}

// parseFlexibleTime handles both Go-formatted ("2026-02-07 01:01:01.381 +0000 UTC")
// and SQLite datetime ("2026-02-07 17:56:52") timestamps from the database.
func parseFlexibleTime(s string) time.Time {
	formats := []string{
		"2006-01-02 15:04:05.999 -0700 MST",
		"2006-01-02 15:04:05.999 +0000 UTC",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
