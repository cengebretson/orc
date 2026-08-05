#!/usr/bin/env bash
# installed by orc
# managed by orc; reinstalling agent hooks overwrites this file.
# ORC_AGENT_HOOK_VERSION=2

set +e

engine="${1:-}"
lifecycle="${2:-}"
orc_bin="${3:-orc}"

[ -n "${ORC_AGENT_ID:-}" ] || exit 0
[ -n "${ORC_AGENT_INSTANCE:-}" ] || exit 0
[ -n "${TMUX_PANE:-}" ] || exit 0
[ -n "$engine" ] || exit 0
[ -n "$lifecycle" ] || exit 0

"$orc_bin" agent-event --hook-input --engine "$engine" --state "$lifecycle" >/dev/null 2>&1 || true

exit 0
