package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseKiroSession(t *testing.T) {
	testCases := []struct {
		name          string
		jsonData      string
		expectedMsgs  int
		expectedTitle string
	}{
		{
			name: "user message with array content",
			jsonData: `{
				"sessionId": "test-session-1",
				"title": "Test Session",
				"workspacePath": "/test/project",
				"history": [
					{
						"message": {
							"role": "user",
							"content": [{"type": "text", "text": "hello world"}],
							"id": "msg-1"
						}
					}
				]
			}`,
			expectedMsgs:  1,
			expectedTitle: "Test Session",
		},
		{
			name: "assistant message with string content",
			jsonData: `{
				"sessionId": "test-session-2",
				"title": "String Content",
				"workspacePath": "/test/project",
				"history": [
					{
						"message": {
							"role": "assistant",
							"content": "I can help with that.",
							"id": "msg-2"
						}
					}
				]
			}`,
			expectedMsgs:  1,
			expectedTitle: "String Content",
		},
		{
			name: "mixed content types",
			jsonData: `{
				"sessionId": "test-session-3",
				"title": "Mixed",
				"workspacePath": "/test/project",
				"history": [
					{
						"message": {
							"role": "user",
							"content": [{"type": "text", "text": "user question"}],
							"id": "msg-1"
						}
					},
					{
						"message": {
							"role": "assistant",
							"content": "assistant answer",
							"id": "msg-2"
						}
					}
				]
			}`,
			expectedMsgs:  2,
			expectedTitle: "Mixed",
		},
		{
			name: "empty history",
			jsonData: `{
				"sessionId": "test-session-4",
				"title": "Empty",
				"workspacePath": "/test/project",
				"history": []
			}`,
			expectedMsgs:  0,
			expectedTitle: "Empty",
		},
		{
			name: "system role filtered out",
			jsonData: `{
				"sessionId": "test-session-5",
				"title": "System Filtered",
				"workspacePath": "/test/project",
				"history": [
					{
						"message": {
							"role": "system",
							"content": "system prompt",
							"id": "msg-1"
						}
					},
					{
						"message": {
							"role": "user",
							"content": [{"type": "text", "text": "user message"}],
							"id": "msg-2"
						}
					}
				]
			}`,
			expectedMsgs:  1,
			expectedTitle: "System Filtered",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var session struct {
				SessionID     string `json:"sessionId"`
				Title         string `json:"title"`
				WorkspacePath string `json:"workspacePath"`
				History       []struct {
					Message struct {
						Role    string      `json:"role"`
						Content interface{} `json:"content"`
						ID      string      `json:"id"`
					} `json:"message"`
				} `json:"history"`
			}

			if err := json.Unmarshal([]byte(tc.jsonData), &session); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			if session.Title != tc.expectedTitle {
				t.Errorf("Expected title %q, got %q", tc.expectedTitle, session.Title)
			}

			msgCount := 0
			for _, h := range session.History {
				var content string
				switch c := h.Message.Content.(type) {
				case string:
					content = c
				case []interface{}:
					for _, item := range c {
						if m, ok := item.(map[string]interface{}); ok {
							if typ, ok := m["type"].(string); ok && typ == "text" {
								if text, ok := m["text"].(string); ok {
									content += text
								}
							}
						}
					}
				}

				if content == "" {
					continue
				}

				role := h.Message.Role
				if role != "user" && role != "assistant" {
					continue
				}

				msgCount++
			}

			if msgCount != tc.expectedMsgs {
				t.Errorf("Expected %d messages, got %d", tc.expectedMsgs, msgCount)
			}
		})
	}
}

