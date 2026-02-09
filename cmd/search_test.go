package cmd

import (
	"testing"
	"time"
)

func TestCleanTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple text", input: "How do I set up auth?", want: "How do I set up auth?"},
		{name: "strips XML tags", input: "<system-reminder>hello</system-reminder>", want: "hello"},
		{name: "strips nested tags", input: "<div><p>hello world</p></div>", want: "hello world"},
		{name: "skips hash lines", input: "# Header\nActual content", want: "Actual content"},
		{name: "skips dashes", input: "---\nReal content", want: "Real content"},
		{name: "skips empty lines", input: "\n\n\nContent here", want: "Content here"},
		{name: "skips background noise", input: "[ALL BACKGROUND tasks complete]\nReal query", want: "Real query"},
		{name: "skips completed noise", input: "**Completed tasks**\nActual question", want: "Actual question"},
		{name: "skips bg_ noise", input: "- `bg_task_1`\nMy question", want: "My question"},
		{name: "skips use background", input: "Use `background` for...\nHello", want: "Hello"},
		{name: "skips brainstorm noise", input: "[BRAINSTORM MODE]\nActual input", want: "Actual input"},
		{name: "skips note prefix", input: "Note: something\nReal content", want: "Real content"},
		{name: "all noise returns empty", input: "# Header\n---\n[ALL BACKGROUND done]", want: ""},
		{name: "empty string", input: "", want: ""},
		{name: "only whitespace", input: "   \n   \n   ", want: ""},
		{name: "multiline takes first meaningful", input: "# Ignore\n---\nFirst real line\nSecond line", want: "First real line"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanTitle(tt.input)
			if got != tt.want {
				t.Errorf("cleanTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name  string
		input time.Time
		want  string
	}{
		{name: "zero time", input: time.Time{}, want: ""},
		{name: "just now", input: now.Add(-30 * time.Second), want: "0m ago"},
		{name: "minutes ago", input: now.Add(-15 * time.Minute), want: "15m ago"},
		{name: "hours ago", input: now.Add(-3 * time.Hour), want: "3h ago"},
		{name: "yesterday", input: now.Add(-30 * time.Hour), want: "yesterday"},
		{name: "days ago", input: now.Add(-5 * 24 * time.Hour), want: "5d ago"},
		{name: "weeks ago", input: now.Add(-14 * 24 * time.Hour), want: "14d ago"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeTime(tt.input)
			if got != tt.want {
				t.Errorf("relativeTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRelativeTimeFarPast(t *testing.T) {
	// More than 30 days ago should show date format
	old := time.Now().Add(-60 * 24 * time.Hour)
	got := relativeTime(old)
	// Should be in "Jan 2" format
	if got == "" {
		t.Error("relativeTime for 60 days ago should not be empty")
	}
	// Verify it's not the "Xd ago" format
	if len(got) > 0 && got[len(got)-1] == 'o' { // ends with "ago"
		t.Errorf("60 days ago should use date format, got %q", got)
	}
}

func TestRelativeTimeShort(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name  string
		input time.Time
		want  string
	}{
		{name: "zero time", input: time.Time{}, want: "?"},
		{name: "minutes", input: now.Add(-15 * time.Minute), want: "15m"},
		{name: "hours", input: now.Add(-3 * time.Hour), want: "3h"},
		{name: "days", input: now.Add(-5 * 24 * time.Hour), want: "5d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := relativeTimeShort(tt.input)
			if got != tt.want {
				t.Errorf("relativeTimeShort() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRelativeTimeShortFarPast(t *testing.T) {
	old := time.Now().Add(-60 * 24 * time.Hour)
	got := relativeTimeShort(old)
	// Should be "Jan2" format (no space)
	if got == "?" || got == "" {
		t.Error("relativeTimeShort for 60 days ago should not be empty/unknown")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "short string", input: "hello", maxLen: 10, want: "hello"},
		{name: "exact length", input: "hello", maxLen: 5, want: "hello"},
		{name: "needs truncation", input: "hello world", maxLen: 8, want: "hello wo..."},
		{name: "very short max", input: "hello world", maxLen: 4, want: "hell..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}
