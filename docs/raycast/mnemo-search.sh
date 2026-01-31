#!/bin/bash

# Required parameters:
# @raycast.schemaVersion 1
# @raycast.mode silent

# Optional parameters:
# @raycast.icon 🔍
# @raycast.packageName Mnemo
# @raycast.title Mnemo: Search
# @raycast.description Search past AI sessions and knowledge
# @raycast.author Mnemo
# @raycast.authorURL https://github.com/Pilan-AI/mnemo

# Documentation:
# @raycast.argument1 { "type": "text", "placeholder": "Enter search query" }

QUERY="$1"

if [ -n "$QUERY" ]; then
    echo "Usage: mnemo-search <query>"
    exit 1
fi

# Run mnemo search
/opt/homebrew/bin/mnemo search "$QUERY" 2>&1

exit 0