func TestParseAmpContent(t *testing.T) {
	testCases := []struct {
		name         string
		contentJSON  string
		expectedText string
	}{
		{
			name:         "simple text array",
			contentJSON:  `[{"type": "text", "text": "hello"}]`,
			expectedText: "hello",
		},
		{
			name:         "multiple text items",
			contentJSON:  `[{"type": "text", "text": "hello "}, {"type": "text", "text": "world"}]`,
			expectedText: "hello world",
		},
		{
			name:         "mixed types filters non-text",
			contentJSON:  `[{"type": "image", "url": "test.png"}, {"type": "text", "text": "caption"}]`,
			expectedText: "caption",
		},
		{
			name:         "empty array",
			contentJSON:  `[]`,
			expectedText: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var content []map[string]interface{}
			if err := json.Unmarshal([]byte(tc.contentJSON), &content); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			var result string
			for _, item := range content {
				if typ, ok := item["type"].(string); ok && typ == "text" {
					if text, ok := item["text"].(string); ok {
						result += text
					}
				}
			}

			if result != tc.expectedText {
				t.Errorf("Expected %q, got %q", tc.expectedText, result)
			}
		})
	}
}

func TestParseCrushParts(t *testing.T) {
	testCases := []struct {
		name         string
		partsJSON    string
		expectedText string
	}{
		{
			name:         "nested data.text structure",
			partsJSON:    `[{"type": "text", "data": {"text": "hello from crush"}}]`,
			expectedText: "hello from crush",
		},
		{
			name:         "multiple parts",
			partsJSON:    `[{"type": "text", "data": {"text": "part1 "}}, {"type": "text", "data": {"text": "part2"}}]`,
			expectedText: "part1 part2",
		},
		{
			name:         "non-text type filtered",
			partsJSON:    `[{"type": "image", "data": {"url": "test.png"}}, {"type": "text", "data": {"text": "caption"}}]`,
			expectedText: "caption",
		},
		{
			name:         "empty parts",
			partsJSON:    `[]`,
			expectedText: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var parts []map[string]interface{}
			if err := json.Unmarshal([]byte(tc.partsJSON), &parts); err != nil {
				t.Fatalf("Failed to parse JSON: %v", err)
			}

			var result string
			for _, part := range parts {
				partType, _ := part["type"].(string)
				if partType == "text" {
					if data, ok := part["data"].(map[string]interface{}); ok {
						if text, ok := data["text"].(string); ok {
							result += text
						}
					}
				}
			}

			if result != tc.expectedText {
				t.Errorf("Expected %q, got %q", tc.expectedText, result)
			}
		})
	}
}

func TestParseCodexJSONL(t *testing.T) {
	testCases := []struct {
		name         string
		lines        []string
		expectedMsgs int
	}{
		{
			name: "user and agent messages",
			lines: []string{
				`{"type": "event_msg", "user_message": {"content": "hello"}}`,
				`{"type": "event_msg", "agent_message": {"content": "hi there"}}`,
			},
			expectedMsgs: 2,
		},
		{
			name: "event_other types ignored",
			lines: []string{
				`{"type": "event_other", "data": "something"}`,
				`{"type": "event_msg", "user_message": {"content": "hello"}}`,
			},
			expectedMsgs: 1,
		},
		{
			name: "empty content filtered",
			lines: []string{
				`{"type": "event_msg", "user_message": {"content": ""}}`,
				`{"type": "event_msg", "agent_message": {"content": "response"}}`,
			},
			expectedMsgs: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msgCount := 0

			for _, line := range tc.lines {
				var event struct {
					Type         string                    `json:"type"`
					UserMessage  *struct{ Content string } `json:"user_message"`
					AgentMessage *struct{ Content string } `json:"agent_message"`
				}

				if err := json.Unmarshal([]byte(line), &event); err != nil {
					continue
				}

				if event.Type != "event_msg" {
					continue
				}

				if event.UserMessage != nil && event.UserMessage.Content != "" {
					msgCount++
				}
				if event.AgentMessage != nil && event.AgentMessage.Content != "" {
					msgCount++
				}
			}

			if msgCount != tc.expectedMsgs {
				t.Errorf("Expected %d messages, got %d", tc.expectedMsgs, msgCount)
			}
		})
	}
}

