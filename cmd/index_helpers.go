// index_helpers.go provides shared utility functions used across all indexer
// adapters: incremental skip logic, string truncation, project name extraction,
// and provider inference from model names.
package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// indexCutoff filters out files older than this time during indexing.
// When zero (default), all files are indexed. Set during onboarding
// to only index recent data for a fast first-run experience.
var indexCutoff time.Time

// indexedSessions caches session_id → indexed_at for incremental indexing.
// Populated once at start when --incremental is set.
var indexedSessions map[string]time.Time

// isSessionUnchanged returns true if a session file hasn't been modified
// since its last index. Used to skip re-indexing unchanged sessions.
func isSessionUnchanged(sessionID string, fileModTime time.Time) bool {
	if indexedSessions == nil {
		return false
	}
	indexedAt, exists := indexedSessions[sessionID]
	if !exists {
		return false
	}
	// File hasn't been modified since last index — skip
	return !fileModTime.After(indexedAt)
}

// skipOldFile returns true if the file should be skipped because it's
// older than the indexCutoff. Always returns false if cutoff is not set.
func skipOldFile(info os.FileInfo) bool {
	if indexCutoff.IsZero() {
		return false
	}
	return info.ModTime().UTC().Before(indexCutoff.UTC())
}

// truncate returns s shortened to maxLen runes with "..." appended if needed.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// extractProjectName infers a human-readable project name from a Claude Code
// session path by parsing the dash-separated directory components.
func extractProjectName(path string) string {
	// Extract from path like: -Users-raghu-Projects-PILAN-INTELLIGENCE-PRISM
	dir := filepath.Dir(path)
	parts := strings.Split(dir, string(os.PathSeparator))

	for _, part := range parts {
		if strings.HasPrefix(part, "-") && strings.Contains(part, "-") {
			// Convert: -Volumes-UMBRA-BACKUP-PERSONAL-FORGE -> PERSONAL-FORGE
			segments := strings.Split(part, "-")
			if len(segments) > 2 {
				// Find last meaningful segment
				for i := len(segments) - 1; i >= 0; i-- {
					if segments[i] != "" && segments[i] != "Users" && segments[i] != "Volumes" {
						return strings.Join(segments[max(1, i-1):], "-")
					}
				}
			}
			return part
		}
	}

	return filepath.Base(dir)
}

// isSourceDBUnchanged returns true if a source database file (e.g. state.vscdb,
// crush.db) hasn't been modified since the last time we indexed sessions from
// that tool. Used for DB-backed tools where per-session mtime checks aren't possible.
func isSourceDBUnchanged(dbPath, toolName string) bool {
	if indexedSessions == nil {
		return false
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		return false
	}
	dbModTime := info.ModTime()

	// Check against the lastSourceIndex marker stored per-tool
	if lastIdx, ok := lastSourceIndex[toolName]; ok {
		return !dbModTime.After(lastIdx)
	}
	return false
}

// lastSourceIndex tracks when each tool's source was last indexed.
// Populated from the max(indexed_at) per tool at startup.
var lastSourceIndex map[string]time.Time

// inferProviderFromModel guesses the provider based on model name patterns
func inferProviderFromModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "claude") || strings.Contains(m, "haiku") || strings.Contains(m, "sonnet") || strings.Contains(m, "opus"):
		return "anthropic"
	case strings.Contains(m, "gpt") || strings.HasPrefix(m, "o1") || strings.HasPrefix(m, "o3") || strings.HasPrefix(m, "o4"):
		return "openai"
	case strings.Contains(m, "gemini") || strings.Contains(m, "gemma"):
		return "google"
	case strings.Contains(m, "glm"):
		return "zai"
	case strings.Contains(m, "deepseek"):
		return "deepseek"
	case strings.Contains(m, "llama") || strings.Contains(m, "mistral"):
		return "meta"
	default:
		return ""
	}
}
