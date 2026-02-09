package db

import (
	"testing"
	"time"
)

func TestBeginTx(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	tx, err := BeginTx()
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}

	// Insert within transaction
	_, err = tx.Exec(`INSERT INTO sessions (id, project, tool) VALUES ('tx-test', 'proj', 'claude')`)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert in tx error = %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = 'tx-test'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 session after tx, got %d", count)
	}
}

func TestBeginTxRollback(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	tx, err := BeginTx()
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}

	_, _ = tx.Exec(`INSERT INTO sessions (id, project, tool) VALUES ('tx-rollback', 'proj', 'claude')`)
	_ = tx.Rollback()

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = 'tx-rollback'").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 sessions after rollback, got %d", count)
	}
}

func TestClearIndex(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)
	_ = InsertSessionSimple("sess-2", "proj", "q", "/p", "opencode", 2)
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "user", Content: "hello"})
	_ = InsertMessage(Message{SessionID: "sess-2", Project: "proj", Role: "user", Content: "world"})
	_ = InsertTokenUsage(TokenUsage{
		SessionID: "sess-1", Model: "opus", InputTokens: 100, TotalTokens: 100,
		Provider: "anthropic", Timestamp: time.Now(),
	})

	err := ClearIndex()
	if err != nil {
		t.Fatalf("ClearIndex() error = %v", err)
	}

	var sessions, messages, usage int
	_ = db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessions)
	_ = db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&messages)
	_ = db.QueryRow("SELECT COUNT(*) FROM token_usage").Scan(&usage)

	if sessions != 0 {
		t.Errorf("sessions after clear = %d, want 0", sessions)
	}
	if messages != 0 {
		t.Errorf("messages after clear = %d, want 0", messages)
	}
	if usage != 0 {
		t.Errorf("token_usage after clear = %d, want 0", usage)
	}
}

func TestClearIndexPreservesProjects(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = UpsertProject("/my/project", time.Now())
	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)

	_ = ClearIndex()

	// Projects should NOT be cleared
	var projectCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&projectCount)
	if projectCount != 1 {
		t.Errorf("projects after clear = %d, want 1 (should be preserved)", projectCount)
	}
}

func TestTxInsertSessionSimple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	tx, err := BeginTx()
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}

	err = TxInsertSessionSimple(tx, "sess-tx-simple", "proj", "hello", "/p", "claude", 5)
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("TxInsertSessionSimple() error = %v", err)
	}

	_ = tx.Commit()

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = 'sess-tx-simple'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 session, got %d", count)
	}
}

func TestGetUsageEntriesFromSessions(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Insert with raw SQL using SQLite datetime format that the parser expects
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := db.Exec(`
		INSERT INTO sessions (id, project, tool, total_input_tokens, total_output_tokens,
			model, provider, start_time, indexed_at)
		VALUES ('sess-1', 'proj', 'claude', 5000, 3000, 'opus', 'anthropic', ?, ?)
	`, now, now)
	if err != nil {
		t.Fatalf("insert session error = %v", err)
	}

	// GetUsageEntries should fall back to sessions table when token_usage is empty
	entries, err := GetUsageEntries(0)
	if err != nil {
		t.Fatalf("GetUsageEntries() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry from sessions fallback, got %d", len(entries))
	}

	if len(entries) > 0 && entries[0].InputTokens != 5000 {
		t.Errorf("InputTokens = %d, want 5000", entries[0].InputTokens)
	}
}

func TestGetUsageEntriesFromTokenUsage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)
	_ = InsertTokenUsage(TokenUsage{
		SessionID: "sess-1", Model: "opus", InputTokens: 1000, OutputTokens: 500,
		TotalTokens: 1500, Provider: "anthropic", Timestamp: now.Add(-1 * time.Hour),
	})

	entries, err := GetUsageEntries(7)
	if err != nil {
		t.Fatalf("GetUsageEntries() error = %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 entry from token_usage, got %d", len(entries))
	}
}

func TestGetUsageEntriesEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	entries, err := GetUsageEntries(7)
	if err != nil {
		t.Fatalf("GetUsageEntries() error = %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestGetSessionBlocks(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)
	_ = InsertTokenUsage(TokenUsage{
		SessionID: "sess-1", Model: "opus", InputTokens: 1000,
		TotalTokens: 1000, Provider: "anthropic", Timestamp: now.Add(-1 * time.Hour),
	})

	blocks, err := GetSessionBlocks(7)
	if err != nil {
		t.Fatalf("GetSessionBlocks() error = %v", err)
	}

	if len(blocks) == 0 {
		t.Error("expected at least 1 block")
	}
}

func TestGetRecentBlocks(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)
	_ = InsertTokenUsage(TokenUsage{
		SessionID: "sess-1", Model: "opus", InputTokens: 500,
		TotalTokens: 500, Provider: "anthropic", Timestamp: now.Add(-1 * time.Hour),
	})
	_ = InsertTokenUsage(TokenUsage{
		SessionID: "sess-1", Model: "opus", InputTokens: 300,
		TotalTokens: 300, Provider: "anthropic", Timestamp: now.Add(-10 * time.Hour),
	})

	blocks, err := GetRecentBlocks(5)
	if err != nil {
		t.Fatalf("GetRecentBlocks() error = %v", err)
	}

	if len(blocks) == 0 {
		t.Error("expected at least 1 block")
	}
	if len(blocks) > 5 {
		t.Errorf("expected at most 5 blocks, got %d", len(blocks))
	}
}
