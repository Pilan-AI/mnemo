package db

import (
	"fmt"
	"path/filepath"
	"time"
)

// Project represents a tracked development directory. Projects are auto-discovered
// from working_directory fields in indexed sessions and classified by recency:
// active (<60 days), inactive (60-90 days), or archived (>90 days).
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

// ClassifyProjects updates project status based on last_activity recency:
// active (<60 days), inactive (60-90 days), archived (>90 days).
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

// MergeProjects moves all session/message history from oldPath to newPath.
// Used when a project directory is relocated on the filesystem.
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
