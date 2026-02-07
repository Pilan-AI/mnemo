package db

import (
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

func InsertMessage(msg Message) error {
	result, err := db.Exec(`
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

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no rows inserted")
	}
	return nil
}
