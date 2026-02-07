package cmd

// projects.go manages tracked project directories. Projects are auto-discovered
// from working_directory in indexed sessions and can be manually added, pruned
// (stale paths), or merged (when a project moves to a new location).

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Pilan-AI/mnemo/internal/db"
	"github.com/Pilan-AI/mnemo/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage tracked projects",
	Long:  "View and manage which projects mnemo tracks for AI session indexing.",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()
		dbPath := filepath.Join(home, ".mnemo", "mnemo.db")

		if !pathExists(dbPath) {
			fmt.Println("No mnemo database found. Run 'mnemo index' first.")
			return
		}

		if err := db.InitDB(); err != nil {
			fmt.Printf("Error initializing database: %v\n", err)
			return
		}
		defer db.CloseDB()

		active, inactive, err := db.GetProjectsForOnboarding()
		if err != nil {
			fmt.Printf("Error getting projects: %v\n", err)
			return
		}

		if len(active) == 0 && len(inactive) == 0 {
			fmt.Println("No projects discovered yet.")
			fmt.Println("Projects are discovered from working_directory in your AI sessions.")
			fmt.Println("Run 'mnemo index --force' to re-index and discover projects.")
			return
		}

		var activeItems []tui.ProjectItem
		for _, p := range active {
			activeItems = append(activeItems, tui.ProjectItem{
				Path:         p.Path,
				Name:         p.Name,
				LastActivity: p.LastActivity,
				Status:       p.Status,
				Selected:     p.UserEnabled,
			})
		}

		var inactiveItems []tui.ProjectItem
		for _, p := range inactive {
			inactiveItems = append(inactiveItems, tui.ProjectItem{
				Path:         p.Path,
				Name:         p.Name,
				LastActivity: p.LastActivity,
				Status:       p.Status,
				Selected:     p.UserEnabled,
			})
		}

		model := tui.NewProjectSelectorModel(activeItems, inactiveItems)
		model.OnComplete = func(enabled []string, disabled []string) {
			for _, path := range enabled {
				_ = db.SetProjectUserEnabled(path, true)
			}
			for _, path := range disabled {
				_ = db.SetProjectUserEnabled(path, false)
			}
			fmt.Printf("\nUpdated %d projects.\n", len(enabled)+len(disabled))
		}

		p := tea.NewProgram(model, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all tracked projects",
	Run: func(cmd *cobra.Command, args []string) {
		if err := db.InitDB(); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer db.CloseDB()

		projects, err := db.GetProjects()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if len(projects) == 0 {
			fmt.Println("No projects found. Run 'mnemo index' to discover projects.")
			return
		}

		fmt.Printf("%-10s %-8s %-6s %-50s %s\n", "STATUS", "ENABLED", "EXISTS", "PATH", "LAST ACTIVITY")
		fmt.Println(repeatChar('-', 110))

		missingCount := 0
		for _, p := range projects {
			enabled := "✓"
			if !p.UserEnabled {
				enabled = "✗"
			}

			exists := "✓"
			if !pathExists(p.Path) {
				exists = "✗"
				missingCount++
			}

			fmt.Printf("%-10s %-8s %-6s %-50s %s\n",
				p.Status,
				enabled,
				exists,
				truncatePath(p.Path, 50),
				p.LastActivity.Format("2006-01-02"),
			)
		}

		if missingCount > 0 {
			fmt.Println()
			fmt.Printf("⚠️  %d project path(s) no longer exist.\n", missingCount)
			fmt.Println("   Use 'mnemo projects prune' to remove stale entries, or")
			fmt.Println("   Use 'mnemo projects merge <old-path> <new-path>' to update moved projects.")
		}
	},
}

var projectsAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a project to track",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := db.InitDB(); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer db.CloseDB()

		path := args[0]
		home, _ := os.UserHomeDir()
		if len(path) > 0 && path[0] == '~' {
			path = filepath.Join(home, path[1:])
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			fmt.Printf("Error resolving path: %v\n", err)
			return
		}

		if !pathExists(absPath) {
			fmt.Printf("Path does not exist: %s\n", absPath)
			return
		}

		if err := db.AddProjectManually(absPath); err != nil {
			fmt.Printf("Error adding project: %v\n", err)
			return
		}

		fmt.Printf("Added project: %s\n", absPath)
	},
}

func repeatChar(c rune, n int) string {
	result := make([]rune, n)
	for i := range result {
		result[i] = c
	}
	return string(result)
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

func init() {
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsAddCmd)
	projectsCmd.AddCommand(projectsPruneCmd)
	projectsCmd.AddCommand(projectsMergeCmd)
	rootCmd.AddCommand(projectsCmd)
}

var projectsPruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove projects with non-existent paths",
	Long:  "Remove project entries whose paths no longer exist on the filesystem.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := db.InitDB(); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer db.CloseDB()

		projects, err := db.GetProjects()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		pruned := 0
		for _, p := range projects {
			if !pathExists(p.Path) {
				if err := db.DeleteProject(p.Path); err == nil {
					fmt.Printf("Removed: %s\n", p.Path)
					pruned++
				}
			}
		}

		if pruned == 0 {
			fmt.Println("No stale projects to remove.")
		} else {
			fmt.Printf("\nPruned %d stale project(s).\n", pruned)
		}
	},
}

var projectsMergeCmd = &cobra.Command{
	Use:   "merge <old-path> <new-path>",
	Short: "Merge project history from old path to new path",
	Long: `Merge all session and message history from an old project path to a new path.

Use this when you've moved a project to a different location:
  mnemo projects merge ~/Projects/old-location ~/Projects/new-location

This will:
  1. Update all sessions referencing the old path to point to the new path
  2. Update all messages referencing the old path to point to the new path
  3. Remove the old project entry
  4. Create/update the new project entry`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := db.InitDB(); err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer db.CloseDB()

		home, _ := os.UserHomeDir()

		oldPath := args[0]
		if len(oldPath) > 0 && oldPath[0] == '~' {
			oldPath = filepath.Join(home, oldPath[1:])
		}
		oldPath, _ = filepath.Abs(oldPath)

		newPath := args[1]
		if len(newPath) > 0 && newPath[0] == '~' {
			newPath = filepath.Join(home, newPath[1:])
		}
		newPath, _ = filepath.Abs(newPath)

		if !pathExists(newPath) {
			fmt.Printf("Warning: New path does not exist: %s\n", newPath)
			fmt.Println("Proceeding anyway (you may be planning to create it).")
		}

		fmt.Printf("Merging project history:\n")
		fmt.Printf("  From: %s\n", oldPath)
		fmt.Printf("  To:   %s\n", newPath)
		fmt.Println()

		sessions, messages, err := db.MergeProjects(oldPath, newPath)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return
		}

		if sessions == 0 && messages == 0 {
			fmt.Println("No sessions or messages found for the old path.")
			fmt.Println("The old path may not have any history, or may already be merged.")
		} else {
			fmt.Printf("✓ Updated %d session(s) and %d message(s)\n", sessions, messages)
			fmt.Printf("✓ Project history merged to: %s\n", newPath)
		}
	},
}
