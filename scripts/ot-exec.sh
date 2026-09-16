#!/usr/bin/env bash
# OpenTunnel ForceCommand wrapper.
#
# tailcat runs this once per SSH session on the host, as the host user, with
# the client's requested command in $SSH_ORIGINAL_COMMAND. It refuses
# interactive shells, records session activity for the supervisor, writes the
# audit line, and runs the command in the session working directory.
#
# Inputs: $SSH_ORIGINAL_COMMAND, $TAILCAT_PEER_KEY, $TAILCAT_REMOTE_ADDR.
# State:  $OPENTUNNEL_WORK from the env file written next to this script.

set -uo pipefail

ot_dir=$(unset CDPATH && cd -- "$(dirname -- "$0")" && pwd)
# shellcheck disable=SC1091
. "$ot_dir/env"

: "${OPENTUNNEL_WORK:=$ot_dir}"
: "${OPENTUNNEL_CWD:=$HOME}"

command_line=${SSH_ORIGINAL_COMMAND:-}
if [ -z "$command_line" ]; then
	echo "opentunnel: interactive shells are disabled; pass a command" >&2
	exit 2
fi

ot_now() {
	date -u +%Y-%m-%dT%H:%M:%SZ
}

# The supervisor reads activity as epoch seconds; write it atomically so a
# concurrent read never sees a half-written file.
ot_touch_activity() {
	date +%s >"$OPENTUNNEL_WORK/activity.$$" 2>/dev/null || return 0
	mv -f "$OPENTUNNEL_WORK/activity.$$" "$OPENTUNNEL_WORK/activity" 2>/dev/null || return 0
}

mkdir -p "$OPENTUNNEL_WORK/sessions" 2>/dev/null || true
: >"$OPENTUNNEL_WORK/sessions/$$" 2>/dev/null || true
ot_touch_activity

trap 'rm -f "$OPENTUNNEL_WORK/sessions/$$" 2>/dev/null; ot_touch_activity' EXIT

# Audit the command line only, never the payload piped through stdin.
audit_command=$command_line
audit_command=${audit_command//\\/\\\\}
audit_command=${audit_command//$'\n'/\\n}
audit_command=${audit_command//$'\r'/\\r}
audit_command=${audit_command//$'\t'/\\t}
printf '%s\t%s\t%s\t%s\n' \
	"$(ot_now)" \
	"${TAILCAT_PEER_KEY:-unknown}" \
	"${TAILCAT_REMOTE_ADDR:-unknown}" \
	"$audit_command" >>"$OPENTUNNEL_WORK/audit.log" 2>/dev/null || true

cd "$OPENTUNNEL_CWD" 2>/dev/null || cd "$HOME" || exit 1

# Not exec: the EXIT trap has to run so the supervisor sees the session end.
bash -lc "$command_line"
rc=$?

printf '%s\texit=%s\n' "$(ot_now)" "$rc" >>"$OPENTUNNEL_WORK/audit.log" 2>/dev/null || true
exit "$rc"