func TestKiroSessionDiscovery(t *testing.T) {
	tmpDir := t.TempDir()

	workspaceDir := filepath.Join(tmpDir, "workspace1")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatal(err)
	}

	sessionsJSON := `[{"sessionId": "sess-1", "title": "test", "dateCreated": "1234567890"}]`
	if err := os.WriteFile(filepath.Join(workspaceDir, "sessions.json"), []byte(sessionsJSON), 0644); err != nil {
		t.Fatal(err)
	}

	sessionJSON := `{"sessionId": "sess-1", "title": "test", "history": [{"message": {"role": "user", "content": "hi", "id": "1"}}]}`
	if err := os.WriteFile(filepath.Join(workspaceDir, "sess-1.json"), []byte(sessionJSON), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := filepath.Glob(filepath.Join(workspaceDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}

	sessionCount := 0
	for _, f := range files {
		if filepath.Base(f) == "sessions.json" {
			continue
		}
		sessionCount++
	}

	if sessionCount != 1 {
		t.Errorf("Expected 1 session file, got %d", sessionCount)
	}
}

func TestParseAntigravityJSONL(t *testing.T) {
	testCases := []struct {
		name         string
		lines        []string
		expectedMsgs int
	}{
		{
			name: "lines with binary prefix",
			lines: []string{
				"\x12\xc9\xad\x02{\"type\":\"user\",\"timestamp\":\"2026-01-30T18:21:40.592Z\",\"content\":\"hello world\"}",
				"\x12\x50{\"type\":\"tool_use\",\"timestamp\":\"2026-01-30T18:21:41.000Z\",\"tool_name\":\"bash\"}",
				"\x12\x30{\"type\":\"user\",\"timestamp\":\"2026-01-30T18:22:00.000Z\",\"content\":\"second message\"}",
			},
			expectedMsgs: 2,
		},
		{
			name: "clean JSON lines without prefix",
			lines: []string{
				"{\"type\":\"user\",\"timestamp\":\"2026-01-30T18:21:40.592Z\",\"content\":\"clean line\"}",
				"{\"type\":\"assistant\",\"timestamp\":\"2026-01-30T18:21:45.000Z\",\"content\":\"response\"}",
			},
			expectedMsgs: 2,
		},
		{
			name: "mixed content types",
			lines: []string{
				"\x12\xc9{\"type\":\"user\",\"timestamp\":\"2026-01-30T18:21:40.592Z\",\"content\":\"query\"}",
				"{\"type\":\"tool_result\",\"timestamp\":\"2026-01-30T18:21:41.000Z\",\"tool_output\":{}}",
				"\x12\x30{\"type\":\"assistant\",\"timestamp\":\"2026-01-30T18:22:00.000Z\",\"content\":\"answer\"}",
			},
			expectedMsgs: 2,
		},
		{
			name:         "empty content ignored",
			lines:        []string{"{\"type\":\"user\",\"timestamp\":\"2026-01-30T18:21:40.592Z\",\"content\":\"\"}"},
			expectedMsgs: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			msgCount := 0
			for _, line := range tc.lines {
				if line == "" {
					continue
				}

				jsonStart := -1
				for i, c := range line {
					if c == '{' {
						jsonStart = i
						break
					}
				}
				if jsonStart == -1 {
					continue
				}
				jsonLine := line[jsonStart:]

				var event struct {
					Type      string `json:"type"`
					Timestamp string `json:"timestamp"`
					Content   string `json:"content"`
				}

				if err := json.Unmarshal([]byte(jsonLine), &event); err != nil {
					continue
				}

				if event.Type != "user" && event.Type != "assistant" {
					continue
				}

				if event.Content == "" {
					continue
				}

				msgCount++
			}

			if msgCount != tc.expectedMsgs {
				t.Errorf("Expected %d messages, got %d", tc.expectedMsgs, msgCount)
			}
		})
	}
}
