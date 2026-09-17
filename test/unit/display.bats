#!/usr/bin/env bats
# The supervisor's fallback display of the audit log (host without a terminal).

setup() {
	REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	export OPENTUNNEL_SOURCE_ONLY=1
	# shellcheck disable=SC1090
	source "$REPO/scripts/host.sh"
	WORK="$BATS_TEST_TMPDIR/work"
	mkdir -p "$WORK"
	SHOW_COMMANDS=1
	HOST_TTY=""
	AUDIT_SHOWN=0
	OUT="$BATS_TEST_TMPDIR/out"
}

record() {
	# record <pid> <command>
	printf '2026-09-17T10:00:00Z\t%s\tnodekey:abc\t[::1]:1\t%s\n' "$1" "$2" >>"$WORK/audit.log"
}

@test "new records are printed once" {
	record 11 'echo one'
	ot_show_new_audit 2>"$OUT"
	ot_show_new_audit 2>>"$OUT"
	[ "$(grep -c 'echo one' "$OUT")" -eq 1 ]
	[ "$AUDIT_SHOWN" -eq 1 ]
}

@test "control characters never reach the terminal" {
	printf '2026-09-17T10:00:00Z\t12\tnodekey:abc\t[::1]:1\techo hi\033[2K\033]0;pwned\007\n' >>"$WORK/audit.log"
	ot_show_new_audit 2>"$OUT"
	! grep -q $'\033' "$OUT"
	grep -q 'echo hi?\[2K?\]0;pwned?' "$OUT"
}

@test "a truncated log is reported and later records still show" {
	record 21 'echo before'
	record 22 'echo also-before'
	ot_show_new_audit 2>"$OUT"
	[ "$AUDIT_SHOWN" -eq 2 ]
	: >"$WORK/audit.log"
	record 23 'echo after'
	ot_show_new_audit 2>"$OUT"
	grep -q 'audit log shrank from 2 to 1' "$OUT"
	grep -q 'echo after' "$OUT"
	[ "$AUDIT_SHOWN" -eq 1 ]
}

@test "the fallback stays quiet when the wrapper writes to a terminal" {
	HOST_TTY="/dev/null"
	record 31 'echo direct'
	ot_show_new_audit 2>"$OUT"
	[ ! -s "$OUT" ]
}
