package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install mnemo plugins for AI tools",
	Long:  "Install mnemo plugins for Claude Code and OpenCode to enable automatic context injection.",
	Run: func(cmd *cobra.Command, args []string) {
		home, _ := os.UserHomeDir()

		fmt.Println("Installing mnemo plugins...")
		fmt.Println()

		// Install Claude Code skill
		claudeSkillDir := filepath.Join(home, ".claude", "skills", "mnemo")
		if err := os.MkdirAll(claudeSkillDir, 0755); err != nil {
			fmt.Printf("  ✗ Claude Code: Failed to create directory: %v\n", err)
		} else {
			skillPath := filepath.Join(claudeSkillDir, "SKILL.md")
			if err := os.WriteFile(skillPath, []byte(claudeCodeSkill), 0644); err != nil {
				fmt.Printf("  ✗ Claude Code: Failed to write skill: %v\n", err)
			} else {
				fmt.Printf("  ✓ Claude Code: %s\n", claudeSkillDir)
			}
		}

		// Install OpenCode plugin
		opencodePluginDir := filepath.Join(home, ".config", "opencode", "plugins", "mnemo")
		if err := os.MkdirAll(opencodePluginDir, 0755); err != nil {
			fmt.Printf("  ✗ OpenCode: Failed to create directory: %v\n", err)
		} else {
			pluginPath := filepath.Join(opencodePluginDir, "mnemo-plugin.ts")
			pkgPath := filepath.Join(opencodePluginDir, "package.json")

			if err := os.WriteFile(pluginPath, []byte(opencodePlugin), 0644); err != nil {
				fmt.Printf("  ✗ OpenCode: Failed to write plugin: %v\n", err)
			} else if err := os.WriteFile(pkgPath, []byte(opencodePackageJSON), 0644); err != nil {
				fmt.Printf("  ✗ OpenCode: Failed to write package.json: %v\n", err)
			} else {
				fmt.Printf("  ✓ OpenCode: %s\n", opencodePluginDir)
			}
		}

		fmt.Println()
		fmt.Println("Plugins installed! Your AI tools now have access to mnemo memory.")
		fmt.Println()
		fmt.Println("Features enabled:")
		fmt.Println("  • Claude Code: Skill auto-activates on context keywords")
		fmt.Println("  • OpenCode: Context survives session compaction")
	},
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
