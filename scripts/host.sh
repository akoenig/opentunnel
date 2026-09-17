#!/usr/bin/env bash
# OpenTunnel host script. Served at https://beta.opentunnel.sh
#
#   curl -fsSL https://beta.opentunnel.sh | sh
#
# Opens a temporary command tunnel from this machine to one coding agent on
# another machine, prints the prompt to paste into that agent, and ends on
# Ctrl-C, on inactivity, or when the optional hard limit is reached.
#
# build/embed.sh replaces the "# @@...@@" marker lines below at release time.

set -euo pipefail

VERSION="${VERSION:-dev}"
# @@VERSION@@

BASE_URL="${OPENTUNNEL_BASE_URL:-https://beta.opentunnel.sh}"
# @@BASE_URL@@

# Pinned per-target sha256 of the tailcat binary. Empty in a development
# checkout, filled in by build/embed.sh for released scripts.
ot_expected_sha256() {
	case "$1" in
	*) printf '' ;;
	esac
}
# @@CHECKSUMS@@

# Writes the ForceCommand wrapper to $WORK/ot-exec.sh. In a development
# checkout the sibling script is used; build/embed.sh replaces this with an
# inline copy so the served script is self-contained.
ot_write_exec_wrapper() {
	local src
	src="$(dirname -- "$0")/ot-exec.sh"
	if [ ! -f "$src" ]; then
		ot_die "ot-exec.sh not found next to $0 (development checkout only)"
	fi
	cat "$src" >"$WORK/ot-exec.sh"
}
# @@OT_EXEC@@

WORK=""
SERVER_PID=""
CLAIM_PID=""
ENDED_REASON=""
SHOW_COMMANDS=0
AUDIT_SHOWN=0
HOST_TTY=""

ot_log() {
	printf '[opentunnel] %s\n' "$*" >&2
}

ot_die() {
	printf '[opentunnel] error: %s\n' "$*" >&2
	exit 1
}

ot_is_uint() {
	case "$1" in
	'' | *[!0-9]*) return 1 ;;
	*) return 0 ;;
	esac
}

# ot_uint_env NAME DEFAULT
ot_uint_env() {
	local name=$1 default=$2 value
	eval "value=\${$name:-}"
	if [ -z "$value" ]; then
		printf '%s' "$default"
		return 0
	fi
	if ! ot_is_uint "$value"; then
		ot_die "$name must be a non-negative whole number of seconds, got: $value"
	fi
	printf '%s' "$value"
}

ot_minutes() {
	printf '%s' "$((($1 + 59) / 60))"
}

ot_require_tools() {
	local missing=0 tool
	for tool in curl mktemp uname date; do
		if ! command -v "$tool" >/dev/null 2>&1; then
			printf '[opentunnel] error: missing required tool: %s\n' "$tool" >&2
			missing=1
		fi
	done
	if ! command -v sha256sum >/dev/null 2>&1 && ! command -v shasum >/dev/null 2>&1; then
		printf '[opentunnel] error: missing required tool: sha256sum or shasum\n' >&2
		missing=1
	fi
	[ "$missing" -eq 0 ] || exit 1
}

ot_detect_platform() {
	case "$(uname -s)" in
	Linux) OS=linux ;;
	Darwin) OS=darwin ;;
	*) ot_die "unsupported OS: $(uname -s). OpenTunnel supports Linux and macOS." ;;
	esac
	case "$(uname -m)" in
	x86_64 | amd64) ARCH=amd64 ;;
	aarch64 | arm64) ARCH=arm64 ;;
	*) ot_die "unsupported architecture: $(uname -m). OpenTunnel supports amd64 and arm64." ;;
	esac
}

ot_sha256_file() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

