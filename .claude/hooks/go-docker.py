#!/usr/bin/env python3
"""
Hook to intercept Go commands and run them in a Go 1.25 Docker container.
"""
import json
import sys
import re

try:
    input_data = json.load(sys.stdin)
except json.JSONDecodeError:
    sys.exit(0)

tool_name = input_data.get("tool_name", "")
tool_input = input_data.get("tool_input", {})
command = tool_input.get("command", "")

# Only process Bash tool calls
if tool_name != "Bash":
    sys.exit(0)

# Check if command starts with 'go ' (go test, go build, go run, etc.)
if re.match(r'^\s*go\s+', command):
    # Wrap the go command in a Docker container
    docker_cmd = f'docker run --rm -v "$(pwd):/app" -w /app golang:1.25 {command}'

    output = {
        "hookSpecificOutput": {
            "hookEventName": "PreToolUse",
            "permissionDecision": "allow",
            "updatedInput": {
                "command": docker_cmd
            },
            "permissionDecisionReason": "Running Go command in golang:1.25 container"
        }
    }
    print(json.dumps(output))

sys.exit(0)
