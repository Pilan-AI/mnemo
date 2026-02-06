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

func GetDB() *sql.DB {
	return db
}

type Message struct {
	ID               int64
	SessionID        string
	Project          string
	Role             string
	Content          string
	Timestamp        time.Time
	Tool             string
	Model            string
	Provider         string
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ReasoningTokens  int
	CostUSD          float64
	MessageUUID      string
	ParentUUID       string
	WorkingDirectory string
	Agent            string
	Date             string
}

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

type Project struct {
	ID           int64
	Path         string
	Name         string
	LastActivity time.Time
	Status       string
	UserEnabled  bool
	CreatedAt    time.Time
}

const (
	ProjectStatusActive   = "active"
	ProjectStatusInactive = "inactive"
	ProjectStatusArchived = "archived"
)

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

func InsertMessage(msg Message) error {
	result, err := db.Exec(`
		INSERT INTO messages (
			session_id, project, role, content, timestamp, tool,
			model, provider, input_tokens, output_tokens,
			cache_read_tokens, cache_write_tokens, reasoning_tokens, cost_usd,
			message_uuid, parent_uuid, working_directory, agent, date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		msg.SessionID, msg.Project, msg.Role, msg.Content, msg.Timestamp, msg.Tool,
		msg.Model, msg.Provider, msg.InputTokens, msg.OutputTokens,
		msg.CacheReadTokens, msg.CacheWriteTokens, msg.ReasoningTokens, msg.CostUSD,
		msg.MessageUUID, msg.ParentUUID, msg.WorkingDirectory, msg.Agent, msg.Date,
	)
	if err != nil {
		return err
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no rows inserted")
	}
	return nil
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

type TokenStats struct {
	TotalInputTokens  int
	TotalOutputTokens int
	TotalCacheRead    int
	TotalCacheWrite   int
	TotalTokens       int
	TotalCostUSD      float64
	SessionCount      int
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

func UpsertProject(path string, lastActivity time.Time) error {
	name := filepath.Base(path)
	_, err := db.Exec(`
		INSERT INTO projects (path, name, last_activity, status, user_enabled)
		VALUES (?, ?, ?, 'active', 1)
		ON CONFLICT(path) DO UPDATE SET
			last_activity = MAX(last_activity, excluded.last_activity),
			name = excluded.name
	`, path, name, lastActivity)
	return err
}

func GetProjects() ([]Project, error) {
	rows, err := db.Query(`
		SELECT id, path, name, last_activity, status, user_enabled, created_at
		FROM projects
		ORDER BY last_activity DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var projects []Project
	for rows.Next() {
		var p Project
		var userEnabled int
		err := rows.Scan(&p.ID, &p.Path, &p.Name, &p.LastActivity, &p.Status, &userEnabled, &p.CreatedAt)
		if err != nil {
			continue
		}
		p.UserEnabled = userEnabled == 1
		projects = append(projects, p)
	}
	return projects, nil
}

func GetProjectsByStatus(status string) ([]Project, error) {
	rows, err := db.Query(`
		SELECT id, path, name, last_activity, status, user_enabled, created_at
		FROM projects
		WHERE status = ?
		ORDER BY last_activity DESC
	`, status)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var projects []Project
	for rows.Next() {
		var p Project
		var userEnabled int
		err := rows.Scan(&p.ID, &p.Path, &p.Name, &p.LastActivity, &p.Status, &userEnabled, &p.CreatedAt)
		if err != nil {
			continue
		}
		p.UserEnabled = userEnabled == 1
		projects = append(projects, p)
	}
	return projects, nil
}

func GetEnabledProjects() ([]Project, error) {
	rows, err := db.Query(`
		SELECT id, path, name, last_activity, status, user_enabled, created_at
		FROM projects
		WHERE user_enabled = 1
		ORDER BY last_activity DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var projects []Project
	for rows.Next() {
		var p Project
		var userEnabled int
		err := rows.Scan(&p.ID, &p.Path, &p.Name, &p.LastActivity, &p.Status, &userEnabled, &p.CreatedAt)
		if err != nil {
			continue
		}
		p.UserEnabled = userEnabled == 1
		projects = append(projects, p)
	}
	return projects, nil
}

func SetProjectUserEnabled(path string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := db.Exec(`UPDATE projects SET user_enabled = ? WHERE path = ?`, enabledInt, path)
	return err
}

func ClassifyProjects() error {
	now := time.Now()
	activeCutoff := now.AddDate(0, 0, -60)
	inactiveCutoff := now.AddDate(0, 0, -90)

	_, err := db.Exec(`
		UPDATE projects SET status = CASE
			WHEN last_activity >= ? THEN 'active'
			WHEN last_activity >= ? THEN 'inactive'
			ELSE 'archived'
		END
	`, activeCutoff, inactiveCutoff)
	return err
}

func GetProjectsForOnboarding() (active []Project, inactive []Project, err error) {
	if err = ClassifyProjects(); err != nil {
		return nil, nil, err
	}

	active, err = GetProjectsByStatus(ProjectStatusActive)
	if err != nil {
		return nil, nil, err
	}

	inactive, err = GetProjectsByStatus(ProjectStatusInactive)
	if err != nil {
		return nil, nil, err
	}

	return active, inactive, nil
}

func AddProjectManually(path string) error {
	name := filepath.Base(path)
	_, err := db.Exec(`
		INSERT INTO projects (path, name, last_activity, status, user_enabled)
		VALUES (?, ?, CURRENT_TIMESTAMP, 'active', 1)
		ON CONFLICT(path) DO UPDATE SET
			status = 'active',
			user_enabled = 1
	`, path, name)
	return err
}

func DeleteProject(path string) error {
	_, err := db.Exec(`DELETE FROM projects WHERE path = ?`, path)
	return err
}

func PruneStaleProjects() (int, error) {
	result, err := db.Exec(`DELETE FROM projects WHERE path NOT IN (SELECT DISTINCT working_directory FROM sessions WHERE working_directory != '')`)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

func MergeProjects(oldPath, newPath string) (int, int, error) {
	sessionsResult, err := db.Exec(`
		UPDATE sessions SET 
			working_directory = ?,
			project = ?
		WHERE working_directory = ?
	`, newPath, filepath.Base(newPath), oldPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to update sessions: %w", err)
	}
	sessionsUpdated, _ := sessionsResult.RowsAffected()

	messagesResult, err := db.Exec(`
		UPDATE messages SET 
			working_directory = ?,
			project = ?
		WHERE working_directory = ?
	`, newPath, filepath.Base(newPath), oldPath)
	if err != nil {
		return int(sessionsUpdated), 0, fmt.Errorf("failed to update messages: %w", err)
	}
	messagesUpdated, _ := messagesResult.RowsAffected()

	_, _ = db.Exec(`DELETE FROM projects WHERE path = ?`, oldPath)

	newName := filepath.Base(newPath)
	_, _ = db.Exec(`
		INSERT INTO projects (path, name, last_activity, status, user_enabled)
		VALUES (?, ?, CURRENT_TIMESTAMP, 'active', 1)
		ON CONFLICT(path) DO UPDATE SET
			name = excluded.name,
			last_activity = MAX(last_activity, excluded.last_activity)
	`, newPath, newName)

	return int(sessionsUpdated), int(messagesUpdated), nil
}

func GetProjectByPath(path string) (*Project, error) {
	row := db.QueryRow(`
		SELECT id, path, name, last_activity, status, user_enabled, created_at
		FROM projects WHERE path = ?
	`, path)

	var p Project
	var userEnabled int
	err := row.Scan(&p.ID, &p.Path, &p.Name, &p.LastActivity, &p.Status, &userEnabled, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	p.UserEnabled = userEnabled == 1
	return &p, nil
}

// ============================================================================
// 5-Hour Session Blocks (ported from ccusage)
// ============================================================================

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
	remainingMins := b.EndTime.Sub(time.Now()).Minutes()
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
	remainingMins := b.EndTime.Sub(time.Now()).Minutes()
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
	remaining := b.EndTime.Sub(time.Now())
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

// GetUsageEntries fetches token usage records for block identification
// Falls back to sessions table if token_usage is empty
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
			continue
		}
		entries = append(entries, e)
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
			continue
		}
		if ts, parseErr := time.Parse("2006-01-02 15:04:05", tsStr); parseErr == nil {
			e.Timestamp = ts
		}
		entries = append(entries, e)
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
			continue
		}
		s.TotalTokens = s.InputTokens + s.OutputTokens + s.CacheRead + s.CacheWrite
		results = append(results, s)
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
