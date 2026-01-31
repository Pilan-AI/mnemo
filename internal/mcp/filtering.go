//go:build ignore

package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InjectionMode represents the context injection behavior
type InjectionMode string

const (
	InjectionModeOff       InjectionMode = "off"       // No auto-injection
	InjectionModeHelper    InjectionMode = "helper"    // Keyword-based filtering (CODE/DEBUG)
	InjectionModeAssistant InjectionMode = "assistant" // Inject on EVERY message
)

// shouldInjectContext determines if context should be injected based on query content and mode
func (s *Server) shouldInjectContext(query string) bool {
	mode := s.loadInjectionMode()

	switch mode {
	case InjectionModeOff:
		return false

	case InjectionModeHelper:
		return s.shouldInjectHelperMode(query)

	case InjectionModeAssistant:
		return true

	default:
		return s.shouldInjectHelperMode(query)
	}
}

// shouldInjectHelperMode determines if context should be injected in helper mode
// (keyword-based filtering for CODE/DEBUG intents only)
// Returns: shouldInject (bool), reason (string for logging)
func (s *Server) shouldInjectHelperMode(query string) (bool, string) {
	query = strings.ToLower(query)

	codeKeywords := []string{
		"implement", "refactor", "add feature", "build", "create",
		"modify", "update", "change code", "write function",
		"how did i", "what did we", "last time", "previously",
		"earlier", "before", "remember when", "past session",
	}

	debugKeywords := []string{
		"fix", "debug", "error", "broken", "not working", "issue",
		"bug", "crash", "fail", "exception", "help with",
	}

	chatKeywords := []string{
		"hello", "hi", "thanks", "thank you", "bye", "okay",
		"got it", "makes sense", "sounds good", "cool", "nice",
	}

	learnKeywords := []string{
		"what is", "how does", "explain", "tell me about",
		"tutorial", "guide", "documentation", "define",
	}

	for _, kw := range chatKeywords {
		if strings.Contains(query, kw) && len(query) < 50 {
			return false, "chat-intent"
		}
	}

	for _, kw := range learnKeywords {
		if strings.Contains(query, kw) {
			if !strings.Contains(query, "our") && !strings.Contains(query, "my") && !strings.Contains(query, "this project") {
				return false, "learn-intent"
			}
		}
	}

	for _, kw := range append(codeKeywords, debugKeywords...) {
		if strings.Contains(query, kw) {
			return true, "code-debug-intent"
		}
	}

	return false, "no-strong-signal"
}

// loadInjectionMode reads the user's injection preference from config file
func (s *Server) loadInjectionMode() InjectionMode {
	mode := InjectionMode("helper")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return mode
	}

	configPath := filepath.Join(homeDir, ".mnemo", "config.json")

	if data, err := os.ReadFile(configPath); err == nil {
		content := strings.ToLower(string(data))
		if strings.Contains(content, `"injection_mode": "off"`) {
			return InjectionModeOff
		}
		if strings.Contains(content, `"injection_mode": "assistant"`) {
			return InjectionModeAssistant
		}
		if strings.Contains(content, `"injection_mode": "helper"`) {
			return InjectionModeHelper
		}
	}

	return mode
}

// saveInjectionMode saves the user's injection preference
func (s *Server) saveInjectionMode(mode InjectionMode) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	mnemoDir := filepath.Join(homeDir, ".mnemo")
	configPath := filepath.Join(mnemoDir, "config.json")

	if err := os.MkdirAll(mnemoDir, 0755); err != nil {
		return err
	}

	content := fmt.Sprintf(`{
   "injection_mode": "%s"
 }`, mode)

	return os.WriteFile(configPath, []byte(content), 0644)
}
