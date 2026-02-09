// index_amp.go indexes Amp CLI sessions from ~/.local/share/amp/.
// Supports threads/ directory containing individual JSON conversation files.
package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
)

// indexAmp indexes Amp CLI sessions from ~/.local/share/amp/
// Format: history.jsonl for prompts, threads/ directory for full conversations
func indexAmp(basePath string) (int, int) {
	threadsPath := filepath.Join(basePath, "threads")
	if !pathExists(threadsPath) {
		return 0, 0
	}

	threadFiles, err := os.ReadDir(threadsPath)
	if err != nil {
		return 0, 0
	}

	totalSessions := 0
	totalMessages := 0

	for _, threadFile := range threadFiles {
		if threadFile.IsDir() || !strings.HasSuffix(threadFile.Name(), ".json") {
			continue
		}
		if info, err := threadFile.Info(); err == nil && skipOldFile(info) {
			continue
		}
		fileSessionID := strings.TrimSuffix(threadFile.Name(), ".json")
		if info, err := threadFile.Info(); err == nil && isSessionUnchanged(fileSessionID, info.ModTime()) {
			continue
		}

		threadPath := filepath.Join(threadsPath, threadFile.Name())
		s, m := indexAmpThread(threadPath)
		totalSessions += s
		totalMessages += m
	}

	return totalSessions, totalMessages
}

