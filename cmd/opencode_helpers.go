package cmd

// OpenCode stores sessions across per-project directories. These helpers
// build a lookup cache from session metadata files (ses_*.json) so the
// indexer can resolve project names and working directories for messages.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// OpenCodeSessionMeta holds resolved metadata for an OpenCode session file.
type OpenCodeSessionMeta struct {
	Title     string
	Directory string
	Version   string
}

var openCodeSessionCache map[string]*OpenCodeSessionMeta

func buildOpenCodeSessionCache(storagePath string) {
	openCodeSessionCache = make(map[string]*OpenCodeSessionMeta)
	sessionBasePath := filepath.Join(storagePath, "session")

	projectDirs, err := os.ReadDir(sessionBasePath)
	if err != nil {
		return
	}

	for _, projectDir := range projectDirs {
		if !projectDir.IsDir() {
			continue
		}

		projectPath := filepath.Join(sessionBasePath, projectDir.Name())
		sessionFiles, err := os.ReadDir(projectPath)
		if err != nil {
			continue
		}

		for _, sessionFile := range sessionFiles {
			if sessionFile.IsDir() || !strings.HasPrefix(sessionFile.Name(), "ses_") {
				continue
			}

			sessionFilePath := filepath.Join(projectPath, sessionFile.Name())
			data, err := os.ReadFile(sessionFilePath)
			if err != nil {
				continue
			}

			var sessionData map[string]interface{}
			if err := json.Unmarshal(data, &sessionData); err != nil {
				continue
			}

			sessionID := strings.TrimSuffix(sessionFile.Name(), ".json")
			meta := &OpenCodeSessionMeta{}
			if title, ok := sessionData["title"].(string); ok {
				meta.Title = title
			}
			if dir, ok := sessionData["directory"].(string); ok {
				meta.Directory = dir
			}
			if version, ok := sessionData["version"].(string); ok {
				meta.Version = version
			}

			openCodeSessionCache[sessionID] = meta
		}
	}
}

func findOpenCodeSessionMeta(storagePath, sessionID string) *OpenCodeSessionMeta {
	if openCodeSessionCache == nil {
		buildOpenCodeSessionCache(storagePath)
	}
	return openCodeSessionCache[sessionID]
}

// isSystemDirective detects injected system prompts that shouldn't be used as
// the session's first_query. AI tools often prepend directive blocks like
// "[SYSTEM DIRECTIVE..." or "<ultrawork-mode>" before the actual user message.
func isSystemDirective(content string) bool {
	trimmed := strings.TrimSpace(content)

	// Empty or very short content is not useful as first_query
	if len(trimmed) < 10 {
		return true
	}

	systemPrefixes := []string{
		"[search-mode]",
		"[analyze-mode]",
		"[SYSTEM DIRECTIVE",
		"[BACKGROUND TASK",
		"[Category+Skill",
		"[Agent Usage",
		"[Project README",
		"Instructions from:",
		"MAXIMIZE SEARCH EFFORT",
		"ANALYSIS MODE",
		"<ultrawork-mode>",
		"<ralph-loop>",
		"<analysis-mode>",
		"<search-mode>",
		"**MANDATORY**:",
		"[CODE RED]",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	return false
}

// sanitizeContent strips internal XML-style blocks (thinking, analysis mode, etc.)
// that leak into message content from various AI tool implementations.
func sanitizeContent(content string) string {
	result := content

	tagsToRemove := []string{
		"thinking",
		"antml:thinking",
		"ultrawork-mode",
		"ralph-loop",
		"analysis-mode",
		"search-mode",
	}

	for _, tagName := range tagsToRemove {
		result = removeTagBlock(result, tagName)
	}

	return strings.TrimSpace(result)
}

func removeTagBlock(content, tagName string) string {
	result := content
	openTag := "<" + tagName + ">"
	closeTag := "</" + tagName + ">"

	for {
		openIdx := strings.Index(result, openTag)
		if openIdx == -1 {
			break
		}

		closeIdx := strings.Index(result[openIdx:], closeTag)
		if closeIdx == -1 {
			result = result[:openIdx]
			break
		}

		closeEnd := openIdx + closeIdx + len(closeTag)
		result = result[:openIdx] + result[closeEnd:]
	}

	return result
}
