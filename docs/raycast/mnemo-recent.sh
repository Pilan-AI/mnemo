#!/bin/bash

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

mnemo recent -d 7 2>&1

exit 0
