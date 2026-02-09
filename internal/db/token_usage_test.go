package db

import (
	"testing"
	"time"
)

func TestInsertTokenUsage(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)

	usage := TokenUsage{
		SessionID:        "sess-1",
		Model:            "claude-3-opus",
		InputTokens:      1000,
		OutputTokens:     500,
		CacheReadTokens:  200,
		CacheWriteTokens: 100,
		TotalTokens:      1800,
		CostUSD:          0.05,
		Provider:         "anthropic",
		Timestamp:        time.Now(),
	}

	err := InsertTokenUsage(usage)
	if err != nil {
		t.Fatalf("InsertTokenUsage() error = %v", err)
	}

	// Verify it was inserted
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM token_usage WHERE session_id = 'sess-1'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 token_usage row, got %d", count)
	}

	// Verify session tokens were updated
	var inputTokens int
	_ = db.QueryRow("SELECT total_input_tokens FROM sessions WHERE id = 'sess-1'").Scan(&inputTokens)
	if inputTokens != 1000 {
		t.Errorf("session total_input_tokens = %d, want 1000", inputTokens)
	}
}

func TestInsertTokenUsageAccumulates(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)

	_ = InsertTokenUsage(TokenUsage{
		SessionID: "sess-1", Model: "opus", InputTokens: 500, OutputTokens: 200,
		TotalTokens: 700, CostUSD: 0.02, Provider: "anthropic", Timestamp: time.Now(),
	})
	_ = InsertTokenUsage(TokenUsage{
		SessionID: "sess-1", Model: "opus", InputTokens: 300, OutputTokens: 100,
		TotalTokens: 400, CostUSD: 0.01, Provider: "anthropic", Timestamp: time.Now(),
	})

	var inputTokens, outputTokens int
	_ = db.QueryRow("SELECT total_input_tokens, total_output_tokens FROM sessions WHERE id = 'sess-1'").
		Scan(&inputTokens, &outputTokens)

	if inputTokens != 800 {
		t.Errorf("accumulated input = %d, want 800", inputTokens)
	}
	if outputTokens != 300 {
		t.Errorf("accumulated output = %d, want 300", outputTokens)
	}
}

func TestGetTokenStats(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)

	_ = InsertTokenUsage(TokenUsage{
		SessionID: "sess-1", Model: "opus", InputTokens: 1000, OutputTokens: 500,
		CacheReadTokens: 200, CacheWriteTokens: 100, TotalTokens: 1800,
		CostUSD: 0.05, Provider: "anthropic", Timestamp: time.Now(),
	})
	_ = InsertTokenUsage(TokenUsage{
		SessionID: "sess-1", Model: "sonnet", InputTokens: 500, OutputTokens: 250,
		TotalTokens: 750, CostUSD: 0.02, Provider: "anthropic", Timestamp: time.Now(),
	})

	stats, err := GetTokenStats(0)
	if err != nil {
		t.Fatalf("GetTokenStats() error = %v", err)
	}

	if stats.TotalInputTokens != 1500 {
		t.Errorf("TotalInputTokens = %d, want 1500", stats.TotalInputTokens)
	}
	if stats.TotalOutputTokens != 750 {
		t.Errorf("TotalOutputTokens = %d, want 750", stats.TotalOutputTokens)
	}
	if stats.SessionCount != 1 {
		t.Errorf("SessionCount = %d, want 1", stats.SessionCount)
	}
}

func TestGetTokenStatsEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	stats, err := GetTokenStats(0)
	if err != nil {
		t.Fatalf("GetTokenStats() error = %v", err)
	}
	if stats.TotalInputTokens != 0 {
		t.Errorf("TotalInputTokens = %d, want 0", stats.TotalInputTokens)
	}
}

func TestGetTokenStatsByProvider(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSessionSimple("sess-1", "proj", "q", "/p", "claude", 1)
	_ = InsertSessionSimple("sess-2", "proj", "q", "/p", "opencode", 1)

	_ = InsertTokenUsage(TokenUsage{
		SessionID: "sess-1", Model: "opus", InputTokens: 1000, TotalTokens: 1000,
		Provider: "anthropic", Timestamp: time.Now(),
	})
	_ = InsertTokenUsage(TokenUsage{
		SessionID: "sess-2", Model: "gpt-4", InputTokens: 500, TotalTokens: 500,
		Provider: "openai", Timestamp: time.Now(),
	})

	stats, err := GetTokenStatsByProvider(0)
	if err != nil {
		t.Fatalf("GetTokenStatsByProvider() error = %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("expected 2 providers, got %d", len(stats))
	}
	if stats["anthropic"].TotalInputTokens != 1000 {
		t.Errorf("anthropic input = %d, want 1000", stats["anthropic"].TotalInputTokens)
	}
	if stats["openai"].TotalInputTokens != 500 {
		t.Errorf("openai input = %d, want 500", stats["openai"].TotalInputTokens)
	}
}

func TestGetUsageStats(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSession(Session{
		ID: "sess-1", Project: "proj", Tool: "claude",
		TotalInputTokens: 5000, TotalOutputTokens: 3000, TotalCostUSD: 0.15,
	})
	_ = InsertSession(Session{
		ID: "sess-2", Project: "proj", Tool: "opencode",
		TotalInputTokens: 2000, TotalOutputTokens: 1000, TotalCostUSD: 0.05,
	})

	stats, err := GetUsageStats()
	if err != nil {
		t.Fatalf("GetUsageStats() error = %v", err)
	}

	if stats.TotalInputTokens != 7000 {
		t.Errorf("TotalInputTokens = %d, want 7000", stats.TotalInputTokens)
	}
	if stats.TotalOutputTokens != 4000 {
		t.Errorf("TotalOutputTokens = %d, want 4000", stats.TotalOutputTokens)
	}
	if stats.TotalTokens != 11000 {
		t.Errorf("TotalTokens = %d, want 11000", stats.TotalTokens)
	}
	if stats.SessionCount != 2 {
		t.Errorf("SessionCount = %d, want 2", stats.SessionCount)
	}
}

func TestGetUsageStatsEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	stats, err := GetUsageStats()
	if err != nil {
		t.Fatalf("GetUsageStats() error = %v", err)
	}
	if stats.SessionCount != 0 {
		t.Errorf("SessionCount = %d, want 0", stats.SessionCount)
	}
}

