// projects.go manages tracked development directories. Projects are auto-discovered
// from working_directory fields in indexed sessions and classified by recency
// (active/stale/archived). Users can enable/disable projects for selective indexing.
package db

import (
	"fmt"
	"log"
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

// UpsertProject creates or updates a project entry, setting last_activity.
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

// GetProjects returns all tracked projects ordered by last activity.
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
			log.Printf("GetProjects: rows.Scan error: %v", err)
			continue
		}
		p.UserEnabled = userEnabled == 1
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return projects, fmt.Errorf("GetProjects iteration error: %w", err)
	}
	return projects, nil
}

// GetProjectsByStatus returns projects filtered by classification (active/inactive/archived).
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
			log.Printf("GetProjectsByStatus: rows.Scan error: %v", err)
			continue
		}
		p.UserEnabled = userEnabled == 1
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return projects, fmt.Errorf("GetProjectsByStatus iteration error: %w", err)
	}
	return projects, nil
}

// GetEnabledProjects returns only projects the user has explicitly enabled.
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
			log.Printf("GetEnabledProjects: rows.Scan error: %v", err)
			continue
		}
		p.UserEnabled = userEnabled == 1
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return projects, fmt.Errorf("GetEnabledProjects iteration error: %w", err)
	}
	return projects, nil
}

// SetProjectUserEnabled toggles the user_enabled flag for a project path.
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

// GetProjectsForOnboarding returns active and inactive projects for the first-run experience.
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

// AddProjectManually creates a project entry from a user-specified path.
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

// DeleteProject removes a project entry by its path.
func DeleteProject(path string) error {
	_, err := db.Exec(`DELETE FROM projects WHERE path = ?`, path)
	return err
}

// PruneStaleProjects removes disabled projects with no activity in the last 180 days.
func PruneStaleProjects() (int, error) {
	result, err := db.Exec(`DELETE FROM projects WHERE path NOT IN (SELECT DISTINCT working_directory FROM sessions WHERE working_directory != '')`)
	if err != nil {
		return 0, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to check rows affected: %w", err)
	}
	return int(count), nil
}

// MergeProjects moves all session/message history from oldPath to newPath.
// Used when a project directory is relocated on the filesystem.
// All operations run in a single transaction for atomicity.
func MergeProjects(oldPath, newPath string) (int, int, error) {
	tx, err := db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	newName := filepath.Base(newPath)

	sessionsResult, err := tx.Exec(`
		UPDATE sessions SET
			working_directory = ?,
			project = ?
		WHERE working_directory = ?
	`, newPath, newName, oldPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to update sessions: %w", err)
	}
	sessionsUpdated, err := sessionsResult.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to check session rows: %w", err)
	}

	messagesResult, err := tx.Exec(`
		UPDATE messages SET
			working_directory = ?,
			project = ?
		WHERE working_directory = ?
	`, newPath, newName, oldPath)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to update messages: %w", err)
	}
	messagesUpdated, err := messagesResult.RowsAffected()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to check message rows: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM projects WHERE path = ?`, oldPath); err != nil {
		return 0, 0, fmt.Errorf("failed to delete old project: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO projects (path, name, last_activity, status, user_enabled)
		VALUES (?, ?, CURRENT_TIMESTAMP, 'active', 1)
		ON CONFLICT(path) DO UPDATE SET
			name = excluded.name,
			last_activity = MAX(last_activity, excluded.last_activity)
	`, newPath, newName); err != nil {
		return 0, 0, fmt.Errorf("failed to create new project: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit merge: %w", err)
	}

	return int(sessionsUpdated), int(messagesUpdated), nil
}

// GetProjectByPath looks up a single project by its filesystem path.
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
