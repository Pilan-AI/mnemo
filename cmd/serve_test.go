package cmd

import (
	"testing"
	"time"
)

func TestShorten(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "short string", input: "hello", maxLen: 10, want: "hello"},
		{name: "exact length", input: "hello", maxLen: 5, want: "hello"},
		{name: "truncated", input: "hello world foo", maxLen: 10, want: "hello w..."},
		{name: "empty", input: "", maxLen: 10, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shorten(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("shorten(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFormatRelativeShort(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name  string
		input time.Time
		want  string
	}{
		{name: "zero time", input: time.Time{}, want: "?"},
		{name: "minutes", input: now.Add(-20 * time.Minute), want: "20m"},
		{name: "hours", input: now.Add(-5 * time.Hour), want: "5h"},
		{name: "days", input: now.Add(-3 * 24 * time.Hour), want: "3d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRelativeShort(tt.input)
			if got != tt.want {
				t.Errorf("formatRelativeShort() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatRelativeShortFarPast(t *testing.T) {
	old := time.Now().Add(-60 * 24 * time.Hour)
	got := formatRelativeShort(old)
	if got == "?" || got == "" {
		t.Error("formatRelativeShort for 60 days ago should not be empty/unknown")
	}
}