// indexAmpThread parses a single Amp thread JSON file and inserts all
// messages atomically within a transaction.
func indexAmpThread(threadPath string) (int, int) {
	data, err := os.ReadFile(threadPath)
	if err != nil {
		return 0, 0
	}

	var thread struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Messages []struct {
			Role      string `json:"role"`
			MessageID int    `json:"messageId"`
			Content   []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Provider string `json:"provider"`
			} `json:"content"`
		} `json:"messages"`
		Created int64 `json:"created"`
		Env     struct {
			Initial struct {
				Tags []string `json:"tags"`
			} `json:"initial"`
		} `json:"env"`
		UsageLedger struct {
			Events []struct {
				Model   string  `json:"model"`
				Credits float64 `json:"credits"`
				Tokens  struct {
					Input  int `json:"input"`
					Output int `json:"output"`
				} `json:"tokens"`
				OperationType string `json:"operationType"`
				FromMessageID int    `json:"fromMessageId"`
				ToMessageID   int    `json:"toMessageId"`
			} `json:"events"`
		} `json:"usageLedger"`
		Debug struct {
			LastInferenceUsage struct {
				Model                    string `json:"model"`
				InputTokens              int    `json:"inputTokens"`
				OutputTokens             int    `json:"outputTokens"`
				CacheReadInputTokens     int    `json:"cacheReadInputTokens"`
				CacheCreationInputTokens int    `json:"cacheCreationInputTokens"`
				TotalInputTokens         int    `json:"totalInputTokens"`
			} `json:"lastInferenceUsage"`
		} `json:"~debug"`
	}

	if err := json.Unmarshal(data, &thread); err != nil {
		return 0, 0
	}

	if len(thread.Messages) == 0 {
		return 0, 0
	}

	sessionID := thread.ID
	if sessionID == "" {
		sessionID = strings.TrimSuffix(filepath.Base(threadPath), ".json")
	}

	projectName := "amp"
	var sessionProvider string
	var sessionModel string

	// Extract model from env.initial.tags (e.g. "model:claude-opus-4-5-20251101")
	for _, tag := range thread.Env.Initial.Tags {
		if strings.HasPrefix(tag, "model:") {
			sessionModel = strings.TrimPrefix(tag, "model:")
			break
		}
	}

	// Build per-message token map from usageLedger events
	type ledgerEntry struct {
		inputTokens  int
		outputTokens int
		model        string
		credits      float64
	}
	msgLedger := make(map[int]ledgerEntry) // keyed by toMessageId (the assistant response)
	var totalInputTokens, totalOutputTokens int
	var totalCredits float64

	for _, event := range thread.UsageLedger.Events {
		if event.OperationType == "title-generation" {
			continue // Skip title generation, not a user-facing message
		}
		entry := msgLedger[event.ToMessageID]
		entry.inputTokens += event.Tokens.Input
		entry.outputTokens += event.Tokens.Output
		entry.credits += event.Credits
		if entry.model == "" && event.Model != "" {
			entry.model = event.Model
		}
		msgLedger[event.ToMessageID] = entry
		totalInputTokens += event.Tokens.Input
		totalOutputTokens += event.Tokens.Output
		totalCredits += event.Credits

		// Update session model from ledger if not set from tags
		if sessionModel == "" && event.Model != "" {
			sessionModel = event.Model
		}
	}

	// Fallback: if no usageLedger, use ~debug.lastInferenceUsage for session-level totals
	if len(thread.UsageLedger.Events) == 0 && thread.Debug.LastInferenceUsage.Model != "" {
		dbg := thread.Debug.LastInferenceUsage
		totalInputTokens = dbg.TotalInputTokens
		totalOutputTokens = dbg.OutputTokens
		if sessionModel == "" {
			sessionModel = dbg.Model
		}
	}

	// Infer provider from model name
	if sessionModel != "" && sessionProvider == "" {
		sessionProvider = inferProviderFromModel(sessionModel)
	}

	var firstUserMsg string
	msgCount := 0

	tx, err := db.BeginTx()
	if err != nil {
		indexErrors++
		return 0, 0
	}
	defer func() { _ = tx.Rollback() }()

	if err := db.TxDeleteSessionMessages(tx, sessionID); err != nil {
		indexErrors++
		return 0, 0
	}

	for _, msg := range thread.Messages {
		var content string
		var msgProvider string
		for _, c := range msg.Content {
			if c.Type == "text" && c.Text != "" {
				content += c.Text
			}
			if c.Provider != "" && msgProvider == "" {
				msgProvider = c.Provider
			}
		}

		if sessionProvider == "" && msgProvider != "" {
			sessionProvider = msgProvider
		}

		if content == "" {
			continue
		}

		if msg.Role == "user" && firstUserMsg == "" {
			firstUserMsg = truncate(content, 200)
		}

		timestamp := time.UnixMilli(thread.Created)

		// Attach token data from usageLedger to this message
		var msgInputTokens, msgOutputTokens int
		var msgModel string
		var msgCost float64
		if le, ok := msgLedger[msg.MessageID]; ok {
			msgInputTokens = le.inputTokens
			msgOutputTokens = le.outputTokens
			msgModel = le.model
			msgCost = le.credits // Amp credits ~ cost units
		}

		if msgModel == "" {
			msgModel = sessionModel
		}
		if msgProvider == "" {
			msgProvider = sessionProvider
		}

		err := db.TxInsertMessage(tx, db.Message{
			SessionID:    sessionID,
			Project:      projectName,
			Role:         msg.Role,
			Content:      content,
			Timestamp:    timestamp,
			Tool:         "amp",
			Provider:     msgProvider,
			Model:        msgModel,
			InputTokens:  msgInputTokens,
			OutputTokens: msgOutputTokens,
			CostUSD:      msgCost,
		})
		if err != nil {
			indexErrors++
			continue
		}
		msgCount++
	}

	if msgCount > 0 {
		if err := db.TxInsertSession(tx, db.Session{
			ID:                sessionID,
			Project:           projectName,
			FirstQuery:        firstUserMsg,
			MessageCount:      msgCount,
			Tool:              "amp",
			FilePath:          threadPath,
			Provider:          sessionProvider,
			Model:             sessionModel,
			TotalInputTokens:  totalInputTokens,
			TotalOutputTokens: totalOutputTokens,
			TotalCostUSD:      totalCredits,
		}); err != nil {
			indexErrors++
			return 0, msgCount
		}
		if err := tx.Commit(); err != nil {
			indexErrors++
			return 0, msgCount
		}
		return 1, msgCount
	}

	return 0, 0
}
