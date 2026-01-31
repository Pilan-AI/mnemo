package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// Message represents a searchable message
type Message struct {
	ID        int64
	SessionID string
	Project   string
	Role      string
	Content   string
	Timestamp time.Time
	Tool      string
}

// SearchResult represents a search match
type SearchResult struct {
	SessionID string
	Project   string
	Role      string
	Content   string
	Snippet   string
	Rank      float64
	Tool      string
}

// InitDB initializes the SQLite database with FTS5
func InitDB() error {
	home, _ := os.UserHomeDir()
	mnemoDir := filepath.Join(home, ".mnemo")
	_ = os.MkdirAll(mnemoDir, 0755)

	dbPath := filepath.Join(mnemoDir, "mnemo.db")

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Create tables
	schema := `
	-- Main messages table
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		project TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		timestamp DATETIME,
		tool TEXT DEFAULT 'claude'
	);

	-- FTS5 virtual table for full-text search
	CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
		content,
		project,
		session_id,
		content='messages',
		content_rowid='id'
	);

	-- Triggers to keep FTS in sync
	CREATE TRIGGER IF NOT EXISTS messages_ai AFTER INSERT ON messages BEGIN
		INSERT INTO messages_fts(rowid, content, project, session_id)
		VALUES (new.id, new.content, new.project, new.session_id);
	END;

	CREATE TRIGGER IF NOT EXISTS messages_ad AFTER DELETE ON messages BEGIN
		INSERT INTO messages_fts(messages_fts, rowid, content, project, session_id)
		VALUES ('delete', old.id, old.content, old.project, old.session_id);
	END;

	CREATE TRIGGER IF NOT EXISTS messages_au AFTER UPDATE ON messages BEGIN
		INSERT INTO messages_fts(messages_fts, rowid, content, project, session_id)
		VALUES ('delete', old.id, old.content, old.project, old.session_id);
		INSERT INTO messages_fts(rowid, content, project, session_id)
		VALUES (new.id, new.content, new.project, new.session_id);
	END;

	-- Sessions metadata table
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		project TEXT NOT NULL,
		first_query TEXT,
		message_count INTEGER DEFAULT 0,
		tool TEXT DEFAULT 'claude',
		file_path TEXT,
		indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	-- Index for faster lookups
	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
	CREATE INDEX IF NOT EXISTS idx_messages_project ON messages(project);
	CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project);
	`

	_, err = db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// CloseDB closes the database connection
func CloseDB() {
	if db != nil {
		_ = db.Close()
	}
}

// ClearIndex removes all indexed data
func ClearIndex() error {
	_, err := db.Exec(`
		DELETE FROM messages;
		DELETE FROM messages_fts;
		DELETE FROM sessions;
	`)
	return err
}

// InsertMessage adds a message to the database
func InsertMessage(msg Message) error {
	result, err := db.Exec(`
		INSERT INTO messages (session_id, project, role, content, timestamp, tool)
		VALUES (?, ?, ?, ?, ?, ?)
	`, msg.SessionID, msg.Project, msg.Role, msg.Content, msg.Timestamp, msg.Tool)
	if err != nil {
		return err
	}

	// Verify insert actually worked
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no rows inserted")
	}
	return nil
}

// InsertSession adds or updates session metadata
func InsertSession(id, project, firstQuery, filePath, tool string, msgCount int) error {
	_, err := db.Exec(`
		INSERT OR REPLACE INTO sessions (id, project, first_query, message_count, tool, file_path, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`, id, project, firstQuery, msgCount, tool, filePath)
	return err
}

// sanitizeFTS5Query escapes special characters for FTS5
func sanitizeFTS5Query(query string) string {
	// FTS5 special characters that need escaping
	specialChars := []string{"?", "*", "(", ")", "^", ":", "+", "-", "\"", "'"}

	result := query
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, " ")
	}

	// Remove extra spaces
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}

	result = strings.TrimSpace(result)

	// If empty after sanitization, return original without special chars
	if result == "" {
		return "error"
	}

	return result
}

// Search performs full-text search using FTS5
func Search(query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 10
	}

	// Sanitize query for FTS5
	safeQuery := sanitizeFTS5Query(query)

	// Use FTS5 MATCH with BM25 ranking
	rows, err := db.Query(`
		SELECT
			m.session_id,
			m.project,
			m.role,
			m.content,
			snippet(messages_fts, 0, '>>>', '<<<', '...', 256) as snippet,
			bm25(messages_fts) as rank
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
		err := rows.Scan(&r.SessionID, &r.Project, &r.Role, &r.Content, &r.Snippet, &r.Rank)
		if err != nil {
			continue
		}
		results = append(results, r)
	}

	return results, nil
}

// GetRecentSessions returns recent sessions
func GetRecentSessions(limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := db.Query(`
		SELECT id, project, first_query, message_count, tool, indexed_at
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
		var id, project, firstQuery, tool string
		var msgCount int
		var indexedAt time.Time

		err := rows.Scan(&id, &project, &firstQuery, &msgCount, &tool, &indexedAt)
		if err != nil {
			continue
		}

		sessions = append(sessions, map[string]interface{}{
			"id":         id,
			"project":    project,
			"firstQuery": firstQuery,
			"messages":   msgCount,
			"tool":       tool,
			"indexedAt":  indexedAt,
		})
	}

	return sessions, nil
}

// GetStats returns indexing statistics
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
