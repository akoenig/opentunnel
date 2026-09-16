#!/usr/bin/env bats
# The prompt host.sh prints for the coding agent.

setup() {
	REPO="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
	export OPENTUNNEL_SOURCE_ONLY=1
	# shellcheck disable=SC1090
	source "$REPO/scripts/host.sh"
	ADDR="tcpGFwWCDaEFxG784iSvAxB5lGKPv5r04UC2EUHheena0rm4PQI2FrWCClZzVXKnI"
	BASE_URL="https://beta.opentunnel.sh"
	CWD="/srv/project"
	USER_NAME="deploy"
	HOST_NAME="build-01"
	OS_DESC="Linux 6.8.0 x86_64"
	IDLE=1800
	TTL=0
}

@test "prompt contains the address and the agent command" {
	run ot_print_prompt
	[[ "$output" == *"curl -fsSL https://beta.opentunnel.sh/agent | sh -s -- $ADDR"* ]]
}

@test "prompt names the remote host and working directory" {
	run ot_print_prompt
	[[ "$output" == *"Remote host: deploy@build-01 (Linux 6.8.0 x86_64), working directory /srv/project."* ]]
}

@test "prompt renders the default idle timeout in minutes" {
	run ot_print_prompt
	[[ "$output" == *"The session ends after 30 minutes without a command."* ]]
}

@test "prompt renders a custom idle timeout" {
	IDLE=600
	run ot_print_prompt
	[[ "$output" == *"ends after 10 minutes without a command"* ]]
}

@test "prompt omits the hard limit clause when TTL is off" {
	run ot_print_prompt
	[[ "$output" != *"in total"* ]]
}

@test "prompt renders the hard limit clause when TTL is set" {
	TTL=3600
	run ot_print_prompt
	[[ "$output" == *"without a command and after 60 minutes in total."* ]]
}

@test "prompt documents the file transfer commands" {
	run ot_print_prompt
	[[ "$output" == *"remote --put <local file> <remote path>"* ]]
	[[ "$output" == *"remote --get <remote path> <local file>"* ]]
	[[ "$output" == *"rsync -av -e <path>/ssh"* ]]
	[[ "$output" == *"scp -O -S <path>/ssh"* ]]
}

@test "prompt avoids words that secret scanners redact" {
	run ot_print_prompt
	[[ "$output" != *"password"* ]]
	[[ "$output" != *"token"* ]]
	[[ "$output" != *"secret"* ]]
}

@test "prompt ends with the task heading" {
	run ot_print_prompt
	[[ "${lines[${#lines[@]} - 2]}" == "Task:" ]]
}
