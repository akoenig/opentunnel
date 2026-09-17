#!/usr/bin/env bats
# ot-exec.sh: what a hostile command cannot do to the host terminal, and what
# happens to a session whose supervisor is gone.

setup() {
	REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	WORK="$BATS_TEST_TMPDIR/work"
	mkdir -p "$WORK"
	cp "$REPO/scripts/ot-exec.sh" "$WORK/ot-exec.sh"
	chmod 700 "$WORK/ot-exec.sh"
	TTY="$BATS_TEST_TMPDIR/terminal"
	: >"$TTY"
	# A stand-in supervisor whose command line looks like the real one. Its
	# output is detached so bats does not wait for it to finish.
	printf 'sleep 300\n' >"$WORK/self.sh"
	bash "$WORK/self.sh" >/dev/null 2>&1 </dev/null &
	SUPERVISOR=$!
	IMPOSTOR=""
}

teardown() {
	pkill -P "$SUPERVISOR" 2>/dev/null || true
	kill "$SUPERVISOR" 2>/dev/null || true
	[ -n "$IMPOSTOR" ] && kill "$IMPOSTOR" 2>/dev/null || true
}

write_env() {
	# write_env <supervisor pid> <tty path>
	{
		printf 'OPENTUNNEL_WORK=%q\n' "$WORK"
		printf 'OPENTUNNEL_CWD=%q\n' "$WORK"
		printf 'OPENTUNNEL_SUPERVISOR_PID=%q\n' "$1"
		printf 'OPENTUNNEL_TTY=%q\n' "$2"
	} >"$WORK/env"
}

@test "the command is on the terminal before it runs" {
	write_env "$SUPERVISOR" "$TTY"
	run env SSH_ORIGINAL_COMMAND="cat '$TTY'" "$WORK/ot-exec.sh"
	[ "$status" -eq 0 ]
	# The command's own output is the terminal as it was when the command ran.
	[[ "$output" == *"\$ cat '$TTY'"* ]]
	[[ "$(cat "$TTY")" == *"exit=0"* ]]
}

@test "terminal escape sequences in the command line are neutralized" {
	write_env "$SUPERVISOR" "$TTY"
	# Quoted, so the ';' inside the OSC sequence stays part of one word.
	run env SSH_ORIGINAL_COMMAND="$(printf "echo 'hi\033[2K\033]0;pwned\007'")" "$WORK/ot-exec.sh"
	[ "$status" -eq 0 ]
	! grep -q $'\033' "$TTY"
	! grep -q $'\033' "$WORK/audit.log"
	grep -q "echo 'hi?\[2K?\]0;pwned?'" "$TTY"
	grep -q "echo 'hi?\[2K?\]0;pwned?'" "$WORK/audit.log"
}

@test "a session without a terminal still records and runs" {
	write_env "$SUPERVISOR" ""
	run env SSH_ORIGINAL_COMMAND='echo plain' "$WORK/ot-exec.sh"
	[ "$status" -eq 0 ]
	[ "$output" = "plain" ]
	grep -q 'echo plain' "$WORK/audit.log"
}

@test "a session whose supervisor is gone refuses the command and ends the tunnel state" {
	sleep 1 >/dev/null 2>&1 </dev/null &
	dead=$!
	wait "$dead"
	write_env "$dead" "$TTY"
	run env SSH_ORIGINAL_COMMAND='echo should-not-run' "$WORK/ot-exec.sh"
	[ "$status" -eq 2 ]
	[[ "$output" != *"should-not-run"* ]]
	[[ "$output" == *"the session has ended"* ]]
	[[ "$(cat "$TTY")" == *"supervisor is gone"* ]]
	[ ! -d "$WORK" ]
}

@test "a reused pid that is not the supervisor counts as gone" {
	sleep 300 >/dev/null 2>&1 </dev/null &
	IMPOSTOR=$!
	write_env "$IMPOSTOR" ""
	run env SSH_ORIGINAL_COMMAND='echo should-not-run' "$WORK/ot-exec.sh"
	[ "$status" -eq 2 ]
	[[ "$output" != *"should-not-run"* ]]
}