ot_download_tailcat() {
	local url="$BASE_URL/bin/$VERSION/tailcat_${OS}_${ARCH}"
	local expected actual
	ot_log "downloading tailcat ($VERSION, ${OS}/${ARCH})"
	if [ -n "${OPENTUNNEL_ALLOW_HTTP:-}" ]; then
		curl -fsSL --retry 3 -o "$WORK/tailcat" "$url" ||
			ot_die "download failed: $url"
	else
		curl -fsSL --proto '=https' --tlsv1.2 --retry 3 -o "$WORK/tailcat" "$url" ||
			ot_die "download failed: $url"
	fi
	expected=$(ot_expected_sha256 "${OS}_${ARCH}")
	if [ -z "$expected" ]; then
		ot_log "warning: no pinned checksum in this script ($VERSION); skipping verification"
	else
		actual=$(ot_sha256_file "$WORK/tailcat")
		if [ "$actual" != "$expected" ]; then
			rm -f "$WORK/tailcat"
			ot_die "checksum mismatch for tailcat_${OS}_${ARCH} (expected $expected, got $actual)"
		fi
	fi
	chmod 700 "$WORK/tailcat"
	TC="$WORK/tailcat"
}

ot_is_addr() {
	case "$1" in
	tc*) ;;
	*) return 1 ;;
	esac
	# shellcheck disable=SC2254
	case "$1" in
	*[!A-Za-z0-9_-]*) return 1 ;;
	esac
	[ "${#1}" -ge 42 ]
}

ot_is_nodekey() {
	local hex=${1#nodekey:}
	[ "$hex" != "$1" ] || return 1
	[ "${#hex}" -eq 64 ] || return 1
	case "$hex" in
	*[!0-9a-f]*) return 1 ;;
	esac
	return 0
}

ot_sanitize() {
	printf '%s' "$1" | tr -c '[:print:]' '?' | cut -c1-40
}

ot_active_sessions() {
	local count=0 marker pid
	[ -d "$WORK/sessions" ] || {
		printf '0'
		return 0
	}
	for marker in "$WORK/sessions"/*; do
		[ -e "$marker" ] || continue
		pid=${marker##*/}
		if kill -0 "$pid" 2>/dev/null; then
			count=$((count + 1))
		else
			rm -f "$marker" 2>/dev/null || true
		fi
	done
	printf '%s' "$count"
}

ot_command_count() {
	[ -f "$WORK/audit.log" ] || {
		printf '0'
		return 0
	}
	# Command records have five fields; the exit records that follow have three.
	awk -F'\t' 'NF >= 5 {n++} END {printf "%d", n+0}' "$WORK/audit.log"
}

# Prints the audit records written since the last call. When the host runs in
# a terminal the wrapper writes there directly, before each command runs, and
# this is not used; it is the fallback for a host whose stderr is a file.
# A shrinking log is reported rather than silently skipped: a command that
# truncates the audit log must not be able to hide what follows it.
ot_show_new_audit() {
	local total pid third command
	[ "${SHOW_COMMANDS:-0}" -eq 1 ] || return 0
	[ -z "${HOST_TTY:-}" ] || return 0
	[ -n "$WORK" ] && [ -f "$WORK/audit.log" ] || return 0
	total=$(awk 'END {print NR}' "$WORK/audit.log" 2>/dev/null || true)
	ot_is_uint "${total:-}" || return 0
	if [ "$total" -lt "$AUDIT_SHOWN" ]; then
		ot_log "warning: the audit log shrank from $AUDIT_SHOWN to $total lines; a command truncated it"
		AUDIT_SHOWN=0
	fi
	[ "$total" -gt "$AUDIT_SHOWN" ] || return 0
	while IFS=$'\t' read -r _ pid third _ command; do
		case "$third" in
		exit=*)
			ot_log "$pid $third"
			;;
		*)
			if [ "${#command}" -gt 200 ]; then
				command="${command:0:200}…"
			fi
			ot_log "$pid \$ $command"
			;;
		esac
	# Neutralize control characters, keeping the tab separators and newlines.
	done < <(awk -v from="$((AUDIT_SHOWN + 1))" -v to="$total" 'NR >= from && NR <= to' "$WORK/audit.log" 2>/dev/null | tr '\000-\010\013-\037\177' '?')
	AUDIT_SHOWN=$total
}

