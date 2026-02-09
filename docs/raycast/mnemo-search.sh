#!/bin/bash

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

# Ensure mnemo is in PATH (Raycast doesn't inherit shell PATH)
export PATH="$HOME/bin:$HOME/go/bin:$HOME/.local/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"

QUERY="$1"

if [ -z "$QUERY" ]; then
    echo "Usage: mnemo-search <query>"
    exit 1
fi

mnemo search "$QUERY" 2>&1

exit 0
