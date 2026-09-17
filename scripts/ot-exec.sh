#!/usr/bin/env bash
# OpenTunnel ForceCommand wrapper.
#
# tailcat runs this once per SSH session on the host, as the host user, with
# the client's requested command in $SSH_ORIGINAL_COMMAND. It refuses
# interactive shells, records session activity for the supervisor, writes the
# audit line, shows the command in the host terminal, and runs the command in
# the session working directory.
#
# Inputs: $SSH_ORIGINAL_COMMAND, $TAILCAT_PEER_KEY, $TAILCAT_REMOTE_ADDR.
# State:  the env file written next to this script by host.sh.

set -uo pipefail

ot_dir=$(unset CDPATH && cd -- "$(dirname -- "$0")" && pwd)
# shellcheck disable=SC1091
. "$ot_dir/env"

: "${OPENTUNNEL_WORK:=$ot_dir}"
: "${OPENTUNNEL_CWD:=$HOME}"
: "${OPENTUNNEL_SUPERVISOR_PID:=}"
: "${OPENTUNNEL_TTY:=}"

ot_now() {
	date -u +%Y-%m-%dT%H:%M:%SZ
}

# Writes one line to the terminal that opened the tunnel, if there is one.
# Written by this process directly, before the command runs, so the line is
# on screen whatever the command does to the audit log afterwards.
ot_show() {
	[ -n "$OPENTUNNEL_TTY" ] && [ -w "$OPENTUNNEL_TTY" ] || return 0
	printf '[opentunnel] %s\n' "$*" >>"$OPENTUNNEL_TTY" 2>/dev/null || true
}

# Replaces C0 control characters and DEL, so a command line can never carry
# terminal escape sequences into the host terminal or the audit log.
ot_printable() {
	printf '%s' "$1" | tr '\000-\037\177' '?'
}

# The supervisor enforces the session lifetime and removes the keys on exit.
# Without it the tunnel would run unbounded, so a session that finds it gone
# ends the tunnel instead of serving the command.
ot_supervisor_alive() {
	local args
	[ -n "$OPENTUNNEL_SUPERVISOR_PID" ] || return 0
	kill -0 "$OPENTUNNEL_SUPERVISOR_PID" 2>/dev/null || return 1
	args=$(ps -p "$OPENTUNNEL_SUPERVISOR_PID" -o args= 2>/dev/null || true)
	case "$args" in
	"" | *host.sh* | *self.sh*) return 0 ;;
	*) return 1 ;;
	esac
}

ot_end_orphaned_tunnel() {
	local parent_args
	ot_show "the session supervisor is gone; ending the tunnel"
	echo "opentunnel: the session has ended" >&2
	parent_args=$(ps -p "$PPID" -o args= 2>/dev/null || true)
	case "$parent_args" in
	*tailcat*serve*) kill -TERM "$PPID" 2>/dev/null || true ;;
	esac
	cd / && rm -rf "$OPENTUNNEL_WORK" 2>/dev/null
	exit 2
}

if ! ot_supervisor_alive; then
	ot_end_orphaned_tunnel
fi

command_line=${SSH_ORIGINAL_COMMAND:-}
if [ -z "$command_line" ]; then
	echo "opentunnel: interactive shells are disabled; pass a command" >&2
	exit 2
fi

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
audit_command=$(ot_printable "$audit_command")
# The session pid is the correlation key between the two record kinds, which
# is what lets the host print overlapping commands without confusing them.
printf '%s\t%s\t%s\t%s\t%s\n' \
	"$(ot_now)" \
	"$$" \
	"${TAILCAT_PEER_KEY:-unknown}" \
	"${TAILCAT_REMOTE_ADDR:-unknown}" \
	"$audit_command" >>"$OPENTUNNEL_WORK/audit.log" 2>/dev/null || true

shown_command=$audit_command
if [ "${#shown_command}" -gt 200 ]; then
	shown_command="${shown_command:0:200}…"
fi
ot_show "$$ \$ $shown_command"

cd "$OPENTUNNEL_CWD" 2>/dev/null || cd "$HOME" || exit 1

# Not exec: the EXIT trap has to run so the supervisor sees the session end.
bash -lc "$command_line"
rc=$?

printf '%s\t%s\texit=%s\n' "$(ot_now)" "$$" "$rc" >>"$OPENTUNNEL_WORK/audit.log" 2>/dev/null || true
ot_show "$$ exit=$rc"
exit "$rc"