# tailcat's log can echo strings the client influenced, so it never reaches
# the terminal raw.
ot_server_log_tail() {
	tail -n 3 "$WORK/server.log" 2>/dev/null | tr '\000-\010\013-\037\177' '?' || true
}

ot_last_activity() {
	local value=""
	[ -f "$WORK/activity" ] && value=$(cat "$WORK/activity" 2>/dev/null || true)
	if ot_is_uint "${value:-}"; then
		printf '%s' "$value"
	else
		printf '%s' "$START"
	fi
}

ot_stop_process() {
	local pid=$1
	[ -n "$pid" ] || return 0
	kill -0 "$pid" 2>/dev/null || return 0
	pkill -TERM -P "$pid" 2>/dev/null || true
	kill -TERM "$pid" 2>/dev/null || true
	for _ in 1 2 3 4 5; do
		kill -0 "$pid" 2>/dev/null || return 0
		sleep 1
	done
	pkill -KILL -P "$pid" 2>/dev/null || true
	kill -KILL "$pid" 2>/dev/null || true
	return 0
}

ot_keep_audit() {
	local stamp target
	[ "${OPENTUNNEL_KEEP_AUDIT:-}" = "1" ] || return 0
	[ -n "$WORK" ] && [ -f "$WORK/audit.log" ] || return 0
	stamp=$(date -u +%Y%m%dT%H%M%SZ)
	target="$LAUNCH_PWD/opentunnel-audit-$stamp.log"
	# The name is predictable to the second, so create the file rather than
	# writing to whatever is there: noclobber makes the open fail on an
	# existing file and on a symlink, which is what stops a shared working
	# directory from turning this into an overwrite of someone else's file.
	# The log holds command lines, so keep it to the owner.
	if (
		umask 077
		set -C
		cat "$WORK/audit.log" >"$target"
	) 2>/dev/null; then
		ot_log "audit log kept at $target"
	else
		ot_log "warning: could not write the audit log to $target"
	fi
}

ot_cleanup() {
	local rc=$?
	trap - EXIT INT TERM
	ot_stop_process "${CLAIM_PID:-}"
	ot_stop_process "${SERVER_PID:-}"
	ot_show_new_audit
	ot_keep_audit
	if [ -n "$WORK" ] && [ -d "$WORK" ]; then
		rm -rf "$WORK"
	fi
	exit "$rc"
}

