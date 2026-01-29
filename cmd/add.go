package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Pilan-AI/mnemo/internal/db"
	"github.com/spf13/cobra"
)

var (
	addName    string
	addPattern string
)

var addCmd = &cobra.Command{
	Use:   "add [path]",
	Short: "Add a knowledge source (docs, notes, code) to mnemo",
	Long: `Index documentation, notes, or other text files as a searchable knowledge source.

Examples:
  mnemo add ~/Projects/my-project --name "my-project"
  mnemo add ~/Documents/notes --name "notes" --pattern "*.md"
  mnemo add ~/Projects/PILAN-INTELLIGENCE-PRISM --name "prism"`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sourcePath := args[0]

		// Expand ~ to home directory
		if strings.HasPrefix(sourcePath, "~") {
			home, _ := os.UserHomeDir()
			sourcePath = filepath.Join(home, sourcePath[1:])
		}

		// Validate path exists
		if !pathExists(sourcePath) {
			fmt.Printf("Error: Path does not exist: %s\n", sourcePath)
			return
		}

		// Default name from directory
		if addName == "" {
			addName = filepath.Base(sourcePath)
		}

		// Default pattern
		if addPattern == "" {
			addPattern = "*.md"
		}

		fmt.Printf("Adding knowledge source: %s\n", addName)
		fmt.Printf("Path: %s\n", sourcePath)
		fmt.Printf("Pattern: %s\n", addPattern)
		fmt.Println()

		// Initialize database
		if err := db.InitDB(); err != nil {
			fmt.Printf("Error initializing database: %v\n", err)
			return
		}
		defer db.CloseDB()

		// Walk and index files
		fileCount := 0
		totalBytes := int64(0)

		filepath.Walk(sourcePath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			// Skip directories
			if info.IsDir() {
				// Skip common non-content directories
				base := filepath.Base(path)
				if base == ".git" || base == "node_modules" || base == ".venv" || base == "__pycache__" {
					return filepath.SkipDir
				}
				return nil
			}

			// Check if file matches pattern
			matched, _ := filepath.Match(addPattern, filepath.Base(path))
			if !matched {
				// Also check common documentation patterns
				ext := strings.ToLower(filepath.Ext(path))
				if ext != ".md" && ext != ".txt" && ext != ".rst" && ext != ".org" {
					return nil
				}
			}

			// Read file content
			content, err := readFileContent(path)
			if err != nil || content == "" {
				return nil
			}

			// Create a session ID based on file path
			relPath, _ := filepath.Rel(sourcePath, path)
			sessionID := fmt.Sprintf("doc:%s:%s", addName, relPath)

			// Insert as a document
			err = db.InsertMessage(db.Message{
				SessionID: sessionID,
				Project:   addName,
				Role:      "document",
				Content:   content,
				Timestamp: info.ModTime(),
				Tool:      "docs",
			})
			if err != nil {
				return nil
			}

			fileCount++
			totalBytes += info.Size()

			return nil
		})

		fmt.Printf("  ✓ Indexed %d files (%.2f MB)\n", fileCount, float64(totalBytes)/1024/1024)
		fmt.Println()
		fmt.Printf("Search with: mnemo search \"your query\" (will include docs)\n")
	},
}

func readFileContent(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// Limit to 1MB per file
	data, err := io.ReadAll(io.LimitReader(file, 1024*1024))
	if err != nil {
		return "", err
	}

	content := string(data)

	// Skip binary files (check for null bytes)
	if strings.Contains(content[:min(len(content), 1000)], "\x00") {
		return "", nil
	}

	return content, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	addCmd.Flags().StringVarP(&addName, "name", "n", "", "Name for this knowledge source")
	addCmd.Flags().StringVarP(&addPattern, "pattern", "p", "*.md", "File pattern to match")
	rootCmd.AddCommand(addCmd)
}
