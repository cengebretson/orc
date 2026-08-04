#!/usr/bin/env bash
# installed by orc
# managed by orc; reinstalling agent hooks overwrites this file.
# ORC_AGENT_HOOK_VERSION=1

set +e

engine="${1:-}"
lifecycle="${2:-}"
orc_bin="${3:-orc}"

[ -n "${ORC_AGENT_ID:-}" ] || exit 0
[ -n "${ORC_AGENT_INSTANCE:-}" ] || exit 0
[ -n "${TMUX_PANE:-}" ] || exit 0
[ -n "$engine" ] || exit 0
[ -n "$lifecycle" ] || exit 0
command -v python3 >/dev/null 2>&1 || exit 0

hook_input_file="$(mktemp "${TMPDIR:-/tmp}/orc-agent-hook.XXXXXX")" || exit 0
trap 'rm -f "$hook_input_file"' EXIT HUP INT TERM
cat >"$hook_input_file" 2>/dev/null || true

ORC_HOOK_ENGINE="$engine" \
ORC_HOOK_LIFECYCLE="$lifecycle" \
ORC_HOOK_INPUT_FILE="$hook_input_file" \
ORC_HOOK_BIN="$orc_bin" \
python3 - <<'PY' >/dev/null 2>&1 || true
import hashlib
import json
import os
import subprocess

engine = os.environ.get("ORC_HOOK_ENGINE", "")
lifecycle = os.environ.get("ORC_HOOK_LIFECYCLE", "")
input_path = os.environ.get("ORC_HOOK_INPUT_FILE", "")
orc_bin = os.environ.get("ORC_HOOK_BIN", "orc")
try:
    command_timeout = min(10.0, max(0.1, float(os.environ.get("ORC_HOOK_TIMEOUT", "5"))))
except ValueError:
    command_timeout = 5.0

try:
    with open(input_path, encoding="utf-8") as handle:
        payload = json.load(handle)
except Exception:
    payload = {}

# A subagent event must never move the lifecycle of the pane's primary agent.
if payload.get("agent_id"):
    raise SystemExit(0)

session_id = payload.get("session_id")
if not isinstance(session_id, str):
    session_id = ""

identity = {
    "engine": engine,
    "state": lifecycle,
    "session_id": session_id,
    "turn_id": payload.get("turn_id"),
    "hook_event_name": payload.get("hook_event_name"),
    "tool_use_id": payload.get("tool_use_id"),
    "notification_type": payload.get("notification_type"),
    "source": payload.get("source"),
}
event_id = "evt_" + hashlib.sha256(
    json.dumps(identity, sort_keys=True, separators=(",", ":"), default=str).encode()
).hexdigest()[:32]

command = [
    orc_bin,
    "agent-event",
    "--engine", engine,
    "--agent-id", os.environ["ORC_AGENT_ID"],
    "--instance", os.environ["ORC_AGENT_INSTANCE"],
    "--state", lifecycle,
    "--event-id", event_id,
]
if session_id:
    command.extend(["--provider-session", session_id])

try:
    subprocess.run(
        command,
        stdin=subprocess.DEVNULL,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        timeout=command_timeout,
        check=False,
    )
except Exception:
    pass
PY

exit 0
