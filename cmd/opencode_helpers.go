package cmd

import "strings"

// isSystemDirective checks if content starts with known system directive prefixes
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

// sanitizeContent removes XML-style tag blocks that aren't user content
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
