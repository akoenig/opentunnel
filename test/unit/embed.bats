#!/usr/bin/env bats
# build/embed.sh: marker replacement and the POSIX shim it wraps around.

setup() {
	REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	WORK="$BATS_TEST_TMPDIR/repo"
	mkdir -p "$WORK/scripts" "$WORK/build"
	cp "$REPO/scripts/host.sh" "$REPO/scripts/agent.sh" "$REPO/scripts/ot-exec.sh" "$WORK/scripts/"
	cp "$REPO/build/embed.sh" "$WORK/build/"
	printf 'dev\n' >"$WORK/VERSION"
	export OPENTUNNEL_SKIP_CHECKSUMS=1
}

build() {
	(cd "$WORK" && bash build/embed.sh 2>&1)
}

@test "embeds both scripts" {
	run build
	[ "$status" -eq 0 ]
	[ -x "$WORK/dist/host.sh" ]
	[ -x "$WORK/dist/agent.sh" ]
}

@test "no markers survive" {
	build
	! grep -q '@@[A-Z_]*@@' "$WORK/dist/host.sh"
	! grep -q '@@[A-Z_]*@@' "$WORK/dist/agent.sh"
}

@test "the host script carries the wrapper inline" {
	build
	grep -q 'interactive shells are disabled' "$WORK/dist/host.sh"
	grep -q 'ot_write_exec_wrapper' "$WORK/dist/host.sh"
}

@test "the version and base URL are baked in" {
	OPENTUNNEL_DEFAULT_BASE_URL=https://example.test build
	grep -q 'VERSION=dev' "$WORK/dist/host.sh"
	grep -q 'BASE_URL="${OPENTUNNEL_BASE_URL:-https://example.test}"' "$WORK/dist/agent.sh"
}

@test "checksums are pinned from SHA256SUMS" {
	mkdir -p "$WORK/dist/bin/dev"
	printf '%s  tailcat_linux_amd64\n%s  tailcat_darwin_arm64\n' \
		aaaabbbbccccddddeeeeffff0000111122223333444455556666777788889999 \
		9999888877776666555544443333222211110000ffffeeeeddddccccbbbbaaaa \
		>"$WORK/dist/bin/dev/SHA256SUMS"
	OPENTUNNEL_SKIP_CHECKSUMS="" run build
	[ "$status" -eq 0 ]
	grep -q 'linux_amd64) printf aaaabbbb' "$WORK/dist/host.sh"
	grep -q 'darwin_arm64) printf 9999' "$WORK/dist/agent.sh"
}

@test "a missing version marker fails the build" {
	grep -v '@@VERSION@@' "$REPO/scripts/host.sh" >"$WORK/scripts/host.sh"
	run build
	[ "$status" -ne 0 ]
	[[ "$output" == *"missing the @@VERSION@@ marker"* ]]
}

@test "a missing wrapper marker fails the build" {
	grep -v '@@OT_EXEC@@' "$REPO/scripts/host.sh" >"$WORK/scripts/host.sh"
	run build
	[ "$status" -ne 0 ]
	[[ "$output" == *"missing the @@OT_EXEC@@ marker"* ]]
}

@test "a missing checksum marker fails the build" {
	grep -v '@@CHECKSUMS@@' "$REPO/scripts/agent.sh" >"$WORK/scripts/agent.sh"
	run build
	[ "$status" -ne 0 ]
	[[ "$output" == *"missing the @@CHECKSUMS@@ marker"* ]]
}

@test "the built scripts parse" {
	build
	bash -n "$WORK/dist/host.sh"
	bash -n "$WORK/dist/agent.sh"
}

@test "the built scripts pass shellcheck" {
	if ! command -v shellcheck >/dev/null 2>&1; then
		skip "shellcheck is not installed"
	fi
	build
	shellcheck -s sh -e SC2034 "$WORK/dist/host.sh"
	shellcheck -s sh -e SC2034 "$WORK/dist/agent.sh"
}

@test "the shim reaches the bash body through sh with arguments intact" {
	build
	run sh -c "cat '$WORK/dist/agent.sh' | sh -s -- --help"
	[ "$status" -eq 0 ]
	[[ "$output" == *"<tunnel address>"* ]]
}

@test "the shim reaches the bash body through bash with arguments intact" {
	build
	run sh -c "cat '$WORK/dist/agent.sh' | bash -s -- --help"
	[ "$status" -eq 0 ]
	[[ "$output" == *"<tunnel address>"* ]]
}

@test "the shim passes an address through as the first argument" {
	build
	run sh -c "cat '$WORK/dist/agent.sh' | sh -s -- not-an-address"
	[ "$status" -eq 2 ]
	[[ "$output" == *"does not look like a tunnel address"* ]]
}

@test "an early exit leaves no temp directory behind" {
	build
	export TMPDIR="$BATS_TEST_TMPDIR/tmp"
	mkdir -p "$TMPDIR"
	run sh -c "cat '$WORK/dist/agent.sh' | sh -s -- --help"
	[ "$status" -eq 0 ]
	run sh -c "cat '$WORK/dist/agent.sh' | sh -s -- not-an-address"
	[ "$status" -eq 2 ]
	run sh -c "cat '$WORK/dist/host.sh' | env OPENTUNNEL_IDLE=nope sh"
	[ "$status" -ne 0 ]
	[ -z "$(ls -A "$TMPDIR")" ]
}

@test "the shim works when the script is run as a file" {
	build
	run sh "$WORK/dist/agent.sh" --help
	[ "$status" -eq 0 ]
	[[ "$output" == *"<tunnel address>"* ]]
}
