#!/bin/sh
# PreToolUse(Agent) hook: no subagent runs without the maintainer saying yes
# (ADR-0057).
#
# Delegation is cheap to start and expensive to finish: a subagent re-derives
# the repo cold, misses the prompt cache, and reports its work optimistically.
# Whether that trade is worth it is the maintainer's call, so every Agent call
# becomes a permission prompt, the built-in Explore and Plan agents included.
#
# Reads the tool call as JSON on stdin and always answers "ask", naming the
# agent and the task so the prompt says what is being approved. Without jq it
# stays silent, which falls back to the normal permission flow.
set -eu

command -v jq >/dev/null 2>&1 || exit 0

input=$(cat)
agent=$(printf '%s' "$input" | jq -r '.tool_input.subagent_type // "general-purpose"' 2>/dev/null) || exit 0
desc=$(printf '%s' "$input" | jq -r '.tool_input.description // "(no description)"' 2>/dev/null) || exit 0

jq -n --arg r "Delegate to '$agent': $desc

Worth it only if this would otherwise flood the main context. Approving this call approves nothing after it." \
  '{hookSpecificOutput:{hookEventName:"PreToolUse",permissionDecision:"ask",permissionDecisionReason:$r}}'
