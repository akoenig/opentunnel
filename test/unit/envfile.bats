#!/usr/bin/env bats
# The env file host.sh writes for the ForceCommand wrapper.

setup() {
	REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	WORK="$BATS_TEST_TMPDIR/work"
	mkdir -p "$WORK"
}

write_env() {
	{
		printf 'OPENTUNNEL_WORK=%q\n' "$1"
		printf 'OPENTUNNEL_CWD=%q\n' "$2"
	} >"$WORK/env"
}

@test "a plain path round-trips" {
	write_env "$WORK" "/srv/project"
	# shellcheck disable=SC1091
	source "$WORK/env"
	[ "$OPENTUNNEL_CWD" = "/srv/project" ]
	[ "$OPENTUNNEL_WORK" = "$WORK" ]
}

@test "a path with spaces round-trips" {
	write_env "$WORK" "/srv/my project/sub dir"
	# shellcheck disable=SC1091
	source "$WORK/env"
	[ "$OPENTUNNEL_CWD" = "/srv/my project/sub dir" ]
}

@test "a path with quotes and a dollar sign round-trips" {
	write_env "$WORK" "/srv/it's \"here\" \$HOME"
	# shellcheck disable=SC1091
	source "$WORK/env"
	[ "$OPENTUNNEL_CWD" = "/srv/it's \"here\" \$HOME" ]
}

@test "a path with a newline round-trips" {
	write_env "$WORK" "$(printf '/srv/two\nlines')"
	# shellcheck disable=SC1091
	source "$WORK/env"
	[ "$OPENTUNNEL_CWD" = "$(printf '/srv/two\nlines')" ]
}

@test "the wrapper refuses an interactive session" {
	printf 'OPENTUNNEL_WORK=%q\nOPENTUNNEL_CWD=%q\n' "$WORK" "$WORK" >"$WORK/env"
	cp "$REPO/scripts/ot-exec.sh" "$WORK/ot-exec.sh"
	chmod 700 "$WORK/ot-exec.sh"
	run env -u SSH_ORIGINAL_COMMAND "$WORK/ot-exec.sh"
	[ "$status" -eq 2 ]
	[[ "$output" == *"interactive shells are disabled"* ]]
}

@test "the wrapper runs a command, records it, and passes the exit code through" {
	printf 'OPENTUNNEL_WORK=%q\nOPENTUNNEL_CWD=%q\n' "$WORK" "$WORK" >"$WORK/env"
	cp "$REPO/scripts/ot-exec.sh" "$WORK/ot-exec.sh"
	chmod 700 "$WORK/ot-exec.sh"
	run env SSH_ORIGINAL_COMMAND='echo hello; exit 3' TAILCAT_PEER_KEY=nodekey:abc TAILCAT_REMOTE_ADDR=[::1]:1 "$WORK/ot-exec.sh"
	[ "$status" -eq 3 ]
	[ "$output" = "hello" ]
	grep -q 'nodekey:abc' "$WORK/audit.log"
	grep -q 'echo hello; exit 3' "$WORK/audit.log"
	grep -q 'exit=3' "$WORK/audit.log"
	[ -s "$WORK/activity" ]
	[ -z "$(ls -A "$WORK/sessions")" ]
}

@test "the wrapper escapes newlines in the audit log" {
	printf 'OPENTUNNEL_WORK=%q\nOPENTUNNEL_CWD=%q\n' "$WORK" "$WORK" >"$WORK/env"
	cp "$REPO/scripts/ot-exec.sh" "$WORK/ot-exec.sh"
	chmod 700 "$WORK/ot-exec.sh"
	run env SSH_ORIGINAL_COMMAND="$(printf 'echo one\necho two')" "$WORK/ot-exec.sh"
	[ "$status" -eq 0 ]
	[ "$(wc -l <"$WORK/audit.log")" -eq 2 ]
	grep -q 'echo one\\necho two' "$WORK/audit.log"
}