func TestGetUsageStatsByTool(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSession(Session{
		ID: "sess-1", Project: "proj", Tool: "claude-code",
		TotalInputTokens: 5000, TotalOutputTokens: 3000, TotalCostUSD: 0.15,
	})
	_ = InsertSession(Session{
		ID: "sess-2", Project: "proj", Tool: "claude-code",
		TotalInputTokens: 2000, TotalOutputTokens: 1000, TotalCostUSD: 0.05,
	})
	_ = InsertSession(Session{
		ID: "sess-3", Project: "proj", Tool: "opencode",
		TotalInputTokens: 1000, TotalOutputTokens: 500, TotalCostUSD: 0.02,
	})

	stats, err := GetUsageStatsByTool()
	if err != nil {
		t.Fatalf("GetUsageStatsByTool() error = %v", err)
	}

	if len(stats) != 2 {
		t.Errorf("expected 2 tools, got %d", len(stats))
	}

	cc := stats["claude-code"]
	if cc.Sessions != 2 {
		t.Errorf("claude-code sessions = %d, want 2", cc.Sessions)
	}
	if cc.InputTokens != 7000 {
		t.Errorf("claude-code input = %d, want 7000", cc.InputTokens)
	}
	if cc.TotalTokens != 11000 {
		t.Errorf("claude-code total = %d, want 11000", cc.TotalTokens)
	}
}

func TestGetUsageStatsByModel(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = InsertSession(Session{
		ID: "sess-1", Project: "proj", Tool: "claude", Model: "claude-3-opus",
		TotalInputTokens: 5000, TotalOutputTokens: 3000,
	})
	_ = InsertSession(Session{
		ID: "sess-2", Project: "proj", Tool: "claude", Model: "claude-3-sonnet",
		TotalInputTokens: 2000, TotalOutputTokens: 1000,
	})
	_ = InsertSession(Session{
		ID: "sess-3", Project: "proj", Tool: "opencode", Model: "",
		TotalInputTokens: 1000,
	})

	stats, err := GetUsageStatsByModel()
	if err != nil {
		t.Fatalf("GetUsageStatsByModel() error = %v", err)
	}

	// Empty model sessions are excluded (WHERE model != '')
	if len(stats) != 2 {
		t.Errorf("expected 2 models, got %d", len(stats))
	}

	opus := stats["claude-3-opus"]
	if opus.InputTokens != 5000 {
		t.Errorf("opus input = %d, want 5000", opus.InputTokens)
	}
}

func TestSetAPICredential(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	err := SetAPICredential("anthropic")
	if err != nil {
		t.Fatalf("SetAPICredential() error = %v", err)
	}

	providers, err := GetAPICredentials()
	if err != nil {
		t.Fatalf("GetAPICredentials() error = %v", err)
	}

	if len(providers) != 1 || providers[0] != "anthropic" {
		t.Errorf("expected [anthropic], got %v", providers)
	}
}

func TestSetAPICredentialUpsert(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = SetAPICredential("anthropic")
	_ = SetAPICredential("anthropic") // Should not duplicate

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM api_credentials WHERE provider = 'anthropic'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 credential row, got %d", count)
	}
}

func TestGetAPICredentialsEmpty(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	providers, err := GetAPICredentials()
	if err != nil {
		t.Fatalf("GetAPICredentials() error = %v", err)
	}
	if len(providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(providers))
	}
}

func TestGetAPICredentialsMultiple(t *testing.T) {
	cleanup := setupTestDB(t)
	defer cleanup()

	_ = SetAPICredential("anthropic")
	_ = SetAPICredential("openai")
	_ = SetAPICredential("google")

	providers, err := GetAPICredentials()
	if err != nil {
		t.Fatalf("GetAPICredentials() error = %v", err)
	}
	if len(providers) != 3 {
		t.Errorf("expected 3 providers, got %d", len(providers))
	}
}
