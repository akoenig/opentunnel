#!/usr/bin/env bash
# shellcheck disable=SC2015,SC2012
# End-to-end test: a real host session and a real agent install on this
# machine, over the public DERP relays. Needs internet access.
#
#   build/build-tailcat.sh && build/embed.sh && test/e2e.sh
#
# The scripts are served from dist/ by a local HTTP server, so this exercises
# exactly what beta.opentunnel.sh serves, including the POSIX shim.

set -uo pipefail

# The "check && ok || fail" idiom is safe here: ok and fail always succeed.
repo_root=$(unset CDPATH && cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root" || exit 1

PORT=${OPENTUNNEL_TEST_PORT:-18080}
BASE_URL="http://127.0.0.1:$PORT"
TMP=$(mktemp -d "${TMPDIR:-/tmp}/opentunnel-e2e.XXXXXX")
SERVER_PID=""
HOST_PID=""
HELPER=""
REMOTE_CWD=""
FAILURES=0
CASES=0

export OPENTUNNEL_BASE_URL="$BASE_URL"
export OPENTUNNEL_ALLOW_HTTP=1

say() { printf '\n=== %s\n' "$*"; }
info() { printf '    %s\n' "$*"; }

ok() {
	CASES=$((CASES + 1))
	printf 'ok   %s\n' "$*"
}

fail() {
	CASES=$((CASES + 1))
	FAILURES=$((FAILURES + 1))
	printf 'FAIL %s\n' "$*"
}

check() {
	# check <description> <expected> <actual>
	if [ "$2" = "$3" ]; then
		ok "$1"
	else
		fail "$1 (expected [$2], got [$3])"
	fi
}

# Invoked by the trap below (SC2317 in shellcheck 0.9, SC2329 in 0.11).
# shellcheck disable=SC2317,SC2329
cleanup() {
	[ -n "$HOST_PID" ] && kill "$HOST_PID" 2>/dev/null
	[ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null
	if [ -n "$HELPER" ] && [ -x "$HELPER" ]; then
		"$HELPER" --close >/dev/null 2>&1
	fi
	rm -rf "$TMP" "${REMOTE_CWD:-}"
}
trap cleanup EXIT INT TERM

for f in dist/host.sh dist/agent.sh; do
	[ -f "$f" ] || {
		echo "missing $f; run build/build-tailcat.sh and build/embed.sh first" >&2
		exit 1
	}
done

say "serving dist/ on $BASE_URL"
python3 -m http.server "$PORT" --bind 127.0.0.1 --directory dist >"$TMP/http.log" 2>&1 &
SERVER_PID=$!
for _ in 1 2 3 4 5 6 7 8 9 10; do
	curl -fsS -o /dev/null "$BASE_URL/host.sh" && break
	sleep 1
done
curl -fsS -o /dev/null "$BASE_URL/host.sh" || {
	echo "local HTTP server did not start" >&2
	exit 1
}

# start_host <name> [env assignments...]
start_host() {
	local name=$1
	shift
	HOST_OUT="$TMP/$name.out"
	HOST_ERR="$TMP/$name.err"
	: >"$HOST_OUT"
	: >"$HOST_ERR"
	env "$@" sh "$repo_root/dist/host.sh" >"$HOST_OUT" 2>"$HOST_ERR" &
	HOST_PID=$!
}

wait_for_addr() {
	local waited=0
	ADDR=""
	while [ "$waited" -lt 60 ]; do
		ADDR=$(grep -o 'tc[A-Za-z0-9_-]\{40,\}' "$HOST_OUT" 2>/dev/null | head -n 1)
		[ -n "$ADDR" ] && return 0
		kill -0 "$HOST_PID" 2>/dev/null || return 1
		sleep 1
		waited=$((waited + 1))
	done
	return 1
}

# wait_for_log <basic regex> [seconds]
wait_for_log() {
	local pattern=$1 limit=${2:-15} waited=0
	while [ "$waited" -lt "$limit" ]; do
		grep -q "$pattern" "$HOST_ERR" 2>/dev/null && return 0
		sleep 1
		waited=$((waited + 1))
	done
	return 1
}

install_agent() {
	local rc
	sh "$repo_root/dist/agent.sh" "$ADDR" >"$TMP/agent.out" 2>"$TMP/agent.err"
	rc=$?
	HELPER=$(sed -n 's/^remote helper: //p' "$TMP/agent.out" | head -n 1)
	[ "$rc" -eq 0 ] && [ -x "$HELPER" ]
}

######################################################################
say "session 1: command execution, transfer, concurrency"
REMOTE_CWD=$(mktemp -d "${TMPDIR:-/tmp}/opentunnel-e2e-cwd.XXXXXX")
start_host s1 OPENTUNNEL_IDLE=120 OPENTUNNEL_CWD="$REMOTE_CWD" OPENTUNNEL_KEEP_AUDIT=1
if ! wait_for_addr; then
	fail "host printed a tunnel address"
	cat "$HOST_ERR" >&2
	exit 1
fi
ok "host printed a tunnel address"
grep -q "working directory $REMOTE_CWD" "$HOST_OUT" && ok "prompt names the working directory" || fail "prompt names the working directory"
grep -q '2 minutes without a command' "$HOST_OUT" && ok "prompt renders the idle timeout" || fail "prompt renders the idle timeout"
grep -q 'in total' "$HOST_OUT" && fail "prompt omits the TTL clause when TTL is off" || ok "prompt omits the TTL clause when TTL is off"

if ! install_agent; then
	fail "agent installed and connected"
	cat "$TMP/agent.err" >&2
	exit 1
fi
ok "agent installed and connected"
info "helper: $HELPER"
AGENT_WORK=$(dirname "$HELPER")

out=$("$HELPER" 'echo hi; pwd' 2>"$TMP/c1.err")
check "remote command output" "hi
$REMOTE_CWD" "$out"

"$HELPER" 'exit 7' >/dev/null 2>&1
check "remote exit code passthrough" "7" "$?"

wait_for_log '\$ echo hi; pwd' && ok "host terminal shows the command" || fail "host terminal shows the command"
wait_for_log 'exit=7' && ok "host terminal shows the exit code" || fail "host terminal shows the exit code"

"$HELPER" >/dev/null 2>"$TMP/usage.err"
check "helper without arguments exits 2" "2" "$?"

"$AGENT_WORK/ssh" opentunnel </dev/null >/dev/null 2>"$TMP/interactive.err"
check "interactive session refused" "2" "$?"
grep -q 'interactive shells are disabled' "$TMP/interactive.err" &&
	ok "interactive refusal explains itself" || fail "interactive refusal explains itself"

say "file transfer"
head -c 65536 /dev/urandom >"$TMP/payload.bin"
"$HELPER" --put "$TMP/payload.bin" "$REMOTE_CWD/payload.bin" >/dev/null 2>"$TMP/put.err"
check "--put exit code" "0" "$?"
"$HELPER" --get "$REMOTE_CWD/payload.bin" "$TMP/payload.back" >/dev/null 2>"$TMP/get.err"
check "--get exit code" "0" "$?"
cmp -s "$TMP/payload.bin" "$TMP/payload.back" && ok "round-tripped file is identical" || fail "round-tripped file is identical"

"$HELPER" "cat > $REMOTE_CWD/raw.bin" <"$TMP/payload.bin" >/dev/null 2>&1
"$HELPER" "cat $REMOTE_CWD/raw.bin" >"$TMP/raw.back" 2>/dev/null
cmp -s "$TMP/payload.bin" "$TMP/raw.back" && ok "raw cat over stdin and stdout" || fail "raw cat over stdin and stdout"

if command -v rsync >/dev/null 2>&1; then
	mkdir -p "$TMP/tree/sub"
	echo one >"$TMP/tree/one.txt"
	echo two >"$TMP/tree/sub/two.txt"
	rsync -a -e "$AGENT_WORK/ssh" "$TMP/tree/" "opentunnel:$REMOTE_CWD/tree/" >"$TMP/rsync.log" 2>&1
	check "rsync round trip" "0" "$?"
	out=$("$HELPER" "cat $REMOTE_CWD/tree/sub/two.txt" 2>/dev/null)
	check "rsync transferred the tree" "two" "$out"
else
	info "rsync not installed, skipping"
fi

say "concurrency"
start=$(date +%s)
"$HELPER" 'sleep 4; echo A' >"$TMP/par.a" 2>"$TMP/par.a.err" &
pa=$!
"$HELPER" 'sleep 4; echo B' >"$TMP/par.b" 2>"$TMP/par.b.err" &
pb=$!
sleep 2
sessions=$(ls "$AGENT_WORK" >/dev/null 2>&1 && "$HELPER" 'true' >/dev/null 2>&1 && echo counted)
wait "$pa"
ra=$?
wait "$pb"
rb=$?
elapsed=$(($(date +%s) - start))
check "first parallel command exit code" "0" "$ra"
check "second parallel command exit code" "0" "$rb"
check "parallel output A" "A" "$(cat "$TMP/par.a")"
check "parallel output B" "B" "$(cat "$TMP/par.b")"
if [ "$elapsed" -lt 8 ]; then
	ok "parallel commands overlapped (${elapsed}s)"
else
	fail "parallel commands overlapped (took ${elapsed}s)"
fi
[ -n "$sessions" ] && info "a third command ran while both were in flight"

say "unauthorized peer"
timeout 25 "$AGENT_WORK/tailcat" --key=new ssh "$ADDR" true >/dev/null 2>&1
rc=$?
if [ "$rc" -ne 0 ]; then
	ok "a foreign client key cannot connect (rc=$rc)"
else
	fail "a foreign client key cannot connect"
fi

say "helper --close"
"$HELPER" --close >/dev/null 2>&1
if [ -d "$AGENT_WORK" ]; then
	fail "--close removed the agent directory"
else
	ok "--close removed the agent directory"
fi
HELPER=""

say "host shutdown on signal keeps the audit log"
HOST_WORK=$(grep -o '/tmp/opentunnel-host\.[A-Za-z0-9]*' "$HOST_ERR" 2>/dev/null | head -n 1)
kill -TERM "$HOST_PID" 2>/dev/null
wait "$HOST_PID" 2>/dev/null
HOST_PID=""
if [ -n "$HOST_WORK" ] && [ -d "$HOST_WORK" ]; then
	fail "host removed its temp directory"
else
	ok "host removed its temp directory"
fi
kept=$(ls opentunnel-audit-*.log 2>/dev/null | head -n 1)
if [ -n "$kept" ]; then
	ok "OPENTUNNEL_KEEP_AUDIT wrote $kept"
	grep -q 'echo hi' "$kept" && ok "audit log records command lines" || fail "audit log records command lines"
	rm -f "$kept"
else
	fail "OPENTUNNEL_KEEP_AUDIT wrote an audit log"
fi

######################################################################
say "session 2: idle timeout"
start_host s2 OPENTUNNEL_IDLE=20 OPENTUNNEL_CWD="$REMOTE_CWD"
if wait_for_addr && install_agent; then
	ok "second session came up"
	AGENT_WORK=$(dirname "$HELPER")
	"$HELPER" 'echo warm' >/dev/null 2>&1
	waited=0
	while kill -0 "$HOST_PID" 2>/dev/null && [ "$waited" -lt 60 ]; do
		sleep 2
		waited=$((waited + 2))
	done
	if kill -0 "$HOST_PID" 2>/dev/null; then
		fail "host ended on idle timeout"
		kill "$HOST_PID" 2>/dev/null
	else
		ok "host ended on idle timeout after ${waited}s"
		grep -q 'session ended (idle)' "$HOST_ERR" && ok "host logged the idle reason" || fail "host logged the idle reason"
	fi
	timeout 90 "$HELPER" 'echo should-fail' >/dev/null 2>&1
	rc=$?
	[ "$rc" -ne 0 ] && ok "commands fail once the session ended (rc=$rc)" || fail "commands fail once the session ended"
	"$HELPER" --close >/dev/null 2>&1
	HELPER=""
else
	fail "second session came up"
fi
HOST_PID=""

######################################################################
say "session 3: hard limit (TTL) while a command runs"
start_host s3 OPENTUNNEL_IDLE=600 OPENTUNNEL_TTL=30 OPENTUNNEL_CWD="$REMOTE_CWD"
if wait_for_addr; then
	grep -q '30 seconds\|1 minutes in total' "$HOST_OUT" && ok "prompt renders the TTL clause" || fail "prompt renders the TTL clause"
	if install_agent; then
		AGENT_WORK=$(dirname "$HELPER")
		"$HELPER" 'sleep 120' >/dev/null 2>&1 &
		long=$!
		waited=0
		while kill -0 "$HOST_PID" 2>/dev/null && [ "$waited" -lt 90 ]; do
			sleep 2
			waited=$((waited + 2))
		done
		if kill -0 "$HOST_PID" 2>/dev/null; then
			fail "host ended on the hard limit"
			kill "$HOST_PID" 2>/dev/null
		else
			ok "host ended on the hard limit after ${waited}s"
			grep -q 'session ended (ttl)' "$HOST_ERR" && ok "host logged the ttl reason" || fail "host logged the ttl reason"
		fi
		kill "$long" 2>/dev/null
		"$HELPER" --close >/dev/null 2>&1
		HELPER=""
	else
		fail "third session came up"
	fi
else
	fail "third session came up"
fi
HOST_PID=""

######################################################################
say "session 4: claim timeout with no agent"
start_host s4 OPENTUNNEL_CLAIM_TIMEOUT=5
if wait_for_addr; then
	waited=0
	while kill -0 "$HOST_PID" 2>/dev/null && [ "$waited" -lt 30 ]; do
		sleep 1
		waited=$((waited + 1))
	done
	if kill -0 "$HOST_PID" 2>/dev/null; then
		fail "host gave up when nobody claimed the tunnel"
		kill "$HOST_PID" 2>/dev/null
	else
		ok "host gave up when nobody claimed the tunnel after ${waited}s"
		grep -q 'no agent claimed the tunnel' "$HOST_ERR" && ok "host logged the claim timeout" || fail "host logged the claim timeout"
	fi
else
	fail "fourth session came up"
fi
HOST_PID=""

######################################################################
say "session 5: a malformed claim ends the session"
start_host s5 OPENTUNNEL_CLAIM_TIMEOUT=120
if wait_for_addr; then
	printf 'garbage\n' | "$repo_root/dist/bin/$(tr -d '[:space:]' <VERSION)/tailcat_linux_amd64" --key=new "$ADDR" >/dev/null 2>&1
	waited=0
	while kill -0 "$HOST_PID" 2>/dev/null && [ "$waited" -lt 30 ]; do
		sleep 1
		waited=$((waited + 1))
	done
	if kill -0 "$HOST_PID" 2>/dev/null; then
		fail "host refused a malformed claim"
		kill "$HOST_PID" 2>/dev/null
	else
		ok "host refused a malformed claim after ${waited}s"
		grep -q 'claim rejected' "$HOST_ERR" && ok "host logged the rejected claim" || fail "host logged the rejected claim"
	fi
else
	fail "fifth session came up"
fi
HOST_PID=""

######################################################################
printf '\n%s\n' "----------------------------------------------------------------"
if [ "$FAILURES" -eq 0 ]; then
	printf 'all %s checks passed\n' "$CASES"
	exit 0
fi
printf '%s of %s checks failed\n' "$FAILURES" "$CASES"
exit 1
