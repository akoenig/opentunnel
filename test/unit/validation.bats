#!/usr/bin/env bats
# Address, node key, and tunable validation in host.sh and agent.sh.

setup() {
	REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	export OPENTUNNEL_SOURCE_ONLY=1
	# shellcheck disable=SC1090
	source "$REPO/scripts/host.sh"
}

@test "accepts a real tunnel address" {
	run ot_is_addr "tcpGFwWCDaEFxG784iSvAxB5lGKPv5r04UC2EUHheena0rm4PQI2FrWCClZzVXKnI_ycYktJKpVYwAaE4bWSHhX08dxBij5D8tWmFxWCBds"
	[ "$status" -eq 0 ]
}

@test "rejects a short address" {
	run ot_is_addr "tcshort"
	[ "$status" -ne 0 ]
}

@test "rejects an address without the tc prefix" {
	run ot_is_addr "xcpGFwWCDaEFxG784iSvAxB5lGKPv5r04UC2EUHheena0rm4PQI2FrWCClZzVXKnI"
	[ "$status" -ne 0 ]
}

@test "rejects an address with a trailing newline" {
	run ot_is_addr "$(printf 'tcpGFwWCDaEFxG784iSvAxB5lGKPv5r04UC2EUHheena0rm4PQI2FrWCClZzVXKnI\n')"
	[ "$status" -eq 0 ]
	run ot_is_addr "tcpGFwWCDaEFxG784iSvAxB5lGKPv5r04UC2EUHheena0rm4PQI2FrWCClZzVXKnI
"
	[ "$status" -ne 0 ]
}

@test "rejects an address with injected shell text" {
	run ot_is_addr 'tcpGFwWCDaEFxG784iSvAxB5lGKPv5r04UC2EUHheena0rm4PQ;rm -rf /'
	[ "$status" -ne 0 ]
}

@test "accepts a well formed node key" {
	run ot_is_nodekey "nodekey:5e1a04ec2ee8b25270667b61bb99d8c0b559368c0a427ba2ebdbbd6d964cdc7b"
	[ "$status" -eq 0 ]
}

@test "rejects a node key with the wrong length" {
	run ot_is_nodekey "nodekey:5e1a04ec"
	[ "$status" -ne 0 ]
}

@test "rejects a node key with uppercase hex" {
	run ot_is_nodekey "nodekey:5E1A04EC2EE8B25270667B61BB99D8C0B559368C0A427BA2EBDBBD6D964CDC7B"
	[ "$status" -ne 0 ]
}

@test "rejects a node key without the prefix" {
	run ot_is_nodekey "5e1a04ec2ee8b25270667b61bb99d8c0b559368c0a427ba2ebdbbd6d964cdc7b"
	[ "$status" -ne 0 ]
}

@test "rejects a node key with a carriage return" {
	run ot_is_nodekey "$(printf 'nodekey:5e1a04ec2ee8b25270667b61bb99d8c0b559368c0a427ba2ebdbbd6d964cdc7b\r')"
	[ "$status" -ne 0 ]
}

@test "rejects a claim with appended text" {
	run ot_is_nodekey "nodekey:5e1a04ec2ee8b25270667b61bb99d8c0b559368c0a427ba2ebdbbd6d964cdc7b extra"
	[ "$status" -ne 0 ]
}

@test "tunables default when unset" {
	run ot_uint_env OPENTUNNEL_IDLE 1800
	[ "$output" = "1800" ]
}

@test "tunables take a whole number of seconds" {
	OPENTUNNEL_IDLE=45
	run ot_uint_env OPENTUNNEL_IDLE 1800
	[ "$output" = "45" ]
}

@test "tunables reject anything that is not a number" {
	OPENTUNNEL_IDLE="30m"
	run ot_uint_env OPENTUNNEL_IDLE 1800
	[ "$status" -ne 0 ]
	[[ "$output" == *"whole number of seconds"* ]]
}

@test "minutes round up" {
	run ot_minutes 1800
	[ "$output" = "30" ]
	run ot_minutes 20
	[ "$output" = "1" ]
	run ot_minutes 61
	[ "$output" = "2" ]
}

@test "sanitizing a rejected claim strips control characters" {
	run ot_sanitize "$(printf 'bad\x01claim')"
	[ "$output" = "bad?claim" ]
}
