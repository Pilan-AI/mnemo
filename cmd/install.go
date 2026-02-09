package cmd

// install.go sets up mnemo integrations for AI coding tools:
//
//   - Claude Code: Installs a skill at ~/.claude/skills/mnemo/ that auto-activates
//     on context-related keywords
//   - Claude Desktop: Adds mnemo as an MCP server in claude_desktop_config.json
//   - OpenCode: Installs a plugin that injects mnemo context during session compaction

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install mnemo plugins and MCP server for AI tools",
	Long: `Install mnemo plugins for Claude Code and OpenCode to enable automatic context injection.

This command will:
  1. Install Claude Code skill for context keywords
  2. Install OpenCode plugin for session compaction
  3. Configure MCP server in Claude Desktop (if installed)

The MCP server provides tools like mnemo_search, mnemo_context, and mnemo_recent
directly in Claude Desktop.`,
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Printf("Error: cannot determine home directory: %v\n", err)
			return
		}

		fmt.Println("Installing mnemo plugins and MCP server...")
		fmt.Println()

		results := runInstallPlugins(home)
		for _, r := range results {
			fmt.Println(r)
		}

		fmt.Println()
		fmt.Println("Installation complete!")
		fmt.Println()
		fmt.Println("Features enabled:")
		fmt.Println("  - Claude Code: Skill auto-activates on context keywords")
		fmt.Println("  - Claude Desktop: MCP tools (mnemo_search, mnemo_context, mnemo_recent)")
		fmt.Println("  - OpenCode: Context survives session compaction")
		fmt.Println("  - Raycast: Search, context, and recent commands (macOS)")
		fmt.Println("  - Background indexer: Sessions re-indexed every 30 minutes")
		fmt.Println()
		fmt.Println("Note: Restart Claude Desktop to activate MCP server.")
	},
}

// installMCPConfig adds or updates mnemo MCP server in Claude Desktop config
func installMCPConfig(configPath, mnemoPath string) error {
	// Read existing config or create new one
	var config map[string]interface{}

	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse existing config: %w", err)
		}
	} else if os.IsNotExist(err) {
		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		config = make(map[string]interface{})
	} else {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Ensure mcpServers exists
	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
		config["mcpServers"] = mcpServers
	}

	// Add/update mnemo server
	mcpServers["mnemo"] = map[string]interface{}{
		"command": mnemoPath,
		"args":    []string{"serve"},
	}

	// Write config back
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// runInstallPlugins installs all mnemo integrations and returns result strings.
// Used by both the install command and onboarding.
func runInstallPlugins(home string) []string {
	var results []string

	mnemoPath, err := exec.LookPath("mnemo")
	if err != nil {
		for _, p := range []string{
			filepath.Join(home, "bin", "mnemo"),
			filepath.Join(home, ".local", "bin", "mnemo"),
			"/usr/local/bin/mnemo",
			"/opt/homebrew/bin/mnemo",
		} {
			if _, err := os.Stat(p); err == nil {
				mnemoPath = p
				break
			}
		}
	}
	if mnemoPath == "" {
		mnemoPath = "mnemo"
	}

	// Claude Code skill
	claudeSkillDir := filepath.Join(home, ".claude", "skills", "mnemo")
	if err := os.MkdirAll(claudeSkillDir, 0755); err == nil {
		skillPath := filepath.Join(claudeSkillDir, "SKILL.md")
		if err := os.WriteFile(skillPath, []byte(claudeCodeSkill), 0644); err == nil {
			results = append(results, "  ✓ Claude Code skill installed")
		}
	}

	// MCP server for Claude Desktop
	var claudeDesktopConfigDir string
	switch runtime.GOOS {
	case "darwin":
		claudeDesktopConfigDir = filepath.Join(home, "Library", "Application Support", "Claude")
	case "windows":
		claudeDesktopConfigDir = filepath.Join(os.Getenv("APPDATA"), "Claude")
	default:
		claudeDesktopConfigDir = filepath.Join(home, ".config", "claude")
	}
	configPath := filepath.Join(claudeDesktopConfigDir, "claude_desktop_config.json")
	if err := installMCPConfig(configPath, mnemoPath); err == nil {
		results = append(results, "  ✓ MCP server configured")
	}

	// OpenCode plugin
	opencodePluginDir := filepath.Join(home, ".config", "opencode", "plugins", "mnemo")
	if err := os.MkdirAll(opencodePluginDir, 0755); err == nil {
		pluginPath := filepath.Join(opencodePluginDir, "mnemo-plugin.ts")
		pkgPath := filepath.Join(opencodePluginDir, "package.json")
		if os.WriteFile(pluginPath, []byte(opencodePlugin), 0644) == nil &&
			os.WriteFile(pkgPath, []byte(opencodePackageJSON), 0644) == nil {
			results = append(results, "  ✓ OpenCode plugin installed")
		}
	}

	// Background indexer (periodic re-index every 30 min)
	if r := installBackgroundIndexer(home, mnemoPath); r != "" {
		results = append(results, r)
	}

	return results
}

