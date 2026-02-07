// mnemo indexes AI coding sessions from 12+ tools into a unified, searchable
// SQLite database. It reads native session formats (JSONL, JSON, SQLite) and
// normalizes them into a common schema with FTS5 full-text search.
//
// Install: go install github.com/Pilan-AI/mnemo@latest
// Usage:   mnemo index → mnemo search "your query"
package main

import (
	"github.com/Pilan-AI/mnemo/cmd"
)

func main() {
	cmd.Execute()
}
