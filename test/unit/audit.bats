#!/usr/bin/env bats
# Keeping the audit log, and the one other place where host.sh prints text it
# did not write itself.

setup() {
	REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	export OPENTUNNEL_SOURCE_ONLY=1
	# shellcheck disable=SC1090
	source "$REPO/scripts/host.sh"
	WORK="$BATS_TEST_TMPDIR/work"
	LAUNCH_PWD="$BATS_TEST_TMPDIR/launch"
	mkdir -p "$WORK" "$LAUNCH_PWD"
	printf 'a-command-line\n' >"$WORK/audit.log"
	OPENTUNNEL_KEEP_AUDIT=1
	OUT="$BATS_TEST_TMPDIR/out"
}

kept_file() {
	find "$LAUNCH_PWD" -name 'opentunnel-audit-*.log' | head -n 1
}

@test "the audit log is copied out when asked" {
	ot_keep_audit 2>"$OUT"
	[ -n "$(kept_file)" ]
	grep -q 'a-command-line' "$(kept_file)"
	grep -q 'audit log kept at' "$OUT"
}

@test "the copy is readable only by its owner" {
	ot_keep_audit 2>"$OUT"
	run stat -c '%a' "$(kept_file)"
	[ "$output" = "600" ]
}

@test "nothing is copied unless asked" {
	OPENTUNNEL_KEEP_AUDIT=""
	ot_keep_audit 2>"$OUT"
	[ -z "$(kept_file)" ]
}

@test "a symlink at the target name is not written through" {
	local victim="$BATS_TEST_TMPDIR/victim"
	printf 'do-not-touch\n' >"$victim"
	# The name is predictable to the second, so plant both candidates.
	ln -s "$victim" "$LAUNCH_PWD/opentunnel-audit-$(date -u +%Y%m%dT%H%M%SZ).log"
	ln -s "$victim" "$LAUNCH_PWD/opentunnel-audit-$(date -u -d '+1 second' +%Y%m%dT%H%M%SZ).log" 2>/dev/null || true
	ot_keep_audit 2>"$OUT"
	[ "$(cat "$victim")" = "do-not-touch" ]
	grep -q 'could not write the audit log' "$OUT"
}

@test "an existing file at the target name is not overwritten" {
	printf 'existing\n' >"$LAUNCH_PWD/opentunnel-audit-$(date -u +%Y%m%dT%H%M%SZ).log"
	ot_keep_audit 2>"$OUT"
	grep -q 'existing' "$LAUNCH_PWD"/opentunnel-audit-*.log
}

@test "the tunnel server log is stripped of control characters" {
	printf 'server said \033[2Kthing\r\n' >"$WORK/server.log"
	run ot_server_log_tail
	[[ "$output" != *$'\033'* ]]
	[[ "$output" == *"server said ?[2Kthing"* ]]
}

@test "a missing tunnel server log is not an error" {
	run ot_server_log_tail
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}
