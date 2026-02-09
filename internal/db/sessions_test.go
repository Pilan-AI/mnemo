package db

import (
	"fmt"
	"testing"
	"time"
)

func TestInsertSession(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	sess := Session{
		ID:                "sess-full",
		Project:           "myproject",
		FirstQuery:        "How do I set up auth?",
		MessageCount:      10,
		Tool:              "claude-code",
		FilePath:          "/path/to/session.jsonl",
		Model:             "claude-3-opus",
		Provider:          "anthropic",
		TotalInputTokens:  5000,
		TotalOutputTokens: 3000,
		TotalCacheRead:    1000,
		TotalCacheWrite:   500,
		TotalCostUSD:      0.15,
		CLIVersion:        "1.0.0",
		GitBranch:         "main",
		WorkingDirectory:  "/Users/test/project",
		StartTime:         time.Now().Add(-1 * time.Hour),
		EndTime:           time.Now(),
	}

	err := InsertSession(sess)
	if err != nil {
		t.Fatalf("InsertSession() error = %v", err)
	}

	var project, tool, model string
	var msgCount int
	err = db.QueryRow("SELECT project, tool, model, message_count FROM sessions WHERE id = 'sess-full'").
		Scan(&project, &tool, &model, &msgCount)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}

	if project != "myproject" {
		t.Errorf("project = %s, want myproject", project)
	}
	if tool != "claude-code" {
		t.Errorf("tool = %s, want claude-code", tool)
	}
	if model != "claude-3-opus" {
		t.Errorf("model = %s, want claude-3-opus", model)
	}
	if msgCount != 10 {
		t.Errorf("message_count = %d, want 10", msgCount)
	}
}

func TestInsertSessionUpsert(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	sess := Session{
		ID:           "sess-upsert",
		Project:      "proj",
		FirstQuery:   "original",
		MessageCount: 5,
		Tool:         "claude",
	}
	_ = InsertSession(sess)

	// Upsert with new data
	sess.FirstQuery = "updated"
	sess.MessageCount = 15
	err := InsertSession(sess)
	if err != nil {
		t.Fatalf("InsertSession upsert error = %v", err)
	}

	var query string
	var count int
	_ = db.QueryRow("SELECT first_query, message_count FROM sessions WHERE id = 'sess-upsert'").
		Scan(&query, &count)

	if query != "updated" {
		t.Errorf("first_query = %s, want updated", query)
	}
	if count != 15 {
		t.Errorf("message_count = %d, want 15", count)
	}

	// Should still be one row
	var total int
	_ = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = 'sess-upsert'").Scan(&total)
	if total != 1 {
		t.Errorf("expected 1 session after upsert, got %d", total)
	}
}

func TestInsertSessionSimple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	err := InsertSessionSimple("sess-simple", "proj", "hello world", "/path/file.jsonl", "opencode", 7)
	if err != nil {
		t.Fatalf("InsertSessionSimple() error = %v", err)
	}

	var project, firstQuery, tool string
	var msgCount int
	err = db.QueryRow("SELECT project, first_query, tool, message_count FROM sessions WHERE id = 'sess-simple'").
		Scan(&project, &firstQuery, &tool, &msgCount)
	if err != nil {
		t.Fatalf("query error: %v", err)
	}

	if project != "proj" {
		t.Errorf("project = %s, want proj", project)
	}
	if firstQuery != "hello world" {
		t.Errorf("first_query = %s, want hello world", firstQuery)
	}
	if tool != "opencode" {
		t.Errorf("tool = %s, want opencode", tool)
	}
	if msgCount != 7 {
		t.Errorf("message_count = %d, want 7", msgCount)
	}
}

func TestTxInsertSession(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	sess := Session{
		ID:           "sess-tx",
		Project:      "proj",
		FirstQuery:   "tx query",
		MessageCount: 3,
		Tool:         "claude",
	}

	err = TxInsertSession(tx, sess)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("TxInsertSession() error = %v", err)
	}

	_ = tx.Commit()

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = 'sess-tx'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 session after tx commit, got %d", count)
	}
}

func TestTxInsertSessionRollback(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_ = TxInsertSession(tx, Session{
		ID: "sess-rb", Project: "proj", Tool: "claude",
	})
	_ = tx.Rollback()

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = 'sess-rb'").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 sessions after rollback, got %d", count)
	}
}