// installBackgroundIndexer sets up a periodic background index.
// On macOS: launchd agent. On Linux: systemd user timer. On Windows: skipped.
func installBackgroundIndexer(home, mnemoPath string) string {
	switch runtime.GOOS {
	case "darwin":
		return installLaunchdAgent(home, mnemoPath)
	case "linux":
		return installSystemdTimer(home, mnemoPath)
	default:
		return ""
	}
}

func installLaunchdAgent(home, mnemoPath string) string {
	label := "com.pilan.mnemo-index"
	plistDir := filepath.Join(home, "Library", "LaunchAgents")
	plistPath := filepath.Join(plistDir, label+".plist")

	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return ""
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>index</string>
    </array>
    <key>StartInterval</key>
    <integer>1800</integer>
    <key>StandardOutPath</key>
    <string>/dev/null</string>
    <key>StandardErrorPath</key>
    <string>/dev/null</string>
    <key>RunAtLoad</key>
    <false/>
</dict>
</plist>`, label, mnemoPath)

	// Skip write+load if plist already exists with identical content.
	// Re-registering triggers macOS "Background Activity" notifications.
	if existing, err := os.ReadFile(plistPath); err == nil && string(existing) == plist {
		return "  ✓ Background indexer active (every 30 min)"
	}

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return ""
	}

	// Load the agent (unload first if exists, ignore errors)
	_ = exec.Command("launchctl", "unload", plistPath).Run()
	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return "  ✓ Background indexer plist written (load manually: launchctl load " + plistPath + ")"
	}

	return "  ✓ Background indexer active (every 30 min)"
}

func installSystemdTimer(home, mnemoPath string) string {
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return ""
	}

	service := fmt.Sprintf(`[Unit]
Description=mnemo index - reindex AI coding sessions

[Service]
Type=oneshot
ExecStart=%s index
`, mnemoPath)

	timer := `[Unit]
Description=mnemo periodic reindex

[Timer]
OnBootSec=5min
OnUnitActiveSec=30min

[Install]
WantedBy=timers.target
`

	servicePath := filepath.Join(unitDir, "mnemo-index.service")
	timerPath := filepath.Join(unitDir, "mnemo-index.timer")

	// Skip if both files already exist with identical content
	existingService, sErr := os.ReadFile(servicePath)
	existingTimer, tErr := os.ReadFile(timerPath)
	if sErr == nil && tErr == nil && string(existingService) == service && string(existingTimer) == timer {
		return "  ✓ Background indexer active (every 30 min)"
	}

	if os.WriteFile(servicePath, []byte(service), 0644) != nil {
		return ""
	}
	if os.WriteFile(timerPath, []byte(timer), 0644) != nil {
		return ""
	}

	// Enable and start the timer
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	_ = exec.Command("systemctl", "--user", "enable", "mnemo-index.timer").Run()
	if err := exec.Command("systemctl", "--user", "start", "mnemo-index.timer").Run(); err != nil {
		return "  ✓ Systemd timer written (enable: systemctl --user enable --now mnemo-index.timer)"
	}

	return "  ✓ Background indexer active (every 30 min)"
}

// installRaycastScripts copies mnemo script commands into Raycast's directory.
// Only runs if Raycast.app is detected on macOS.
func installRaycastScripts(home, mnemoPath string) string {
	raycastApp := "/Applications/Raycast.app"
	if _, err := os.Stat(raycastApp); err != nil {
		return ""
	}

	scriptDir := filepath.Join(home, "Library", "Application Support", "Raycast", "Script Commands")
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		return ""
	}

	scripts := []struct {
		name    string
		content string
	}{
		{"mnemo-search.sh", fmt.Sprintf(`#!/bin/bash

# Required parameters:
# @raycast.schemaVersion 1
# @raycast.mode fullOutput

# Optional parameters:
# @raycast.icon 🔍
# @raycast.packageName Mnemo
# @raycast.title Mnemo: Search
# @raycast.description Search past AI sessions and knowledge
# @raycast.author 0xraghu
# @raycast.authorURL https://github.com/Pilan-AI/mnemo

# Documentation:
# @raycast.argument1 { "type": "text", "placeholder": "Enter search query" }

QUERY="$1"

if [ -z "$QUERY" ]; then
    echo "Usage: mnemo-search <query>"
    exit 1
fi

"%s" search "$QUERY" 2>&1

exit 0
`, mnemoPath)},
		{"mnemo-context.sh", fmt.Sprintf(`#!/bin/bash

# Required parameters:
# @raycast.schemaVersion 1
# @raycast.mode fullOutput

# Optional parameters:
# @raycast.icon 📦
# @raycast.packageName Mnemo
# @raycast.title Mnemo: Context
# @raycast.description Get mnemo context for a project
# @raycast.author 0xraghu
# @raycast.authorURL https://github.com/Pilan-AI/mnemo

# Documentation:
# @raycast.argument1 { "type": "text", "placeholder": "Project name" }

PROJECT="$1"

if [ -z "$PROJECT" ]; then
    echo "Usage: mnemo-context <project name>"
    exit 1
fi

"%s" context "$PROJECT" 2>&1

exit 0
`, mnemoPath)},
		{"mnemo-recent.sh", fmt.Sprintf(`#!/bin/bash

# Required parameters:
# @raycast.schemaVersion 1
# @raycast.mode fullOutput

# Optional parameters:
# @raycast.icon 📋
# @raycast.packageName Mnemo
# @raycast.title Mnemo: Recent
# @raycast.description Show recent AI coding sessions
# @raycast.author 0xraghu
# @raycast.authorURL https://github.com/Pilan-AI/mnemo

"%s" recent -d 7 2>&1

exit 0
`, mnemoPath)},
	}

	installed := 0
	for _, s := range scripts {
		scriptPath := filepath.Join(scriptDir, s.name)
		if err := os.WriteFile(scriptPath, []byte(s.content), 0755); err == nil {
			installed++
		}
	}

	if installed == len(scripts) {
		return "  ✓ Raycast script commands installed"
	}
	return ""
}

func init() {
	rootCmd.AddCommand(installCmd)
}

const claudeCodeSkill = `# Mnemo Project Memory

## Description
Automatically loads project history and context from past AI coding sessions using mnemo.

## Keywords
- project
- context
- history
- remember
- previous
- last time
- earlier
- before

## Instructions

You have access to the project's AI coding history through mnemo. Before exploring the codebase fresh, check if relevant context exists from past sessions.

### Available Commands

Run these via terminal to access project memory:

` + "```bash" + `
# Search past sessions for relevant context
mnemo search "<keywords>"

# Get context summary for current project
mnemo context "$(basename $(pwd))"

# List recent sessions
mnemo recent

# Re-index if sessions seem missing
mnemo index
` + "```" + `

### When to Use

1. **Starting a new session**: Run ` + "`mnemo context`" + ` to see what was discussed before
2. **Debugging issues**: Search for past discussions about the error/component
3. **Continuing work**: Find where you left off with ` + "`mnemo search`" + `
4. **Architecture questions**: Check if decisions were already made

### Important Notes

- Mnemo indexes Claude Code, OpenCode, and other AI tool sessions
- Search uses SQLite FTS5 with BM25 ranking
- Results show highlighted snippets with context
- Use ` + "`mnemo index --force`" + ` to rebuild if data seems stale
`

const opencodePlugin = `/**
 * Mnemo Plugin for OpenCode
 * Provides persistent project memory across sessions
 */

import { execSync } from 'child_process';
import { basename } from 'path';

function runMnemo(args) {
  try {
    return execSync('mnemo ' + args.join(' '), {
      encoding: 'utf-8',
      timeout: 10000,
    }).trim();
  } catch (error) {
    return 'Error: ' + error.message;
  }
}

function getProjectName(cwd) {
  return basename(cwd);
}

function getMnemoContext(project) {
  const context = runMnemo(['context', project]);
  if (context.includes('Error') || context.includes('No context')) {
    return '';
  }
  return context;
}

export default {
  name: 'mnemo',
  version: '1.0.0',
  description: 'Project memory from past AI coding sessions',

  tools: [
    {
      name: 'mnemo_search',
      description: 'Search past AI coding sessions for relevant context',
      parameters: {
        type: 'object',
        properties: {
          query: { type: 'string', description: 'Search query' },
          limit: { type: 'number', description: 'Max results (default: 10)' },
        },
        required: ['query'],
      },
      execute: async (params) => {
        return runMnemo(['search', params.query, '--limit', (params.limit || 10).toString()]);
      },
    },
    {
      name: 'mnemo_context',
      description: 'Get context summary for a project',
      parameters: {
        type: 'object',
        properties: {
          project: { type: 'string', description: 'Project name' },
        },
        required: ['project'],
      },
      execute: async (params) => {
        return getMnemoContext(params.project) || 'No context available.';
      },
    },
  ],

  experimental: {
    session: {
      compacting: async (summary, ctx) => {
        const project = getProjectName(ctx.cwd);
        const mnemoContext = getMnemoContext(project);
        if (!mnemoContext) return summary;
        return summary + '\n\n---\n## Project Memory (mnemo)\n' + mnemoContext + '\n---';
      },
    },
  },
};
`

const opencodePackageJSON = `{
  "name": "mnemo-opencode-plugin",
  "version": "1.0.0",
  "description": "Mnemo project memory plugin for OpenCode",
  "main": "mnemo-plugin.ts",
  "type": "module"
}
`
