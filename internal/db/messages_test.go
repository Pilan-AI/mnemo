package db

import (
	"testing"
	"time"
)

func TestInsertMessage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	// Need a session first (foreign key isn't enforced in SQLite by default, but good practice)
	err := InsertSessionSimple("sess-1", "myproject", "hello", "/path/to/file", "claude", 0)
	if err != nil {
		t.Fatalf("InsertSessionSimple() error = %v", err)
	}

	msg := Message{
		SessionID:   "sess-1",
		Project:     "myproject",
		Role:        "user",
		Content:     "How do I implement authentication?",
		Timestamp:   time.Now(),
		Tool:        "claude",
		Model:       "claude-3-opus",
		Provider:    "anthropic",
		InputTokens: 100,
	}

	err = InsertMessage(msg)
	if err != nil {
		t.Fatalf("InsertMessage() error = %v", err)
	}

	// Verify it was inserted
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = 'sess-1'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 message, got %d", count)
	}
}

func TestInsertMultipleMessages(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "myproject", "hello", "/path", "claude", 0)

	messages := []Message{
		{SessionID: "sess-1", Project: "myproject", Role: "user", Content: "question one"},
		{SessionID: "sess-1", Project: "myproject", Role: "assistant", Content: "answer one"},
		{SessionID: "sess-1", Project: "myproject", Role: "user", Content: "question two"},
	}

	for _, msg := range messages {
		if err := InsertMessage(msg); err != nil {
			t.Fatalf("InsertMessage() error = %v", err)
		}
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = 'sess-1'").Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 messages, got %d", count)
	}
}

func TestDeleteSessionMessages(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 0)
	_ = InsertSessionSimple("sess-2", "proj", "q", "/p", "claude", 0)

	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "user", Content: "msg 1"})
	_ = InsertMessage(Message{SessionID: "sess-1", Project: "proj", Role: "assistant", Content: "msg 2"})
	_ = InsertMessage(Message{SessionID: "sess-2", Project: "proj", Role: "user", Content: "msg 3"})

	err := DeleteSessionMessages("sess-1")
	if err != nil {
		t.Fatalf("DeleteSessionMessages() error = %v", err)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = 'sess-1'").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 messages for sess-1 after delete, got %d", count)
	}

	// sess-2 should be untouched
	_ = db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = 'sess-2'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 message for sess-2, got %d", count)
	}
}

func TestDeleteSessionMessagesNonexistent(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	err := DeleteSessionMessages("nonexistent")
	if err != nil {
		t.Errorf("DeleteSessionMessages on nonexistent should not error, got %v", err)
	}
}

func TestTxInsertMessage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-tx", "proj", "q", "/p", "claude", 0)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	err = TxInsertMessage(tx, Message{
		SessionID: "sess-tx",
		Project:   "proj",
		Role:      "user",
		Content:   "transactional message",
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("TxInsertMessage() error = %v", err)
	}

	err = tx.Commit()
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = 'sess-tx'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 message after tx commit, got %d", count)
	}
}

func TestTxInsertMessageRollback(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-rb", "proj", "q", "/p", "claude", 0)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	_ = TxInsertMessage(tx, Message{
		SessionID: "sess-rb",
		Project:   "proj",
		Role:      "user",
		Content:   "this will be rolled back",
	})

	_ = tx.Rollback()

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = 'sess-rb'").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 messages after rollback, got %d", count)
	}
}

func TestTxDeleteSessionMessages(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-del", "proj", "q", "/p", "claude", 0)
	_ = InsertMessage(Message{SessionID: "sess-del", Project: "proj", Role: "user", Content: "hello"})
	_ = InsertMessage(Message{SessionID: "sess-del", Project: "proj", Role: "assistant", Content: "hi"})

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}

	err = TxDeleteSessionMessages(tx, "sess-del")
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("TxDeleteSessionMessages() error = %v", err)
	}

	_ = tx.Commit()

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = 'sess-del'").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 messages after tx delete, got %d", count)
	}
}

func TestInsertMessageFTSIndex(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-fts", "proj", "q", "/p", "claude", 0)
	_ = InsertMessage(Message{
		SessionID: "sess-fts",
		Project:   "proj",
		Role:      "user",
		Content:   "authentication flow with JWT tokens",
	})

	// Verify FTS5 index was updated via trigger
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH 'authentication'").Scan(&count)
	if err != nil {
		t.Fatalf("FTS5 query error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 FTS match for 'authentication', got %d", count)
	}
}

func TestDeleteMessageRemovesFTSEntry(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-fts-del", "proj", "q", "/p", "claude", 0)
	_ = InsertMessage(Message{
		SessionID: "sess-fts-del",
		Project:   "proj",
		Role:      "user",
		Content:   "unique_search_term_xyz123",
	})

	// Verify it's in FTS
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH 'unique_search_term_xyz123'").Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 FTS match before delete, got %d", count)
	}

	// Delete and verify FTS is cleaned
	_ = DeleteSessionMessages("sess-fts-del")
	_ = db.QueryRow("SELECT COUNT(*) FROM messages_fts WHERE messages_fts MATCH 'unique_search_term_xyz123'").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 FTS matches after delete, got %d", count)
	}
}