ot_print_prompt() {
	local idle_min ttl_min ttl_clause transfer_hint
	idle_min=$(ot_minutes "$IDLE")
	ttl_clause=""
	if [ "$TTL" -gt 0 ]; then
		ttl_min=$(ot_minutes "$TTL")
		ttl_clause=" and after $ttl_min minutes in total"
	fi
	transfer_hint="    <path>/remote --put <local file> <remote path>
    <path>/remote --get <remote path> <local file>
    rsync -av -e <path>/ssh ./dir opentunnel:<remote path>
    scp -O -S <path>/ssh <local file> opentunnel:<remote path>"

	printf '%s\n' "────────────────────────────────────────────────────────────────────────"
	cat <<PROMPT
I opened an OpenTunnel session for you: temporary command access to a remote machine through an encrypted tunnel.

Setup (run once, on this machine):

    curl -fsSL $BASE_URL/agent | sh -s -- $ADDR

The installer prints the path of a \`remote\` helper, e.g. /tmp/opentunnel-agent.XXXXXX/remote.
Use it for every remote command:

    <path>/remote 'uname -a && pwd'
$transfer_hint

Remote host: $USER_NAME@$HOST_NAME ($OS_DESC), working directory $CWD.

Rules:
- Commands run non-interactively through SSH as that user with their login environment. No TTY, no interactive programs, no editors, no sudo prompts. Several commands may run at the same time.
- Always ask me to confirm before running anything destructive or irreversible.
- The session ends after $idle_min minutes without a command$ttl_clause. A connection error means it has ended: report that to me and stop; do not retry in a loop.
- Do not copy the address into shared logs, tickets, summaries, or long-lived notes. Do not persist the helper or its keys anywhere else. When finished, run \`<path>/remote --close\`.

Task:
PROMPT
	printf '%s\n' "────────────────────────────────────────────────────────────────────────"
}

ot_phase_one() {
	local waited=0 claim
	ot_log "waiting for the agent to claim (timeout $(ot_minutes "$CLAIM_TIMEOUT")m). Ctrl-C to abort."
	"$TC" --key="$WORK/session.private.json" serve >"$WORK/claim.txt" 2>"$WORK/phase1.log" &
	CLAIM_PID=$!
	while kill -0 "$CLAIM_PID" 2>/dev/null; do
		if [ "$waited" -ge "$CLAIM_TIMEOUT" ]; then
			ot_stop_process "$CLAIM_PID"
			CLAIM_PID=""
			ot_die "no agent claimed the tunnel within $(ot_minutes "$CLAIM_TIMEOUT") minutes"
		fi
		sleep 1
		waited=$((waited + 1))
	done
	wait "$CLAIM_PID" 2>/dev/null || true
	CLAIM_PID=""

	claim=$(tr -d ' \t\r\n' <"$WORK/claim.txt" 2>/dev/null || true)
	if ! ot_is_nodekey "$claim"; then
		ot_log "claim rejected: $(ot_sanitize "$claim")"
		ot_die "the tunnel was claimed with an invalid key. Run the command again for a fresh address."
	fi
	PEER=$claim
	ot_log "claimed by nodekey:$(printf '%s' "${claim#nodekey:}" | cut -c1-12)…"
}

ot_phase_two() {
	local waited=0
	"$TC" --key="$WORK/session.private.json" serve --allow="$PEER" no-auth-ssh -- "$WORK/ot-exec.sh" \
		>"$WORK/server.out" 2>"$WORK/server.log" &
	SERVER_PID=$!
	while :; do
		if grep -q 'Server listening' "$WORK/server.log" 2>/dev/null; then
			break
		fi
		if ! kill -0 "$SERVER_PID" 2>/dev/null; then
			ot_log "$(ot_server_log_tail)"
			ot_die "the tunnel server exited during startup"
		fi
		if [ "$waited" -ge 30 ]; then
			ot_die "the tunnel server did not start within 30 seconds"
		fi
		sleep 1
		waited=$((waited + 1))
	done
	if [ "$TTL" -gt 0 ]; then
		ot_log "active. idle timeout $(ot_minutes "$IDLE")m, hard limit $(ot_minutes "$TTL")m, cwd $CWD"
	else
		ot_log "active. idle timeout $(ot_minutes "$IDLE")m, cwd $CWD"
	fi
}

ot_end_session() {
	ENDED_REASON=$1
	ot_stop_process "$SERVER_PID"
	SERVER_PID=""
	ot_show_new_audit
	ot_log "session ended ($ENDED_REASON). audit log was $WORK/audit.log (removed with the temp dir)"
}

ot_supervise() {
	local now active last heartbeat_at expires_in
	heartbeat_at=$(date +%s)
	while :; do
		if ! kill -0 "$SERVER_PID" 2>/dev/null; then
			ot_log "$(ot_server_log_tail)"
			SERVER_PID=""
			ot_die "the tunnel server exited unexpectedly"
		fi
		ot_show_new_audit
		now=$(date +%s)
		active=$(ot_active_sessions)
		last=$(ot_last_activity)

		if [ "$TTL" -gt 0 ] && [ "$((now - START))" -ge "$TTL" ]; then
			ot_end_session ttl
			return 0
		fi
		if [ "$active" -eq 0 ] && [ "$((now - last))" -ge "$IDLE" ]; then
			ot_end_session idle
			return 0
		fi
		if [ "$((now - heartbeat_at))" -ge 60 ]; then
			heartbeat_at=$now
			if [ "$TTL" -gt 0 ]; then
				expires_in=$(ot_minutes "$((TTL - (now - START)))")
				ot_log "alive, sessions=$active, commands=$(ot_command_count), expires in ${expires_in}m"
			else
				ot_log "alive, sessions=$active, commands=$(ot_command_count)"
			fi
		fi
		sleep 2
	done
}

main() {
	# The temp directory and the trap come first: the POSIX shim may already
	# have created the directory, and every later failure has to remove it.
	if [ -n "${OPENTUNNEL_WORK_DIR:-}" ] && [ -d "${OPENTUNNEL_WORK_DIR:-}" ]; then
		WORK="$OPENTUNNEL_WORK_DIR"
	else
		WORK=$(mktemp -d "${TMPDIR:-/tmp}/opentunnel-host.XXXXXX")
	fi
	chmod 700 "$WORK"
	LAUNCH_PWD=$PWD
	trap ot_cleanup EXIT INT TERM

	ot_require_tools
	ot_detect_platform

	CLAIM_TIMEOUT=$(ot_uint_env OPENTUNNEL_CLAIM_TIMEOUT 300)
	IDLE=$(ot_uint_env OPENTUNNEL_IDLE 1800)
	TTL=$(ot_uint_env OPENTUNNEL_TTL 0)
	SHOW_COMMANDS=$(ot_uint_env OPENTUNNEL_SHOW_COMMANDS 1)
	[ "$SHOW_COMMANDS" -eq 0 ] || SHOW_COMMANDS=1
	CWD="${OPENTUNNEL_CWD:-$PWD}"
	[ -d "$CWD" ] || ot_die "OPENTUNNEL_CWD is not a directory: $CWD"

	ot_download_tailcat
	ot_write_exec_wrapper
	chmod 700 "$WORK/ot-exec.sh"
	# With a terminal on stderr the wrapper shows each command there itself,
	# before running it, so the line is on screen whatever the command does.
	HOST_TTY=""
	if [ "$SHOW_COMMANDS" -eq 1 ] && [ -t 2 ]; then
		HOST_TTY=$(tty <&2 2>/dev/null || true)
		[ -w "$HOST_TTY" ] || HOST_TTY=""
	fi
	{
		printf 'OPENTUNNEL_WORK=%q\n' "$WORK"
		printf 'OPENTUNNEL_CWD=%q\n' "$CWD"
		printf 'OPENTUNNEL_SUPERVISOR_PID=%q\n' "$$"
		printf 'OPENTUNNEL_TTY=%q\n' "$HOST_TTY"
	} >"$WORK/env"
	mkdir -p "$WORK/sessions"

	ADDR=$("$TC" genkey --key="$WORK/session.private.json" --embed-derp-map 2>"$WORK/genkey.log") ||
		ot_die "could not generate the session key: $(tail -n 2 "$WORK/genkey.log" 2>/dev/null || true)"
	ot_is_addr "$ADDR" || ot_die "unexpected tunnel address from tailcat"
	START=$(date +%s)
	date +%s >"$WORK/activity"

	USER_NAME="${USER:-$(id -un 2>/dev/null || echo unknown)}"
	HOST_NAME="$(hostname 2>/dev/null || uname -n)"
	OS_DESC="$(uname -srm)"

	ot_print_prompt
	ot_phase_one
	ot_phase_two
	ot_supervise
}

# test/unit sources this script to exercise the functions above.
if [ -z "${OPENTUNNEL_SOURCE_ONLY:-}" ]; then
	main "$@"
fi
