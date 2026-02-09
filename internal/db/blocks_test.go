package db

import (
	"testing"
	"time"
)

func TestSessionBlockTotalTokens(t *testing.T) {
	b := SessionBlock{
		InputTokens:  1000,
		OutputTokens: 500,
		CacheRead:    200,
		CacheWrite:   100,
	}
	if got := b.TotalTokens(); got != 1800 {
		t.Errorf("TotalTokens() = %d, want 1800", got)
	}
}

func TestSessionBlockDurationMinutes(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		b    SessionBlock
		want float64
	}{
		{
			name: "30 minutes",
			b:    SessionBlock{StartTime: now, ActualEndTime: now.Add(30 * time.Minute)},
			want: 30,
		},
		{
			name: "zero start",
			b:    SessionBlock{ActualEndTime: now},
			want: 0,
		},
		{
			name: "zero end",
			b:    SessionBlock{StartTime: now},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.b.DurationMinutes()
			if got != tt.want {
				t.Errorf("DurationMinutes() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestSessionBlockTokensPerMinute(t *testing.T) {
	now := time.Now()
	b := SessionBlock{
		StartTime:     now,
		ActualEndTime: now.Add(10 * time.Minute),
		InputTokens:   600,
		OutputTokens:  400,
	}

	got := b.TokensPerMinute()
	want := 100.0 // 1000 tokens / 10 minutes
	if got != want {
		t.Errorf("TokensPerMinute() = %f, want %f", got, want)
	}
}

func TestSessionBlockTokensPerMinuteZeroDuration(t *testing.T) {
	b := SessionBlock{InputTokens: 1000}
	if got := b.TokensPerMinute(); got != 0 {
		t.Errorf("TokensPerMinute() with zero duration = %f, want 0", got)
	}
}

func TestSessionBlockCostPerHour(t *testing.T) {
	now := time.Now()
	b := SessionBlock{
		StartTime:     now,
		ActualEndTime: now.Add(30 * time.Minute),
		CostUSD:       1.50,
	}

	got := b.CostPerHour()
	want := 3.0 // $1.50 per 30 min = $3/hr
	if got != want {
		t.Errorf("CostPerHour() = %f, want %f", got, want)
	}
}

func TestSessionBlockRemainingTime(t *testing.T) {
	now := time.Now()

	// Active block with time remaining
	active := SessionBlock{
		IsActive: true,
		EndTime:  now.Add(2 * time.Hour),
	}
	remaining := active.RemainingTime()
	if remaining <= 0 {
		t.Error("active block should have positive remaining time")
	}

	// Inactive block
	inactive := SessionBlock{IsActive: false, EndTime: now.Add(1 * time.Hour)}
	if got := inactive.RemainingTime(); got != 0 {
		t.Errorf("inactive block RemainingTime() = %v, want 0", got)
	}

	// Expired block
	expired := SessionBlock{IsActive: true, EndTime: now.Add(-1 * time.Hour)}
	if got := expired.RemainingTime(); got != 0 {
		t.Errorf("expired block RemainingTime() = %v, want 0", got)
	}
}

func TestSessionBlockProjectedTokens(t *testing.T) {
	// Inactive block should return actual tokens
	inactive := SessionBlock{
		IsActive:     false,
		InputTokens:  1000,
		OutputTokens: 500,
	}
	if got := inactive.ProjectedTokens(); got != 1500 {
		t.Errorf("inactive ProjectedTokens() = %d, want 1500", got)
	}
}

func TestFloorToHour(t *testing.T) {
	input := time.Date(2026, 2, 8, 14, 37, 45, 123, time.UTC)
	got := floorToHour(input)
	want := time.Date(2026, 2, 8, 14, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("floorToHour() = %v, want %v", got, want)
	}
}

func TestFloorToHourExactHour(t *testing.T) {
	input := time.Date(2026, 2, 8, 14, 0, 0, 0, time.UTC)
	got := floorToHour(input)
	if !got.Equal(input) {
		t.Errorf("floorToHour() on exact hour = %v, want %v", got, input)
	}
}

func TestIdentifySessionBlocksEmpty(t *testing.T) {
	blocks := IdentifySessionBlocks(nil)
	if blocks != nil {
		t.Errorf("expected nil for empty entries, got %v", blocks)
	}
}

func TestIdentifySessionBlocksSingleEntry(t *testing.T) {
	now := time.Now()
	entries := []UsageEntry{
		{SessionID: "s1", Timestamp: now, InputTokens: 1000, OutputTokens: 500, Model: "opus", Provider: "anthropic"},
	}

	blocks := IdentifySessionBlocks(entries)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	b := blocks[0]
	if b.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", b.InputTokens)
	}
	if b.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", b.OutputTokens)
	}
	if b.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", b.MessageCount)
	}
	if len(b.Models) != 1 || b.Models[0] != "opus" {
		t.Errorf("Models = %v, want [opus]", b.Models)
	}
}

func TestIdentifySessionBlocksMultipleInSameWindow(t *testing.T) {
	now := time.Now()
	entries := []UsageEntry{
		{SessionID: "s1", Timestamp: now, InputTokens: 500, Model: "opus", Provider: "anthropic"},
		{SessionID: "s1", Timestamp: now.Add(30 * time.Minute), InputTokens: 300, Model: "opus", Provider: "anthropic"},
		{SessionID: "s2", Timestamp: now.Add(1 * time.Hour), InputTokens: 200, Model: "sonnet", Provider: "anthropic"},
	}

	blocks := IdentifySessionBlocks(entries)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block (all within 5h), got %d", len(blocks))
	}

	b := blocks[0]
	if b.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", b.InputTokens)
	}
	if b.SessionCount != 2 {
		t.Errorf("SessionCount = %d, want 2", b.SessionCount)
	}
	if b.MessageCount != 3 {
		t.Errorf("MessageCount = %d, want 3", b.MessageCount)
	}
}

func TestIdentifySessionBlocksSplitAcrossWindows(t *testing.T) {
	now := time.Now()
	entries := []UsageEntry{
		{SessionID: "s1", Timestamp: now, InputTokens: 500, Provider: "anthropic"},
		{SessionID: "s2", Timestamp: now.Add(6 * time.Hour), InputTokens: 300, Provider: "anthropic"},
	}

	blocks := IdentifySessionBlocks(entries)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks (>5h gap), got %d", len(blocks))
	}

	if blocks[0].InputTokens != 500 {
		t.Errorf("block 0 InputTokens = %d, want 500", blocks[0].InputTokens)
	}
	if blocks[1].InputTokens != 300 {
		t.Errorf("block 1 InputTokens = %d, want 300", blocks[1].InputTokens)
	}
}

func TestIdentifySessionBlocksCostAccumulation(t *testing.T) {
	now := time.Now()
	entries := []UsageEntry{
		{SessionID: "s1", Timestamp: now, CostUSD: 0.10},
		{SessionID: "s1", Timestamp: now.Add(1 * time.Hour), CostUSD: 0.20},
		{SessionID: "s1", Timestamp: now.Add(2 * time.Hour), CostUSD: 0.15},
	}

	blocks := IdentifySessionBlocks(entries)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	expected := 0.45
	if blocks[0].CostUSD < expected-0.001 || blocks[0].CostUSD > expected+0.001 {
		t.Errorf("CostUSD = %f, want ~%f", blocks[0].CostUSD, expected)
	}
}

func TestMapKeys(t *testing.T) {
	m := map[string]bool{"a": true, "b": true, "c": true}
	keys := mapKeys(m)
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}

func TestMapKeysEmpty(t *testing.T) {
	m := map[string]bool{}
	keys := mapKeys(m)
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}
