#!/usr/bin/env bats
# Platform detection in host.sh and agent.sh.

setup() {
	REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	export OPENTUNNEL_SOURCE_ONLY=1
	FAKE="$BATS_TEST_TMPDIR/bin"
	mkdir -p "$FAKE"
}

# fake_uname <kernel> <machine>
fake_uname() {
	cat >"$FAKE/uname" <<EOF
#!/bin/sh
case "\$1" in
-s) echo "$1" ;;
-m) echo "$2" ;;
-srm) echo "$1 0.0 $2" ;;
*) echo "$1" ;;
esac
EOF
	chmod 700 "$FAKE/uname"
	PATH="$FAKE:$PATH"
}

detect() {
	# shellcheck disable=SC1090
	source "$REPO/scripts/host.sh"
	ot_detect_platform
	echo "$OS/$ARCH"
}

@test "linux x86_64 maps to linux/amd64" {
	fake_uname Linux x86_64
	run detect
	[ "$status" -eq 0 ]
	[ "$output" = "linux/amd64" ]
}

@test "linux aarch64 maps to linux/arm64" {
	fake_uname Linux aarch64
	run detect
	[ "$output" = "linux/arm64" ]
}

@test "darwin arm64 maps to darwin/arm64" {
	fake_uname Darwin arm64
	run detect
	[ "$output" = "darwin/arm64" ]
}

@test "darwin amd64 maps to darwin/amd64" {
	fake_uname Darwin amd64
	run detect
	[ "$output" = "darwin/amd64" ]
}

@test "freebsd is rejected" {
	fake_uname FreeBSD amd64
	run detect
	[ "$status" -ne 0 ]
	[[ "$output" == *"unsupported OS"* ]]
}

@test "windows is rejected" {
	fake_uname MINGW64_NT-10.0 x86_64
	run detect
	[ "$status" -ne 0 ]
	[[ "$output" == *"unsupported OS"* ]]
}

@test "armv7 is rejected" {
	fake_uname Linux armv7l
	run detect
	[ "$status" -ne 0 ]
	[[ "$output" == *"unsupported architecture"* ]]
}

@test "agent.sh maps platforms the same way" {
	fake_uname Darwin arm64
	run bash -c "source '$REPO/scripts/agent.sh'; ot_detect_platform; echo \"\$OS/\$ARCH\""
	[ "$output" = "darwin/arm64" ]
}
