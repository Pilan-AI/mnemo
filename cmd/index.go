package cmd

// index.go is the core indexing engine. It detects installed AI tools,
// reads their native session formats, and normalizes everything into
// the unified mnemo schema (sessions + messages + FTS5).
//
// Supported tools and their storage formats:
//
//   Claude Code     — ~/.claude/projects/**/*/   JSONL (one file per session)
//   OpenCode        — ~/.local/share/opencode/   JSON  (message/ + session/ dirs)
//   Gemini CLI      — ~/.gemini/sessions/         JSON  (one file per session)
//   Cursor          — ~/Library/.../Cursor/       SQLite (state.vscdb)
//   Crush           — ~/.crush/crush.db           SQLite (conversations table)
//   Aider           — ~/.aider.chat.history.md    Markdown (single file)
//   Codex           — ~/.codex/                   JSONL
//   Amp             — ~/.local/share/amp/         JSONL
//   Antigravity     — ~/Library/.../Antigravity/  JSONL (state.vscdb)
//   Kiro            — ~/Library/.../Kiro/         SQLite (state.vscdb)
//   Cline/Roo/Kilo  — VS Code extensions          JSON  (tasks/ directory)
//
// The first run triggers the interactive onboarding TUI instead of
// a bare index operation.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pilan-AI/mnemo/internal/db"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	indexAll    bool
	indexTool   string
	indexForce  bool
	indexErrors int
	indexPath   string
	indexFormat string
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index AI tool conversation history",
	Long:  "Scan and index conversation history from all detected AI coding tools.",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()

		// Normal index/re-index flow
		fmt.Println("Indexing AI tool conversations...")
		fmt.Println()

		// Initialize SQLite database
		if err := db.InitDB(); err != nil {
			fmt.Printf("Error initializing database: %v\n", err)
			return
		}
		defer db.CloseDB()

		// Clear existing index if force flag
		if indexForce {
			if err := db.ClearIndex(); err != nil {
				fmt.Printf("Error clearing index: %v\n", err)
				os.Exit(1)
			}
		}

		// Handle custom path indexing
		if indexPath != "" {
			if indexFormat == "" {
				fmt.Println("Error: --format is required when using --path")
				fmt.Println("Supported formats: claude, opencode, codex, amp, gemini, cline, kiro, antigravity, crush")
				os.Exit(1)
			}

			expandedPath := indexPath
			if strings.HasPrefix(expandedPath, "~") {
				expandedPath = filepath.Join(home, expandedPath[1:])
			}

			if !pathExists(expandedPath) {
				fmt.Printf("Error: Path does not exist: %s\n", expandedPath)
				os.Exit(1)
			}

			fmt.Printf("Indexing custom path: %s (format: %s)\n", expandedPath, indexFormat)
			sessions, messages := indexCustomPath(expandedPath, indexFormat)
			fmt.Printf("  ✓ Custom (%s): %d sessions, %d messages\n", indexFormat, sessions, messages)
			fmt.Printf("\nIndex saved to: ~/.mnemo/mnemo.db\n")
			return
		}

		totalSessions := 0
		totalMessages := 0

		shouldIndex := func(toolName string) bool {
			if indexTool == "" {
				return true
			}
			return strings.EqualFold(indexTool, toolName)
		}

		// Index Claude Code - main directory (legacy path)
		claudePath := filepath.Join(home, ".claude", "projects")
		if pathExists(claudePath) && shouldIndex("claude") {
			sessions, messages := indexClaudeCode(claudePath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Claude Code (projects): %d sessions, %d messages\n", sessions, messages)
		}

		// Index Claude Code - XDG path (Claude Code v1.0.30+)
		xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
		if xdgConfigHome == "" {
			xdgConfigHome = filepath.Join(home, ".config")
		}
		claudeXDGPath := filepath.Join(xdgConfigHome, "claude", "projects")
		if pathExists(claudeXDGPath) && claudeXDGPath != claudePath && shouldIndex("claude") {
			sessions, messages := indexClaudeCode(claudeXDGPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Claude Code (XDG): %d sessions, %d messages\n", sessions, messages)
		}

		// Index Claude Code - transcripts directory
		claudeTranscripts := filepath.Join(home, ".claude", "transcripts")
		if pathExists(claudeTranscripts) && shouldIndex("claude") {
			sessions, messages := indexClaudeCode(claudeTranscripts)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Claude Code (transcripts): %d sessions, %d messages\n", sessions, messages)
		}

		// Index Claude Code - backup directory (for users who reinstalled)
		claudeBackup := filepath.Join(home, ".claude-backup")
		if pathExists(claudeBackup) && shouldIndex("claude") {
			sessions, messages := indexClaudeCode(claudeBackup)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Claude Code (backup): %d sessions, %d messages\n", sessions, messages)
		}

		// Index Opencode
		opencodePath := filepath.Join(home, ".local", "share", "opencode")
		if pathExists(opencodePath) && shouldIndex("opencode") {
			sessions, messages := indexOpencode(opencodePath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Opencode: %d sessions, %d messages\n", sessions, messages)
		}

		// Index Gemini CLI
		geminiSessionsPath := filepath.Join(home, ".gemini", "sessions")
		if pathExists(geminiSessionsPath) && shouldIndex("gemini") {
			sessions, messages := indexGeminiCLI(geminiSessionsPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Gemini CLI: %d sessions, %d messages\n", sessions, messages)
		}

		// Index Cursor (new globalStorage format)
		cursorGlobalPath := filepath.Join(appSupportDir(home, "Cursor"), "User", "globalStorage", "state.vscdb")
		if pathExists(cursorGlobalPath) && shouldIndex("cursor") {
			sessions, messages := indexCursorGlobalStorage(cursorGlobalPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Cursor: %d sessions, %d messages\n", sessions, messages)
		}

		// Index VS Code extensions (Kilo Code, Cline, Roo Code) across ALL IDEs
		vscodeIDEs := []string{"Code", "Code - Insiders", "Cursor", "Windsurf", "VSCodium", "Antigravity", "Kiro", "Trae"}
		clineExtensions := []struct {
			ExtID    string
			ToolName string
		}{
			{"kilocode.kilo-code", "kilo-code"},
			{"saoudrizwan.claude-dev", "cline"},
			{"rooveterinaryinc.roo-cline", "roo-code"},
		}

		for _, ext := range clineExtensions {
			if !shouldIndex(ext.ToolName) {
				continue
			}
			extSessions := 0
			extMessages := 0
			for _, ide := range vscodeIDEs {
				tasksPath := vscodeExtTasksPath(home, ide, ext.ExtID)
				if pathExists(tasksPath) {
					s, m := indexClineFamily(tasksPath, ext.ToolName)
					extSessions += s
					extMessages += m
				}
			}
			if extSessions > 0 {
				totalSessions += extSessions
				totalMessages += extMessages
				fmt.Printf("  ✓ %s: %d sessions, %d messages\n", ext.ToolName, extSessions, extMessages)
			}
		}

		// Index Crush CLI - main directory
		if shouldIndex("crush") {
			crushPath := filepath.Join(home, ".crush", "crush.db")
			crushSessions := 0
			crushMessages := 0
			if pathExists(crushPath) {
				s, m := indexCrush(crushPath)
				crushSessions += s
				crushMessages += m
			}

			crushSessions, crushMessages = scanCrushPerProject(home, crushSessions, crushMessages)
			if crushSessions > 0 {
				totalSessions += crushSessions
				totalMessages += crushMessages
				fmt.Printf("  ✓ Crush: %d sessions, %d messages\n", crushSessions, crushMessages)
			}
		}

		// Index Antigravity (code_tracker JSONL files in ~/.gemini/antigravity/)
		antigravityPath := filepath.Join(home, ".gemini", "antigravity", "code_tracker", "active")
		if pathExists(antigravityPath) && shouldIndex("antigravity") {
			sessions, messages := indexAntigravityCodeTracker(antigravityPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Antigravity: %d sessions, %d messages\n", sessions, messages)
		}

		// Index Kiro (stores JSON files in globalStorage/kiro.kiroagent/workspace-sessions/)
		kiroPath := filepath.Join(appSupportDir(home, "Kiro"), "User", "globalStorage", "kiro.kiroagent", "workspace-sessions")
		if pathExists(kiroPath) && shouldIndex("kiro") {
			sessions, messages := indexKiro(kiroPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Kiro: %d sessions, %d messages\n", sessions, messages)
		}

		// Index Amp CLI
		ampPath := filepath.Join(home, ".local", "share", "amp")
		if pathExists(ampPath) && shouldIndex("amp") {
			sessions, messages := indexAmp(ampPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Amp: %d sessions, %d messages\n", sessions, messages)
		}

		// Index Codex CLI
		codexPath := filepath.Join(home, ".codex")
		if pathExists(codexPath) && shouldIndex("codex") {
			sessions, messages := indexCodex(codexPath)
			totalSessions += sessions
			totalMessages += messages
			fmt.Printf("  ✓ Codex: %d sessions, %d messages\n", sessions, messages)
		}

		fmt.Println()
		fmt.Printf("Total: %d sessions, %d messages indexed\n", totalSessions, totalMessages)
		if indexErrors > 0 {
			fmt.Printf("  (Skipped %d messages due to errors)\n", indexErrors)
		}
		fmt.Printf("Index saved to: ~/.mnemo/mnemo.db\n")

		if err := populateProjectsFromSessions(); err != nil {
			fmt.Printf("  (Warning: could not populate projects: %v)\n", err)
		} else {
			if err := db.ClassifyProjects(); err == nil {
				active, inactive, _ := db.GetProjectsForOnboarding()
				if len(active)+len(inactive) > 0 {
					fmt.Printf("  → %d active projects, %d inactive projects discovered\n", len(active), len(inactive))
				}
			}
		}

		// Warn if fewer than 100 sessions found
		if totalSessions < 100 && totalSessions > 0 {
			fmt.Println()
			fmt.Println("⚠️  Only", totalSessions, "sessions found. This seems low.")
			fmt.Println("   If you have more sessions, check if they're in a different location:")
			fmt.Println("   - ~/.claude-backup/ (if you reinstalled Claude)")
			fmt.Println("   - Different user account")
			fmt.Println("   - External drive")
			fmt.Println()
			fmt.Println("   Use 'mnemo index --path /custom/path' to index additional locations.")
		}

		fmt.Println()
		fmt.Println("Run 'mnemo search <query>' to search your history.")

		// Clean up .indexing status file (background index complete)
		mnemoDir := filepath.Join(home, ".mnemo")
		_ = os.Remove(filepath.Join(mnemoDir, ".indexing"))
	},
}

func runOnboarding() {
	// Only index last 30 days during onboarding for speed
	indexCutoff = time.Now().AddDate(0, -1, 0)

	if err := db.InitDB(); err != nil {
		fmt.Printf("Error initializing database: %v\n", err)
		os.Exit(1)
	}
	defer db.CloseDB()

	home, _ := os.UserHomeDir()
	start := time.Now()

	// Inline styles
	cyan := lipgloss.NewStyle().Foreground(lipgloss.Color("#00D9FF")).Bold(true)
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#94a3b8"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80"))

	// ASCII art banner
	banner := cyan.Render(`##     ## ##    ## ######## ##     ##  #######
###   ### ###   ## ##       ###   ### ##     ##
#### #### ####  ## ##       #### #### ##     ##
## ### ## ## ## ## ######   ## ### ## ##     ##
##     ## ##  #### ##       ##     ## ##     ##
##     ## ##   ### ##       ##     ## ##     ##
##     ## ##    ## ######## ##     ##  #######`)
	fmt.Println()
	fmt.Println(banner)
	fmt.Printf("  %s\n", dim.Render("by Pilan"))
	fmt.Println()
	fmt.Printf("  First run — indexing recent history...\n\n")

	// Brand colors per tool
	brandColor := map[string]lipgloss.Style{
		"Claude Code":       lipgloss.NewStyle().Foreground(lipgloss.Color("#da7756")),
		"OpenCode":          lipgloss.NewStyle().Foreground(lipgloss.Color("#00dc82")),
		"Gemini CLI":        lipgloss.NewStyle().Foreground(lipgloss.Color("#4285F4")),
		"Cursor":            lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC")),
		"Codex":             lipgloss.NewStyle().Foreground(lipgloss.Color("#10a37f")),
		"Amp":               lipgloss.NewStyle().Foreground(lipgloss.Color("#F34E3F")),
		"Kiro":              lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900")),
		"Crush":             lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6EC7")),
		"Antigravity":       lipgloss.NewStyle().Foreground(lipgloss.Color("#8AB4F8")),
		"VS Code Extensions": lipgloss.NewStyle().Foreground(lipgloss.Color("#007ACC")),
	}

	// XDG config for Claude Code
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		xdgConfigHome = filepath.Join(home, ".config")
	}

	type toolEntry struct {
		name string
		scan func() (int, int)
	}

	tools := []toolEntry{
		{"Claude Code", func() (int, int) {
			s, m := 0, 0
			for _, p := range []string{
				filepath.Join(home, ".claude", "projects"),
				filepath.Join(xdgConfigHome, "claude", "projects"),
				filepath.Join(home, ".claude", "transcripts"),
			} {
				if pathExists(p) {
					a, b := indexClaudeCode(p)
					s += a
					m += b
				}
			}
			return s, m
		}},
		{"OpenCode", func() (int, int) {
			p := filepath.Join(home, ".local", "share", "opencode")
			if pathExists(p) {
				return indexOpencode(p)
			}
			return 0, 0
		}},
		{"Gemini CLI", func() (int, int) {
			p := filepath.Join(home, ".gemini", "sessions")
			if pathExists(p) {
				return indexGeminiCLI(p)
			}
			return 0, 0
		}},
		{"Cursor", func() (int, int) {
			p := filepath.Join(appSupportDir(home, "Cursor"), "User", "globalStorage", "state.vscdb")
			if pathExists(p) {
				return indexCursorGlobalStorage(p)
			}
			return 0, 0
		}},
		{"Codex", func() (int, int) {
			p := filepath.Join(home, ".codex")
			if pathExists(p) {
				return indexCodex(p)
			}
			return 0, 0
		}},
		{"Amp", func() (int, int) {
			p := filepath.Join(home, ".local", "share", "amp")
			if pathExists(p) {
				return indexAmp(p)
			}
			return 0, 0
		}},
		{"Kiro", func() (int, int) {
			p := filepath.Join(appSupportDir(home, "Kiro"), "User", "globalStorage", "kiro.kiroagent", "workspace-sessions")
			if pathExists(p) {
				return indexKiro(p)
			}
			return 0, 0
		}},
		{"Crush", func() (int, int) {
			s, m := 0, 0
			crushPath := filepath.Join(home, ".crush", "crush.db")
			if pathExists(crushPath) {
				a, b := indexCrush(crushPath)
				s += a
				m += b
			}
			a, b := scanCrushPerProject(home, 0, 0)
			s += a
			m += b
			return s, m
		}},
		{"Antigravity", func() (int, int) {
			p := filepath.Join(home, ".gemini", "antigravity", "code_tracker", "active")
			if pathExists(p) {
				return indexAntigravityCodeTracker(p)
			}
			return 0, 0
		}},
		{"VS Code Extensions", func() (int, int) {
			s, m := 0, 0
			ides := []string{"Code", "Code - Insiders", "Cursor", "Windsurf", "VSCodium", "Antigravity", "Kiro", "Trae"}
			exts := []struct{ ExtID, ToolName string }{
				{"kilocode.kilo-code", "kilo-code"},
				{"saoudrizwan.claude-dev", "cline"},
				{"rooveterinaryinc.roo-cline", "roo-code"},
			}
			for _, ext := range exts {
				for _, ide := range ides {
					tasksPath := vscodeExtTasksPath(home, ide, ext.ExtID)
					if pathExists(tasksPath) {
						a, b := indexClineFamily(tasksPath, ext.ToolName)
						s += a
						m += b
					}
				}
			}
			return s, m
		}},
	}

	totalSessions := 0
	totalMessages := 0
	toolCount := 0
	spinFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

	for _, tool := range tools {
		// Run scan in goroutine so we can animate the spinner
		type scanResult struct{ s, m int }
		ch := make(chan scanResult, 1)
		scanFn := tool.scan
		go func() {
			s, m := scanFn()
			ch <- scanResult{s, m}
		}()

		// Resolve brand color for this tool (fallback to cyan)
		toolStyle, ok := brandColor[tool.name]
		if !ok {
			toolStyle = cyan
		}
		styledName := toolStyle.Render(fmt.Sprintf("%-22s", tool.name))

		// Animate spinner while scan runs
		ticker := time.NewTicker(80 * time.Millisecond)
		frame := 0
		fmt.Printf("  %s %s scanning...", cyan.Render(spinFrames[0]), styledName)

		var r scanResult
	waitLoop:
		for {
			select {
			case r = <-ch:
				ticker.Stop()
				break waitLoop
			case <-ticker.C:
				frame++
				fmt.Printf("\033[2K\r  %s %s scanning...",
					cyan.Render(spinFrames[frame%len(spinFrames)]), styledName)
			}
		}

		if r.s > 0 {
			fmt.Printf("\033[2K\r  %s %s %s   %s\n",
				green.Render("✓"), styledName,
				cyan.Render(fmt.Sprintf("%d sessions", r.s)),
				dim.Render(fmt.Sprintf("%d messages", r.m)))
			totalSessions += r.s
			totalMessages += r.m
			toolCount++
		} else {
			fmt.Printf("\033[2K\r  %s %s %s\n",
				dim.Render("-"), styledName, dim.Render("not found"))
		}
	}

	elapsed := time.Since(start)
	fmt.Println()
	fmt.Printf("  %s | %s | %d tools | %s\n",
		cyan.Render(fmt.Sprintf("%d sessions", totalSessions)),
		cyan.Render(fmt.Sprintf("%d messages", totalMessages)),
		toolCount,
		dim.Render(fmt.Sprintf("%.1fs", elapsed.Seconds())))

	_ = populateProjectsFromSessions()
	_ = db.ClassifyProjects()

	// Surface discoveries from indexed data
	gold := lipgloss.NewStyle().Foreground(lipgloss.Color("#f59e0b"))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color("#f59e0b")).Bold(true)

	type discovery struct {
		label  string
		detail string
		score  float64
	}

	var discoveries []discovery

	// Longest session in the last 30 days
	var longestMsgs int
	var longestTool, longestProject, longestQuery string
	row := db.GetDB().QueryRow(`
		SELECT message_count, tool, COALESCE(working_directory, project, ''), COALESCE(first_query, '')
		FROM sessions
		WHERE message_count > 0
		ORDER BY message_count DESC LIMIT 1`)
	if row.Scan(&longestMsgs, &longestTool, &longestProject, &longestQuery) == nil && longestMsgs > 10 {
		detail := fmt.Sprintf("%d messages", longestMsgs)
		if longestProject != "" {
			parts := strings.Split(longestProject, "/")
			detail += " in " + parts[len(parts)-1]
		}
		detail += " (" + longestTool + ")"
		discoveries = append(discoveries, discovery{
			label:  "Longest session",
			detail: detail,
			score:  math.Log(float64(longestMsgs)),
		})
	}

	// Most-discussed project in the last 30 days
	var topProject string
	var topProjectSessions, topProjectMsgs int
	row = db.GetDB().QueryRow(`
		SELECT COALESCE(working_directory, project) as proj,
		       COUNT(*) as sess, SUM(message_count) as msgs
		FROM sessions
		WHERE proj != ''
		GROUP BY proj
		ORDER BY msgs DESC LIMIT 1`)
	if row.Scan(&topProject, &topProjectSessions, &topProjectMsgs) == nil && topProjectSessions > 1 {
		parts := strings.Split(topProject, "/")
		projName := parts[len(parts)-1]
		discoveries = append(discoveries, discovery{
			label:  "Most discussed",
			detail: fmt.Sprintf("%s — %d sessions, %d messages", projName, topProjectSessions, topProjectMsgs),
			score:  math.Log(float64(topProjectMsgs)) * 1.5,
		})
	}

	// Busiest day in the last 30 days
	var busiestDay string
	var busiestSessions, busiestMsgs int
	row = db.GetDB().QueryRow(`
		SELECT date(COALESCE(start_time, indexed_at)) as day,
		       COUNT(*) as sess, SUM(message_count) as msgs
		FROM sessions
		WHERE day IS NOT NULL
		GROUP BY day
		ORDER BY msgs DESC LIMIT 1`)
	if row.Scan(&busiestDay, &busiestSessions, &busiestMsgs) == nil && busiestSessions > 2 {
		discoveries = append(discoveries, discovery{
			label:  "Busiest day",
			detail: fmt.Sprintf("%s — %d sessions, %d messages", busiestDay, busiestSessions, busiestMsgs),
			score:  math.Log(float64(busiestMsgs)) * 0.8,
		})
	}

	// Tool distribution (if multiple tools found)
	if toolCount >= 2 {
		var toolDist []string
		rows, err := db.GetDB().Query(`
			SELECT tool, COUNT(*) as sess
			FROM sessions
			GROUP BY tool
			ORDER BY sess DESC LIMIT 3`)
		if err == nil {
			for rows.Next() {
				var t string
				var s int
				if rows.Scan(&t, &s) == nil {
					toolDist = append(toolDist, fmt.Sprintf("%s (%d)", t, s))
				}
			}
			_ = rows.Close()
			if len(toolDist) >= 2 {
				discoveries = append(discoveries, discovery{
					label:  "Tool mix",
					detail: strings.Join(toolDist, ", "),
					score:  float64(len(toolDist)) * 1.2,
				})
			}
		}
	}

	// Show top 3 discoveries by score
	if len(discoveries) > 0 {
		// Sort by score descending
		for i := 0; i < len(discoveries); i++ {
			for j := i + 1; j < len(discoveries); j++ {
				if discoveries[j].score > discoveries[i].score {
					discoveries[i], discoveries[j] = discoveries[j], discoveries[i]
				}
			}
		}
		if len(discoveries) > 3 {
			discoveries = discoveries[:3]
		}

		fmt.Println()
		fmt.Printf("  %s\n", dim.Render("From the last 30 days:"))
		for _, d := range discoveries {
			fmt.Printf("  %s %s %s\n",
				gold.Render("◆"),
				dim.Render(d.label+":"),
				accent.Render(d.detail))
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Clear cutoff for any subsequent operations
	indexCutoff = time.Time{}

	// Y/n prompt for plugin install
	fmt.Println()
	fmt.Printf("Install mnemo plugins for your AI tools? [Y/n] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "" || answer == "y" || answer == "yes" {
		results := runInstallPlugins(home)
		for _, r := range results {
			fmt.Println(r)
		}
	}

	// Spawn full index in background for complete history
	fmt.Println()
	if exe, err := os.Executable(); err == nil {
		bgCmd := exec.Command(exe, "index")
		bgCmd.Stdout = nil
		bgCmd.Stderr = nil
		bgCmd.Stdin = nil
		if bgCmd.Start() == nil {
			// Write .indexing status file
			mnemoDir := filepath.Join(home, ".mnemo")
			indexingPath := filepath.Join(mnemoDir, ".indexing")
			indexingData, _ := json.Marshal(map[string]interface{}{
				"pid":     bgCmd.Process.Pid,
				"started": time.Now().Format(time.RFC3339),
			})
			_ = os.WriteFile(indexingPath, indexingData, 0644)
			_ = bgCmd.Process.Release()

			fmt.Printf("Full history indexing in background.\n")
			fmt.Printf("  Check progress: %s\n", cyan.Render("mnemo status"))
		}
	}
	fmt.Println()
}

func indexCustomPath(customPath, format string) (int, int) {
	switch strings.ToLower(format) {
	case "claude":
		return indexClaudeCode(customPath)
	case "opencode":
		return indexOpencode(customPath)
	case "codex":
		return indexCodex(customPath)
	case "amp":
		return indexAmp(customPath)
	case "gemini":
		return indexGeminiCLI(customPath)
	case "kiro":
		return indexKiro(customPath)
	case "antigravity":
		return indexAntigravityCodeTracker(customPath)
	case "crush":
		return indexCrush(customPath)
	case "cline", "kilo", "kilo-code", "roo", "roo-code":
		return indexClineFamily(customPath, format)
	default:
		fmt.Printf("Unknown format: %s\n", format)
		fmt.Println("Supported formats: claude, opencode, cursor, codex, amp, gemini, kiro, antigravity, crush, cline, kilo, roo")
		return 0, 0
	}
}

func populateProjectsFromSessions() error {
	home, _ := os.UserHomeDir()

	rows, err := db.GetDB().Query(`
		SELECT DISTINCT working_directory, MAX(COALESCE(start_time, indexed_at)) as last_activity
		FROM sessions
		WHERE working_directory != '' AND working_directory IS NOT NULL
		GROUP BY working_directory
	`)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var workDir string
		var lastActivity time.Time
		if err := rows.Scan(&workDir, &lastActivity); err != nil {
			continue
		}

		workDir = strings.TrimSpace(workDir)
		if workDir == "" {
			continue
		}

		if strings.HasPrefix(workDir, "~") {
			workDir = filepath.Join(home, workDir[1:])
		}

		if !filepath.IsAbs(workDir) {
			continue
		}

		if lastActivity.IsZero() {
			lastActivity = time.Now()
		}

		_ = db.UpsertProject(workDir, lastActivity)
	}

	return nil
}

func init() {
	indexCmd.Flags().BoolVarP(&indexAll, "all", "a", true, "Index all detected tools")
	indexCmd.Flags().StringVarP(&indexTool, "tool", "t", "", "Index specific tool only")
	indexCmd.Flags().BoolVarP(&indexForce, "force", "f", false, "Force re-index (clear existing)")
	indexCmd.Flags().StringVarP(&indexPath, "path", "p", "", "Custom path to index (requires --format)")
	indexCmd.Flags().StringVar(&indexFormat, "format", "", "Format for custom path: claude, opencode, codex, amp, gemini, cline, kiro, antigravity")
	rootCmd.AddCommand(indexCmd)
}
