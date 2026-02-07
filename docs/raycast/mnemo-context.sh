#!/bin/bash

# Required parameters:
# @raycast.schemaVersion 1
# @raycast.mode fullOutput

# Optional parameters:
# @raycast.icon 📦
# @raycast.packageName Mnemo
# @raycast.title Mnemo: Context
# @raycast.description Get mnemo context for current project
# @raycast.author 0xraghu
# @raycast.authorURL https://github.com/Pilan-AI/mnemo

PROJECT=$(basename "$(pwd)")

mnemo context "$PROJECT" 2>&1

exit 0
