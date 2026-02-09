// messages.go handles message-level CRUD operations. Each message belongs to a
// session and is stored with full metadata (role, content, tokens, cost).
// Provides both direct (InsertMessage) and transactional (TxInsertMessage) variants.
package db

import (
	"database/sql"
	"fmt"
	"time"
)

// Message represents a single message within an AI coding session.
type Message struct {
	ID               int64
	SessionID        string
	Project          string
	Role             string
	Content          string
	Timestamp        time.Time
	Tool             string
	Model            string
	Provider         string
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	ReasoningTokens  int
	CostUSD          float64
	MessageUUID      string
	ParentUUID       string
	WorkingDirectory string
	Agent            string
	Date             string
}

func deleteSessionMessages(ex execer, sessionID string) error {
	_, err := ex.Exec("DELETE FROM messages WHERE session_id = ?", sessionID)
	return err
}

// DeleteSessionMessages removes all messages for a session.
// Must be called before re-indexing a session to prevent duplicates.
func DeleteSessionMessages(sessionID string) error {
	return deleteSessionMessages(db, sessionID)
}

// TxDeleteSessionMessages removes all messages for a session within a transaction.
func TxDeleteSessionMessages(tx *sql.Tx, sessionID string) error {
	return deleteSessionMessages(tx, sessionID)
}

func insertMessage(ex execer, msg Message) error {
	result, err := ex.Exec(`
		INSERT INTO messages (
			session_id, project, role, content, timestamp, tool,
			model, provider, input_tokens, output_tokens,
			cache_read_tokens, cache_write_tokens, reasoning_tokens, cost_usd,
			message_uuid, parent_uuid, working_directory, agent, date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		msg.SessionID, msg.Project, msg.Role, msg.Content, msg.Timestamp, msg.Tool,
		msg.Model, msg.Provider, msg.InputTokens, msg.OutputTokens,
		msg.CacheReadTokens, msg.CacheWriteTokens, msg.ReasoningTokens, msg.CostUSD,
		msg.MessageUUID, msg.ParentUUID, msg.WorkingDirectory, msg.Agent, msg.Date,
	)
	if err != nil {
		return err
	}

	rows, err2 := result.RowsAffected()
	if err2 != nil {
		return fmt.Errorf("failed to check rows affected: %w", err2)
	}
	if rows == 0 {
		return fmt.Errorf("no rows inserted")
	}
	return nil
}

// InsertMessage inserts a single message record using the global DB connection.
func InsertMessage(msg Message) error {
	return insertMessage(db, msg)
}

// TxInsertMessage inserts a message within a transaction.
func TxInsertMessage(tx *sql.Tx, msg Message) error {
	return insertMessage(tx, msg)
}
