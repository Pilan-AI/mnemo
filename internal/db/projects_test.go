package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) func() {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "mnemo-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	var openErr error
	db, openErr = sql.Open("sqlite", dbPath)
	if openErr != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to open database: %v", openErr)
	}
	db.SetMaxOpenConns(1)

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
		working_directory TEXT DEFAULT '',
		reasoning_tokens INTEGER DEFAULT 0,
		agent TEXT DEFAULT '',
		date TEXT DEFAULT ''
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
		end_time DATETIME,
		total_reasoning_tokens INTEGER DEFAULT 0,
		agent TEXT DEFAULT '',
		date TEXT DEFAULT ''
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

	CREATE TABLE IF NOT EXISTS projects (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT UNIQUE NOT NULL,
		name TEXT,
		last_activity DATETIME,
		status TEXT DEFAULT 'active',
		user_enabled INTEGER DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
	CREATE INDEX IF NOT EXISTS idx_messages_project ON messages(project);
	CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project);
	CREATE INDEX IF NOT EXISTS idx_sessions_tool ON sessions(tool);
	CREATE INDEX IF NOT EXISTS idx_sessions_start_time ON sessions(start_time);
	CREATE INDEX IF NOT EXISTS idx_sessions_indexed_at ON sessions(indexed_at);
	CREATE INDEX IF NOT EXISTS idx_sessions_working_directory ON sessions(working_directory);
	CREATE INDEX IF NOT EXISTS idx_token_usage_session ON token_usage(session_id);
	CREATE INDEX IF NOT EXISTS idx_token_usage_timestamp ON token_usage(timestamp);
	CREATE INDEX IF NOT EXISTS idx_token_usage_provider ON token_usage(provider);
	CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
	CREATE INDEX IF NOT EXISTS idx_projects_last_activity ON projects(last_activity);
	`

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to create schema: %v", err)
	}

	return func() {
		_ = db.Close()
		_ = os.RemoveAll(tmpDir)
	}
}

func TestUpsertProject(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	tests := []struct {
		name         string
		path         string
		lastActivity time.Time
		wantErr      bool
	}{
		{
			name:         "insert new project",
			path:         "/Users/test/Projects/foo",
			lastActivity: time.Now(),
			wantErr:      false,
		},
		{
			name:         "insert another project",
			path:         "/Users/test/Projects/bar",
			lastActivity: time.Now().Add(-24 * time.Hour),
			wantErr:      false,
		},
		{
			name:         "upsert existing project with newer activity",
			path:         "/Users/test/Projects/foo",
			lastActivity: time.Now().Add(24 * time.Hour),
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UpsertProject(tt.path, tt.lastActivity)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpsertProject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	projects, err := GetProjects()
	if err != nil {
		t.Fatalf("GetProjects() error = %v", err)
	}
	if len(projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(projects))
	}
}

func TestGetProjects(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	_ = UpsertProject("/path/a", now)
	_ = UpsertProject("/path/b", now.Add(-1*time.Hour))
	_ = UpsertProject("/path/c", now.Add(-2*time.Hour))

	projects, err := GetProjects()
	if err != nil {
		t.Fatalf("GetProjects() error = %v", err)
	}

	if len(projects) != 3 {
		t.Errorf("expected 3 projects, got %d", len(projects))
	}

	if projects[0].Path != "/path/a" {
		t.Errorf("expected first project to be /path/a (most recent), got %s", projects[0].Path)
	}
}

func TestGetProjectsByStatus(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	_ = UpsertProject("/active1", now)
	_ = UpsertProject("/active2", now.Add(-30*24*time.Hour))

	_, _ = db.Exec("UPDATE projects SET status = 'inactive' WHERE path = ?", "/active2")

	active, err := GetProjectsByStatus(ProjectStatusActive)
	if err != nil {
		t.Fatalf("GetProjectsByStatus(active) error = %v", err)
	}
	if len(active) != 1 {
		t.Errorf("expected 1 active project, got %d", len(active))
	}

	inactive, err := GetProjectsByStatus(ProjectStatusInactive)
	if err != nil {
		t.Fatalf("GetProjectsByStatus(inactive) error = %v", err)
	}
	if len(inactive) != 1 {
		t.Errorf("expected 1 inactive project, got %d", len(inactive))
	}
}

func TestGetEnabledProjects(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	_ = UpsertProject("/enabled1", now)
	_ = UpsertProject("/enabled2", now)
	_ = UpsertProject("/disabled1", now)

	_ = SetProjectUserEnabled("/disabled1", false)

	enabled, err := GetEnabledProjects()
	if err != nil {
		t.Fatalf("GetEnabledProjects() error = %v", err)
	}

	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled projects, got %d", len(enabled))
	}

	for _, p := range enabled {
		if p.Path == "/disabled1" {
			t.Error("disabled project should not be in enabled list")
		}
	}
}

func TestSetProjectUserEnabled(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = UpsertProject("/test/project", time.Now())

	err := SetProjectUserEnabled("/test/project", false)
	if err != nil {
		t.Fatalf("SetProjectUserEnabled(false) error = %v", err)
	}

	p, err := GetProjectByPath("/test/project")
	if err != nil {
		t.Fatalf("GetProjectByPath() error = %v", err)
	}
	if p.UserEnabled {
		t.Error("expected UserEnabled to be false")
	}

	err = SetProjectUserEnabled("/test/project", true)
	if err != nil {
		t.Fatalf("SetProjectUserEnabled(true) error = %v", err)
	}

	p, err = GetProjectByPath("/test/project")
	if err != nil {
		t.Fatalf("GetProjectByPath() error = %v", err)
	}
	if !p.UserEnabled {
		t.Error("expected UserEnabled to be true")
	}
}

func TestClassifyProjects(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()

	_ = UpsertProject("/recent", now.Add(-10*24*time.Hour))
	_ = UpsertProject("/active-boundary", now.Add(-59*24*time.Hour))
	_ = UpsertProject("/inactive", now.Add(-75*24*time.Hour))
	_ = UpsertProject("/inactive-boundary", now.Add(-89*24*time.Hour))
	_ = UpsertProject("/archived", now.Add(-100*24*time.Hour))
	_ = UpsertProject("/very-old", now.Add(-365*24*time.Hour))

	err := ClassifyProjects()
	if err != nil {
		t.Fatalf("ClassifyProjects() error = %v", err)
	}

	tests := []struct {
		path           string
		expectedStatus string
	}{
		{"/recent", ProjectStatusActive},
		{"/active-boundary", ProjectStatusActive},
		{"/inactive", ProjectStatusInactive},
		{"/inactive-boundary", ProjectStatusInactive},
		{"/archived", ProjectStatusArchived},
		{"/very-old", ProjectStatusArchived},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			p, err := GetProjectByPath(tt.path)
			if err != nil {
				t.Fatalf("GetProjectByPath(%s) error = %v", tt.path, err)
			}
			if p.Status != tt.expectedStatus {
				t.Errorf("path %s: expected status %s, got %s", tt.path, tt.expectedStatus, p.Status)
			}
		})
	}
}

func TestClassifyProjectsBoundaries(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()

	_ = UpsertProject("/exactly-60-days", now.Add(-60*24*time.Hour))
	_ = UpsertProject("/exactly-90-days", now.Add(-90*24*time.Hour))

	err := ClassifyProjects()
	if err != nil {
		t.Fatalf("ClassifyProjects() error = %v", err)
	}

	p60, _ := GetProjectByPath("/exactly-60-days")
	if p60.Status != ProjectStatusInactive {
		t.Errorf("60-day boundary: expected inactive, got %s", p60.Status)
	}

	p90, _ := GetProjectByPath("/exactly-90-days")
	if p90.Status != ProjectStatusArchived {
		t.Errorf("90-day boundary: expected archived, got %s", p90.Status)
	}
}

func TestGetProjectsForOnboarding(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()

	_ = UpsertProject("/active1", now.Add(-10*24*time.Hour))
	_ = UpsertProject("/active2", now.Add(-30*24*time.Hour))
	_ = UpsertProject("/inactive1", now.Add(-70*24*time.Hour))
	_ = UpsertProject("/inactive2", now.Add(-80*24*time.Hour))
	_ = UpsertProject("/archived1", now.Add(-100*24*time.Hour))

	active, inactive, err := GetProjectsForOnboarding()
	if err != nil {
		t.Fatalf("GetProjectsForOnboarding() error = %v", err)
	}

	if len(active) != 2 {
		t.Errorf("expected 2 active projects, got %d", len(active))
	}

	if len(inactive) != 2 {
		t.Errorf("expected 2 inactive projects, got %d", len(inactive))
	}
}

func TestAddProjectManually(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	err := AddProjectManually("/manual/project")
	if err != nil {
		t.Fatalf("AddProjectManually() error = %v", err)
	}

	p, err := GetProjectByPath("/manual/project")
	if err != nil {
		t.Fatalf("GetProjectByPath() error = %v", err)
	}

	if p.Name != "project" {
		t.Errorf("expected name 'project', got %s", p.Name)
	}
	if p.Status != "active" {
		t.Errorf("expected status 'active', got %s", p.Status)
	}
	if !p.UserEnabled {
		t.Error("expected UserEnabled to be true")
	}

	_, _ = db.Exec("UPDATE projects SET status = 'archived', user_enabled = 0 WHERE path = ?", "/manual/project")

	err = AddProjectManually("/manual/project")
	if err != nil {
		t.Fatalf("AddProjectManually() second call error = %v", err)
	}

	p, _ = GetProjectByPath("/manual/project")
	if p.Status != "active" {
		t.Errorf("expected status to be reset to 'active', got %s", p.Status)
	}
	if !p.UserEnabled {
		t.Error("expected UserEnabled to be reset to true")
	}
}

func TestDeleteProject(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = UpsertProject("/to-delete", time.Now())
	_ = UpsertProject("/to-keep", time.Now())

	err := DeleteProject("/to-delete")
	if err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}

	projects, _ := GetProjects()
	if len(projects) != 1 {
		t.Errorf("expected 1 project after delete, got %d", len(projects))
	}
	if projects[0].Path != "/to-keep" {
		t.Errorf("expected /to-keep to remain, got %s", projects[0].Path)
	}

	err = DeleteProject("/nonexistent")
	if err != nil {
		t.Errorf("DeleteProject on nonexistent path should not error, got %v", err)
	}
}

func TestGetProjectByPath(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	_ = UpsertProject("/find/me", now)

	p, err := GetProjectByPath("/find/me")
	if err != nil {
		t.Fatalf("GetProjectByPath() error = %v", err)
	}
	if p.Path != "/find/me" {
		t.Errorf("expected path /find/me, got %s", p.Path)
	}
	if p.Name != "me" {
		t.Errorf("expected name 'me', got %s", p.Name)
	}

	_, err = GetProjectByPath("/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestGetProjectsForOnboardingErrors(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	_ = UpsertProject("/project1", now.Add(-10*24*time.Hour))
	_ = UpsertProject("/project2", now.Add(-70*24*time.Hour))

	active, inactive, err := GetProjectsForOnboarding()
	if err != nil {
		t.Fatalf("GetProjectsForOnboarding() error = %v", err)
	}

	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}
	if len(inactive) != 1 {
		t.Errorf("expected 1 inactive, got %d", len(inactive))
	}
}

func TestGetProjectsEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	projects, err := GetProjects()
	if err != nil {
		t.Fatalf("GetProjects() error = %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestGetProjectsByStatusEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	projects, err := GetProjectsByStatus("nonexistent")
	if err != nil {
		t.Fatalf("GetProjectsByStatus() error = %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestGetEnabledProjectsEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	projects, err := GetEnabledProjects()
	if err != nil {
		t.Fatalf("GetEnabledProjects() error = %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestPruneStaleProjectsEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	pruned, err := PruneStaleProjects()
	if err != nil {
		t.Fatalf("PruneStaleProjects() error = %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 pruned, got %d", pruned)
	}
}

func TestMergeProjectsUpdatesProjectName(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	oldPath := "/old/path/oldname"
	newPath := "/new/path/newname"

	_, _ = db.Exec(`
		INSERT INTO sessions (id, project, working_directory, first_query, message_count, tool)
		VALUES ('sess1', 'oldname', ?, 'test', 1, 'claude')
	`, oldPath)

	_, _, err := MergeProjects(oldPath, newPath)
	if err != nil {
		t.Fatalf("MergeProjects() error = %v", err)
	}

	var projectName string
	_ = db.QueryRow("SELECT project FROM sessions WHERE id = 'sess1'").Scan(&projectName)
	if projectName != "newname" {
		t.Errorf("expected project name 'newname', got %s", projectName)
	}
}

func TestMergeProjects(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	oldPath := "/old/location/myproject"
	newPath := "/new/location/myproject"

	_, _ = db.Exec(`
		INSERT INTO sessions (id, project, working_directory, first_query, message_count, tool)
		VALUES 
			('sess1', 'myproject', ?, 'hello', 5, 'claude'),
			('sess2', 'myproject', ?, 'world', 3, 'claude'),
			('sess3', 'other', '/other/path', 'test', 1, 'claude')
	`, oldPath, oldPath)

	_, _ = db.Exec(`
		INSERT INTO messages (session_id, project, working_directory, role, content)
		VALUES 
			('sess1', 'myproject', ?, 'user', 'message 1'),
			('sess1', 'myproject', ?, 'assistant', 'response 1'),
			('sess2', 'myproject', ?, 'user', 'message 2'),
			('sess3', 'other', '/other/path', 'user', 'unrelated')
	`, oldPath, oldPath, oldPath)

	_ = UpsertProject(oldPath, time.Now())

	sessionsUpdated, messagesUpdated, err := MergeProjects(oldPath, newPath)
	if err != nil {
		t.Fatalf("MergeProjects() error = %v", err)
	}

	if sessionsUpdated != 2 {
		t.Errorf("expected 2 sessions updated, got %d", sessionsUpdated)
	}
	if messagesUpdated != 3 {
		t.Errorf("expected 3 messages updated, got %d", messagesUpdated)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE working_directory = ?", newPath).Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 sessions with new path, got %d", count)
	}

	_ = db.QueryRow("SELECT COUNT(*) FROM messages WHERE working_directory = ?", newPath).Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 messages with new path, got %d", count)
	}

	_ = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE working_directory = ?", oldPath).Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 sessions with old path, got %d", count)
	}

	_, err = GetProjectByPath(oldPath)
	if err == nil {
		t.Error("old project should be deleted")
	}

	p, err := GetProjectByPath(newPath)
	if err != nil {
		t.Fatalf("new project should exist: %v", err)
	}
	if p.Name != "myproject" {
		t.Errorf("expected project name 'myproject', got %s", p.Name)
	}
}

func TestMergeProjectsNoMatches(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	sessions, messages, err := MergeProjects("/nonexistent/old", "/nonexistent/new")
	if err != nil {
		t.Fatalf("MergeProjects() should not error on no matches: %v", err)
	}

	if sessions != 0 || messages != 0 {
		t.Errorf("expected 0 updates, got sessions=%d, messages=%d", sessions, messages)
	}
}

func TestPruneStaleProjects(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, _ = db.Exec(`
		INSERT INTO sessions (id, project, working_directory, first_query, message_count, tool)
		VALUES 
			('sess1', 'proj1', '/has/sessions', 'test', 1, 'claude'),
			('sess2', 'proj2', '/also/has/sessions', 'test', 1, 'claude')
	`)

	_ = UpsertProject("/has/sessions", time.Now())
	_ = UpsertProject("/also/has/sessions", time.Now())
	_ = UpsertProject("/no/sessions/orphan1", time.Now())
	_ = UpsertProject("/no/sessions/orphan2", time.Now())

	pruned, err := PruneStaleProjects()
	if err != nil {
		t.Fatalf("PruneStaleProjects() error = %v", err)
	}

	if pruned != 2 {
		t.Errorf("expected 2 projects pruned, got %d", pruned)
	}

	projects, _ := GetProjects()
	if len(projects) != 2 {
		t.Errorf("expected 2 projects remaining, got %d", len(projects))
	}

	for _, p := range projects {
		if p.Path == "/no/sessions/orphan1" || p.Path == "/no/sessions/orphan2" {
			t.Errorf("orphan project %s should have been pruned", p.Path)
		}
	}
}

func TestGetDB(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	gotDB := GetDB()
	if gotDB == nil {
		t.Error("GetDB() returned nil")
	}
	if gotDB != db {
		t.Error("GetDB() returned different db instance")
	}
}

func TestProjectStatusConstants(t *testing.T) {
	if ProjectStatusActive != "active" {
		t.Errorf("ProjectStatusActive = %s, want 'active'", ProjectStatusActive)
	}
	if ProjectStatusInactive != "inactive" {
		t.Errorf("ProjectStatusInactive = %s, want 'inactive'", ProjectStatusInactive)
	}
	if ProjectStatusArchived != "archived" {
		t.Errorf("ProjectStatusArchived = %s, want 'archived'", ProjectStatusArchived)
	}
}

func TestUpsertProjectUpdatesLastActivity(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	oldTime := time.Now().Add(-100 * 24 * time.Hour)
	newTime := time.Now()

	_ = UpsertProject("/test/project", oldTime)

	p1, _ := GetProjectByPath("/test/project")
	firstActivity := p1.LastActivity

	_ = UpsertProject("/test/project", newTime)

	p2, _ := GetProjectByPath("/test/project")
	if !p2.LastActivity.After(firstActivity) {
		t.Error("last_activity should be updated to newer time")
	}

	_ = UpsertProject("/test/project", oldTime)

	p3, _ := GetProjectByPath("/test/project")
	if p3.LastActivity.Before(p2.LastActivity) {
		t.Error("last_activity should not be downgraded to older time")
	}
}

func TestGetUsageByToolInWindow(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	windowStart := now.Add(-5 * time.Hour)
	windowEnd := now

	_, _ = db.Exec(`
		INSERT INTO sessions (id, project, working_directory, tool, provider, 
			total_input_tokens, total_output_tokens, total_cache_read, total_cache_write,
			start_time, indexed_at)
		VALUES 
			('sess1', 'proj', '/path', 'claude-code', 'anthropic', 50000, 10000, 5000, 1000, ?, ?),
			('sess2', 'proj', '/path', 'claude-code', 'anthropic', 30000, 5000, 2000, 500, ?, ?),
			('sess3', 'proj', '/path', 'opencode', 'anthropic', 40000, 8000, 3000, 800, ?, ?),
			('sess4', 'proj', '/path', 'cursor', 'anthropic', 20000, 4000, 1000, 200, ?, ?),
			('sess5', 'proj', '/path', 'claude-code', 'openai', 10000, 2000, 500, 100, ?, ?)
	`, now.Add(-1*time.Hour), now.Add(-1*time.Hour),
		now.Add(-2*time.Hour), now.Add(-2*time.Hour),
		now.Add(-3*time.Hour), now.Add(-3*time.Hour),
		now.Add(-4*time.Hour), now.Add(-4*time.Hour),
		now.Add(-30*time.Minute), now.Add(-30*time.Minute))

	stats, err := GetUsageByToolInWindow("anthropic", windowStart, windowEnd)
	if err != nil {
		t.Fatalf("GetUsageByToolInWindow() error = %v", err)
	}

	if len(stats) != 3 {
		t.Errorf("expected 3 tools (claude-code, opencode, cursor), got %d", len(stats))
	}

	toolTokens := make(map[string]int)
	for _, s := range stats {
		toolTokens[s.Tool] = s.TotalTokens
	}

	expectedClaudeCode := 50000 + 10000 + 5000 + 1000 + 30000 + 5000 + 2000 + 500
	if toolTokens["claude-code"] != expectedClaudeCode {
		t.Errorf("claude-code tokens = %d, want %d", toolTokens["claude-code"], expectedClaudeCode)
	}

	expectedOpencode := 40000 + 8000 + 3000 + 800
	if toolTokens["opencode"] != expectedOpencode {
		t.Errorf("opencode tokens = %d, want %d", toolTokens["opencode"], expectedOpencode)
	}

	expectedCursor := 20000 + 4000 + 1000 + 200
	if toolTokens["cursor"] != expectedCursor {
		t.Errorf("cursor tokens = %d, want %d", toolTokens["cursor"], expectedCursor)
	}
}

func TestGetUsageByToolInWindowEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	stats, err := GetUsageByToolInWindow("anthropic", now.Add(-5*time.Hour), now)
	if err != nil {
		t.Fatalf("GetUsageByToolInWindow() error = %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected 0 stats for empty window, got %d", len(stats))
	}
}

func TestGetUsageByToolInWindowOutsideWindow(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()

	_, _ = db.Exec(`
		INSERT INTO sessions (id, project, working_directory, tool, provider, 
			total_input_tokens, total_output_tokens, start_time, indexed_at)
		VALUES ('sess1', 'proj', '/path', 'claude-code', 'anthropic', 50000, 10000, ?, ?)
	`, now.Add(-10*time.Hour), now.Add(-10*time.Hour))

	stats, err := GetUsageByToolInWindow("anthropic", now.Add(-5*time.Hour), now)
	if err != nil {
		t.Fatalf("GetUsageByToolInWindow() error = %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected 0 stats (session outside window), got %d", len(stats))
	}
}
