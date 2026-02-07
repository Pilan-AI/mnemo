// Package db provides the SQLite persistence layer for mnemo.
//
// All indexed sessions and messages are stored in ~/.mnemo/mnemo.db using
// pure-Go SQLite (modernc.org/sqlite, no CGO required). The schema includes:
//
//   - messages table with FTS5 virtual table for full-text search
//   - sessions table linking messages to projects and tools
//   - token_usage table for per-request cost tracking
//   - projects table for directory-based project management
//
// FTS5 triggers automatically keep the search index in sync with inserts,
// updates, and deletes on the messages table.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// GetDB returns the package-level database connection.
// Must call InitDB first.
func GetDB() *sql.DB {
	return db
}

// InitDB opens (or creates) the mnemo database at ~/.mnemo/mnemo.db and
// applies the schema and any pending migrations. Uses WAL mode for
// concurrent read access.
func InitDB() error {
	home, _ := os.UserHomeDir()
	mnemoDir := filepath.Join(home, ".mnemo")
	_ = os.MkdirAll(mnemoDir, 0755)

	dbPath := filepath.Join(mnemoDir, "mnemo.db")

	var err error
	db, err = sql.Open("sqlite", dbPath+"?_synchronous=NORMAL&_journal_mode=WAL&_cache_size=-10000&_temp_store=MEMORY")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		project TEXT NOT NULL,
		role TEXT NOT NULL,
		content TEXT NOT NULL,
		timestamp DATETIME,
		tool TEXT DEFAULT 'claude',
		model TEXT DEFAULT '',
		provider TEXT DEFAULT '',
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		cache_read_tokens INTEGER DEFAULT 0,
		cache_write_tokens INTEGER DEFAULT 0,
		cost_usd REAL DEFAULT 0.0,
		message_uuid TEXT DEFAULT '',
		parent_uuid TEXT DEFAULT '',
		working_directory TEXT DEFAULT ''
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
		content,
		project,
		session_id,
		content='messages',
		content_rowid='id'
	);

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

	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		project TEXT NOT NULL,
		first_query TEXT,
		message_count INTEGER DEFAULT 0,
		tool TEXT DEFAULT 'claude',
		file_path TEXT,
		indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		model TEXT DEFAULT '',
		provider TEXT DEFAULT '',
		total_input_tokens INTEGER DEFAULT 0,
		total_output_tokens INTEGER DEFAULT 0,
		total_cache_read INTEGER DEFAULT 0,
		total_cache_write INTEGER DEFAULT 0,
		total_cost_usd REAL DEFAULT 0.0,
		cli_version TEXT DEFAULT '',
		git_branch TEXT DEFAULT '',
		working_directory TEXT DEFAULT '',
		start_time DATETIME,
		end_time DATETIME
	);

	CREATE TABLE IF NOT EXISTS token_usage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		model TEXT NOT NULL,
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		cache_read_tokens INTEGER DEFAULT 0,
		cache_write_tokens INTEGER DEFAULT 0,
		total_tokens INTEGER DEFAULT 0,
		cost_usd REAL DEFAULT 0.0,
		provider TEXT DEFAULT 'anthropic',
		FOREIGN KEY (session_id) REFERENCES sessions(id)
	);

	CREATE TABLE IF NOT EXISTS api_credentials (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider TEXT NOT NULL UNIQUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_used DATETIME,
		is_valid INTEGER DEFAULT 1
	);

	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
	CREATE INDEX IF NOT EXISTS idx_messages_project ON messages(project);
	CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);
	CREATE INDEX IF NOT EXISTS idx_messages_model ON messages(model);
	CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project);
	CREATE INDEX IF NOT EXISTS idx_sessions_tool ON sessions(tool);
	CREATE INDEX IF NOT EXISTS idx_sessions_model ON sessions(model);
	CREATE INDEX IF NOT EXISTS idx_token_usage_session ON token_usage(session_id);
	CREATE INDEX IF NOT EXISTS idx_token_usage_timestamp ON token_usage(timestamp);

	CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT UNIQUE NOT NULL,
		name TEXT,
		last_activity DATETIME,
		status TEXT DEFAULT 'active',
		user_enabled INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
	CREATE INDEX IF NOT EXISTS idx_projects_last_activity ON projects(last_activity);
	`

	_, err = db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	if err := runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// runMigrations applies schema additions idempotently.
// Each ALTER TABLE is fire-and-forget — if the column already exists, the
// error is silently ignored. This avoids needing a migration version table.
func runMigrations() error {
	migrations := []string{
		"ALTER TABLE messages ADD COLUMN model TEXT DEFAULT ''",
		"ALTER TABLE messages ADD COLUMN provider TEXT DEFAULT ''",
		"ALTER TABLE messages ADD COLUMN input_tokens INTEGER DEFAULT 0",
		"ALTER TABLE messages ADD COLUMN output_tokens INTEGER DEFAULT 0",
		"ALTER TABLE messages ADD COLUMN cache_read_tokens INTEGER DEFAULT 0",
		"ALTER TABLE messages ADD COLUMN cache_write_tokens INTEGER DEFAULT 0",
		"ALTER TABLE messages ADD COLUMN cost_usd REAL DEFAULT 0.0",
		"ALTER TABLE messages ADD COLUMN message_uuid TEXT DEFAULT ''",
		"ALTER TABLE messages ADD COLUMN parent_uuid TEXT DEFAULT ''",
		"ALTER TABLE messages ADD COLUMN working_directory TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN provider TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN total_cache_read INTEGER DEFAULT 0",
		"ALTER TABLE sessions ADD COLUMN total_cache_write INTEGER DEFAULT 0",
		"ALTER TABLE sessions ADD COLUMN cli_version TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN git_branch TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN working_directory TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN start_time DATETIME",
		"ALTER TABLE sessions ADD COLUMN end_time DATETIME",
		"ALTER TABLE token_usage ADD COLUMN cache_read_tokens INTEGER DEFAULT 0",
		"ALTER TABLE token_usage ADD COLUMN cache_write_tokens INTEGER DEFAULT 0",
		"ALTER TABLE messages ADD COLUMN reasoning_tokens INTEGER DEFAULT 0",
		"ALTER TABLE messages ADD COLUMN agent TEXT DEFAULT ''",
		"ALTER TABLE messages ADD COLUMN date TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN total_reasoning_tokens INTEGER DEFAULT 0",
		"ALTER TABLE sessions ADD COLUMN agent TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN date TEXT DEFAULT ''",
	}

	for _, migration := range migrations {
		_, _ = db.Exec(migration)
	}

	return nil
}

func CloseDB() {
	if db != nil {
		_ = db.Close()
	}
}

func OpenReadOnlySQLite(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path+"?mode=ro")
}

func ClearIndex() error {
	_, err := db.Exec(`
		DELETE FROM messages;
		DELETE FROM messages_fts;
		DELETE FROM sessions;
		DELETE FROM token_usage;
	`)
	return err
}
