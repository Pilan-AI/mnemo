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

// skipOldFile returns true if the file should be skipped because it's
// older than the indexCutoff. Always returns false if cutoff is not set.
func skipOldFile(info os.FileInfo) bool {
	if indexCutoff.IsZero() {
		return false
	}
	return info.ModTime().Before(indexCutoff)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// inferProviderFromModel guesses the provider based on model name patterns
func inferProviderFromModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "claude") || strings.Contains(m, "haiku") || strings.Contains(m, "sonnet") || strings.Contains(m, "opus"):
		return "anthropic"
	case strings.Contains(m, "gpt") || strings.Contains(m, "o1") || strings.Contains(m, "o3") || strings.Contains(m, "o4"):
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
