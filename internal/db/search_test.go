package db

import (
	"fmt"
	"testing"
	"time"
)

func TestSanitizeFTS5Query(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "simple query", input: "authentication", want: "authentication"},
		{name: "multi word", input: "auth flow", want: "auth flow"},
		{name: "strips parens", input: "func(x)", want: "func x"},
		{name: "strips quotes", input: `"hello world"`, want: "hello world"},
		{name: "strips asterisk", input: "auth*", want: "auth"},
		{name: "strips colon", input: "project:foo", want: "project foo"},
		{name: "strips plus minus", input: "+required -excluded", want: "required excluded"},
		{name: "strips caret", input: "term^2", want: "term 2"},
		{name: "collapses spaces", input: "a  b   c", want: "a b c"},
		{name: "trims whitespace", input: "  hello  ", want: "hello"},
		{name: "empty after sanitize", input: "???***", wantErr: true},
		{name: "single quotes", input: "it's working", want: "it s working"},
		{name: "mixed special chars", input: `search("hello":world*)`, want: "search hello world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizeFTS5Query(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("sanitizeFTS5Query(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("sanitizeFTS5Query(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseFlexibleTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantUTC  bool
		wantZero bool
	}{
		{name: "empty string", input: "", wantZero: true},
		{name: "sqlite format", input: "2026-02-08 14:30:00", wantUTC: true},
		{name: "RFC3339", input: "2026-02-08T14:30:00Z", wantUTC: true},
		{name: "RFC3339 with offset", input: "2026-02-08T14:30:00+05:30", wantUTC: true},
		{name: "Go timestamp with tz", input: "2026-02-08 14:30:00.000 +0000 UTC", wantUTC: true},
		{name: "ISO without T", input: "2026-02-08T14:30:00Z", wantUTC: true},
		{name: "invalid format", input: "not-a-date", wantZero: true},
		{name: "partial date", input: "2026-02", wantZero: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFlexibleTime(tt.input)
			if tt.wantZero {
				if !got.IsZero() {
					t.Errorf("parseFlexibleTime(%q) = %v, want zero", tt.input, got)
				}
				return
			}
			if got.IsZero() {
				t.Errorf("parseFlexibleTime(%q) returned zero, want non-zero", tt.input)
				return
			}
			if tt.wantUTC && got.Location() != time.UTC {
				t.Errorf("parseFlexibleTime(%q) location = %v, want UTC", tt.input, got.Location())
			}
		})
	}
}

func TestParseFlexibleTimeUTCNormalization(t *testing.T) {
	// A timestamp with +05:30 offset should be normalized to UTC
	result := parseFlexibleTime("2026-02-08T14:30:00+05:30")
	if result.IsZero() {
		t.Fatal("expected non-zero result")
	}
	if result.Hour() != 9 {
		t.Errorf("expected UTC hour 9 (14:30 IST = 09:00 UTC), got %d", result.Hour())
	}
}

func TestSearch(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "auth question", "/p", "claude", 2)
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "user", Content: "How to implement JWT authentication?"})
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "assistant", Content: "Here is how to implement JWT authentication in Go."})

	_ = InsertSessionSimple("sess-2", "proj", "database design", "/p", "claude", 1)
	_ = InsertMessage(Message{SessionID: "sess-2", Project: "proj", Role: "user", Content: "Help me design the database schema."})

	results, err := Search("authentication", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for 'authentication', got %d", len(results))
	}
}

func TestSearchNoResults(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "user", Content: "hello world"})

	results, err := Search("nonexistent_term_xyz", 10)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchLimit(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 5)
	for i := 0; i < 5; i++ {
		_ = InsertMessage(Message{
			SessionID: "sess-1", Project: "proj", Role: "user",
			Content: "authentication implementation discussion",
		})
	}

	results, err := Search("authentication", 2)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results (limited), got %d", len(results))
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, err := Search("", 10)
	if err == nil {
		t.Error("Search with empty query should return error")
	}
}

func TestSearchSpecialChars(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "user", Content: "function call test"})

	// Should not error even with special chars
	results, err := Search("function()", 10)
	if err != nil {
		t.Fatalf("Search with special chars error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSearchGrouped(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()

	// Session 1: 3 messages about auth
	_ = InsertSession(Session{
		ID: "sess-1", Project: "proj-a", FirstQuery: "auth question",
		MessageCount: 3, Tool: "claude", StartTime: now.Add(-1 * time.Hour),
	})
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj-a", Role: "user", Content: "How to implement authentication?"})
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj-a", Role: "assistant", Content: "Use JWT for authentication."})
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj-a", Role: "user", Content: "Show authentication middleware."})

	// Session 2: 1 message about auth
	_ = InsertSession(Session{
		ID: "sess-2", Project: "proj-b", FirstQuery: "login",
		MessageCount: 1, Tool: "opencode", StartTime: now.Add(-24 * time.Hour),
	})
	_ = InsertMessage(Message{SessionID: "sess-2", Project: "proj-b", Role: "user", Content: "Authentication flow for login page."})

	results, err := SearchGrouped("authentication", 5)
	if err != nil {
		t.Fatalf("SearchGrouped() error = %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 session results, got %d", len(results))
	}

	// Session with more matches and more recent should rank higher
	if results[0].SessionID != "sess-1" {
		t.Errorf("expected sess-1 to rank first (more matches, more recent), got %s", results[0].SessionID)
	}

	if results[0].MatchCount < results[1].MatchCount {
		t.Errorf("first result should have more matches: %d vs %d", results[0].MatchCount, results[1].MatchCount)
	}
}

func TestSearchGroupedLimit(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	for i := 0; i < 10; i++ {
		sid := fmt.Sprintf("sess-%d", i)
		_ = InsertSession(Session{
			ID: sid, Project: "proj", FirstQuery: "test query",
			MessageCount: 1, Tool: "claude", StartTime: now.Add(-time.Duration(i) * time.Hour),
		})
		_ = InsertMessage(Message{SessionID: sid, Project: "proj", Role: "user", Content: "common keyword search term"})
	}

	results, err := SearchGrouped("keyword", 3)
	if err != nil {
		t.Fatalf("SearchGrouped() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results (limited), got %d", len(results))
	}
}

func TestSearchGroupedZeroLimit(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSession(Session{
		ID: "sess-1", Project: "proj", FirstQuery: "q",
		MessageCount: 1, Tool: "claude", StartTime: time.Now(),
	})
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "user", Content: "hello world"})

	// 0 defaults to 5
	results, err := SearchGrouped("hello", 0)
	if err != nil {
		t.Fatalf("SearchGrouped(0) error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSearchGroupedUserSnippetPreference(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSession(Session{
		ID: "sess-1", Project: "proj", FirstQuery: "q",
		MessageCount: 2, Tool: "claude", StartTime: time.Now(),
	})
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "assistant", Content: "database migration pattern"})
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "user", Content: "How to do database migration?"})

	results, err := SearchGrouped("database migration", 5)
	if err != nil {
		t.Fatalf("SearchGrouped() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 session result, got %d", len(results))
	}

	// Should prefer user snippet
	if results[0].SnippetRole != "user" {
		t.Errorf("expected snippet from user role, got %s", results[0].SnippetRole)
	}
}

func TestSearchGroupedEmptyQuery(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_, err := SearchGrouped("***", 5)
	if err == nil {
		t.Error("SearchGrouped with empty-after-sanitize query should error")
	}
}