func TestGetRecentSessions(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj-a", "first", "/p", "claude", 5)
	_ = InsertSessionSimple("sess-2", "proj-b", "second", "/p", "opencode", 10)
	_ = InsertSessionSimple("sess-3", "proj-c", "third", "/p", "cursor", 3)

	sessions, err := GetRecentSessions(10)
	if err != nil {
		t.Fatalf("GetRecentSessions() error = %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestGetRecentSessionsLimit(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	for i := 0; i < 10; i++ {
		_ = InsertSessionSimple(
			fmt.Sprintf("sess-%d", i), "proj", "query", "/p", "claude", 1,
		)
	}

	sessions, err := GetRecentSessions(3)
	if err != nil {
		t.Fatalf("GetRecentSessions() error = %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions (limit), got %d", len(sessions))
	}
}

func TestGetRecentSessionsZeroLimit(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)

	sessions, err := GetRecentSessions(0)
	if err != nil {
		t.Fatalf("GetRecentSessions(0) error = %v", err)
	}

	// 0 defaults to 10
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}
}

func TestGetRecentSessionsEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	sessions, err := GetRecentSessions(10)
	if err != nil {
		t.Fatalf("GetRecentSessions() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestGetStats(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 3)
	_ = InsertSessionSimple("sess-2", "proj", "q", "/p", "opencode", 5)

	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "user", Content: "msg 1"})
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "assistant", Content: "msg 2"})
	_ = InsertMessage(Message{SessionID: "sess-2", Project: "proj", Role: "user", Content: "msg 3"})

	sessionCount, messageCount, err := GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if sessionCount != 2 {
		t.Errorf("sessionCount = %d, want 2", sessionCount)
	}
	if messageCount != 3 {
		t.Errorf("messageCount = %d, want 3", messageCount)
	}
}

func TestGetStatsEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	sessionCount, messageCount, err := GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if sessionCount != 0 {
		t.Errorf("sessionCount = %d, want 0", sessionCount)
	}
	if messageCount != 0 {
		t.Errorf("messageCount = %d, want 0", messageCount)
	}
}

func TestGetIndexedSessions(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)
	_ = InsertSessionSimple("sess-2", "proj", "q", "/p", "opencode", 2)

	indexed, err := GetIndexedSessions()
	if err != nil {
		t.Fatalf("GetIndexedSessions() error = %v", err)
	}

	if len(indexed) != 2 {
		t.Errorf("expected 2 indexed sessions, got %d", len(indexed))
	}
	if _, ok := indexed["sess-1"]; !ok {
		t.Error("sess-1 should be in indexed sessions")
	}
	if _, ok := indexed["sess-2"]; !ok {
		t.Error("sess-2 should be in indexed sessions")
	}
}

func TestGetIndexedSessionsEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	indexed, err := GetIndexedSessions()
	if err != nil {
		t.Fatalf("GetIndexedSessions() error = %v", err)
	}
	if len(indexed) != 0 {
		t.Errorf("expected 0 indexed sessions, got %d", len(indexed))
	}
}

func TestUpdateSessionTokens(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-tok", "proj", "q", "/p", "claude", 1)

	err := UpdateSessionTokens("sess-tok", 1000, 500, 200, 100, 0.05, "claude-3-opus", "anthropic")
	if err != nil {
		t.Fatalf("UpdateSessionTokens() error = %v", err)
	}

	var input, output int
	var cost float64
	_ = db.QueryRow("SELECT total_input_tokens, total_output_tokens, total_cost_usd FROM sessions WHERE id = 'sess-tok'").
		Scan(&input, &output, &cost)

	if input != 1000 {
		t.Errorf("total_input_tokens = %d, want 1000", input)
	}
	if output != 500 {
		t.Errorf("total_output_tokens = %d, want 500", output)
	}

	// Second update should accumulate
	_ = UpdateSessionTokens("sess-tok", 500, 250, 0, 0, 0.02, "", "")

	_ = db.QueryRow("SELECT total_input_tokens, total_output_tokens, total_cost_usd FROM sessions WHERE id = 'sess-tok'").
		Scan(&input, &output, &cost)

	if input != 1500 {
		t.Errorf("accumulated input = %d, want 1500", input)
	}
	if output != 750 {
		t.Errorf("accumulated output = %d, want 750", output)
	}
}

func TestGetMaxIndexedAtByTool(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)
	_ = InsertSessionSimple("sess-2", "proj", "q", "/p", "opencode", 2)

	result, err := GetMaxIndexedAtByTool()
	if err != nil {
		t.Fatalf("GetMaxIndexedAtByTool() error = %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 tools, got %d", len(result))
	}
	if _, ok := result["claude"]; !ok {
		t.Error("expected 'claude' in results")
	}
	if _, ok := result["opencode"]; !ok {
		t.Error("expected 'opencode' in results")
	}
}
