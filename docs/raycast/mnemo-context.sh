#!/bin/bash

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

# Ensure mnemo is in PATH (Raycast doesn't inherit shell PATH)
export PATH="$HOME/bin:$HOME/go/bin:$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"

PROJECT="$1"

if [ -z "$PROJECT" ]; then
    echo "Usage: mnemo-context <project name>"
    exit 1
fi

mnemo context "$PROJECT" 2>&1

exit 0
