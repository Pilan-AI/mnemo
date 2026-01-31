#!/bin/bash

# Required parameters:
# @raycast.schemaVersion 1
# @raycast.mode silent

# Optional parameters:
# @raycast.icon 📦
# @raycast.packageName Mnemo
# @raycast.title Mnemo: Context
# @raycast.description Get mnemo context for current project
# @raycast.author Mnemo
# @raycast.authorURL https://github.com/Pilan-AI/mnemo

# Documentation:
# Get context for current directory

PROJECT=$(basename "$(pwd)")

/opt/homebrew/bin/mnemo context "$PROJECT" 2>&1

exit 0
