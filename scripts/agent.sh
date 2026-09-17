#!/usr/bin/env bash
# OpenTunnel agent script. Served at https://beta.opentunnel.sh/agent
#
#   curl -fsSL https://beta.opentunnel.sh/agent | sh -s -- <tunnel address>
#
# Claims the tunnel opened on the remote machine, pins this machine as its
# only client, and writes a "remote" helper that runs commands over it.
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

WORK=""
CLAIM_PID=""
INSTALLED=0

ot_log() {
	printf '[opentunnel] %s\n' "$*" >&2
}

ot_die() {
	printf '[opentunnel] error: %s\n' "$*" >&2
	exit 1
}

ot_usage() {
	cat >&2 <<'USAGE'
usage: curl -fsSL https://beta.opentunnel.sh/agent | sh -s -- <tunnel address>

The tunnel address is printed by the host command on the remote machine.
USAGE
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

ot_require_tools() {
	local missing=0 tool
	for tool in curl mktemp uname ssh; do
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
	case "$1" in
	*[!A-Za-z0-9_-]*) return 1 ;;
	esac
	[ "${#1}" -ge 42 ]
}

ot_cleanup() {
	local rc=$?
	trap - EXIT INT TERM
	if [ -n "$CLAIM_PID" ] && kill -0 "$CLAIM_PID" 2>/dev/null; then
		kill -TERM "$CLAIM_PID" 2>/dev/null || true
	fi
	# The helper has to outlive the installer, so only a failed install cleans up.
	if [ "$INSTALLED" -eq 0 ] && [ -n "$WORK" ] && [ -d "$WORK" ]; then
		if [ -S "$WORK/cm.sock" ]; then
			"$WORK/ssh" -O exit opentunnel >/dev/null 2>&1 || true
		fi
		rm -rf "$WORK"
	fi
	exit "$rc"
}

ot_claim() {
	local pub
	pub=$("$TC" genkey --client --key="$WORK/client.private.json" 2>"$WORK/genkey.log" |
		grep -o 'nodekey:[0-9a-f]\{64\}' | head -n 1) || true
	[ -n "$pub" ] || ot_die "could not generate a client key: $(tail -n 2 "$WORK/genkey.log" 2>/dev/null || true)"
	# The host prints the key it was claimed by; this is what it should show.
	ot_log "claiming the tunnel as nodekey:$(printf '%s' "${pub#nodekey:}" | cut -c1-12)…"
	(printf '%s\n' "$pub" | "$TC" --key="$WORK/client.private.json" "$ADDR" >"$WORK/claim.out" 2>"$WORK/claim.log") &
	CLAIM_PID=$!
}

# A tailcat client holds one connection per node key: two client processes
# using the same key fight over it, so the claim client has to be gone before
# the tunnel connection is opened.
ot_finish_claim() {
	local waited=0
	[ -n "$CLAIM_PID" ] || return 0
	while [ "$waited" -lt 20 ]; do
		kill -0 "$CLAIM_PID" 2>/dev/null || break
		sleep 1
		waited=$((waited + 1))
	done
	if kill -0 "$CLAIM_PID" 2>/dev/null; then
		kill -TERM "$CLAIM_PID" 2>/dev/null || true
		sleep 1
		kill -KILL "$CLAIM_PID" 2>/dev/null || true
	fi
	CLAIM_PID=""
}

ot_wait_for_tunnel() {
	local waited=0
	while [ "$waited" -lt "$CONNECT_TIMEOUT" ]; do
		if "$TC" --key="$WORK/client.private.json" ping --timeout=3s "$ADDR" >/dev/null 2>&1; then
			if "$WORK/ssh" "$SSH_ALIAS" true >/dev/null 2>&1; then
				return 0
			fi
		fi
		sleep 2
		waited=$((waited + 2))
	done
	return 1
}

# One shared SSH connection carries every later command, so a burst of
# commands never opens a second tailcat client.
ot_start_master() {
	"$WORK/ssh" -N -f "$SSH_ALIAS" >>"$WORK/ssh-master.log" 2>&1 || return 1
	[ -S "$WORK/cm.sock" ]
}

ot_write_helpers() {
	# Drop-in ssh for every command, and for scp and rsync. tailcat client mode
	# is a stdio pipe to the tunnel's SSH port, so it works as a ProxyCommand.
	# The host argument is ignored; the tunnel address is baked in. Connection
	# sharing keeps all of it on the one tailcat client started at install time.
	cat >"$WORK/ssh" <<SSHW
#!/usr/bin/env bash
# OpenTunnel ssh transport. Generated by the OpenTunnel agent script.
# Usable as scp -S and rsync -e, for example:
#   rsync -av -e $WORK/ssh ./dir $SSH_ALIAS:<remote path>
#   scp -O -S $WORK/ssh <local file> $SSH_ALIAS:<remote path>
set -u

WORK=$(printf '%q' "$WORK")

exec ssh \\
	-o ProxyCommand="\$WORK/tailcat --key=\$WORK/client.private.json $ADDR 22" \\
	-o StrictHostKeyChecking=no \\
	-o UserKnownHostsFile=/dev/null \\
	-o LogLevel=ERROR \\
	-o BatchMode=yes \\
	-o ConnectTimeout=30 \\
	-o ServerAliveInterval=15 \\
	-o ServerAliveCountMax=3 \\
	-o ControlMaster=auto \\
	-o ControlPath="\$WORK/cm.sock" \\
	-o ControlPersist=$CONTROL_PERSIST \\
	"\$@"
SSHW

	cat >"$WORK/remote" <<REMOTE
#!/usr/bin/env bash
# OpenTunnel remote command helper. Generated by the OpenTunnel agent script.
set -u

WORK=$(printf '%q' "$WORK")
SSH="\$WORK/ssh"
HOST=$(printf '%q' "$SSH_ALIAS")

usage() {
	cat >&2 <<'USAGE'
usage:
  remote '<command>'                run a command on the remote machine
  remote --put <local> <remote>     copy a file to the remote machine
  remote --get <remote> <local>     copy a file from the remote machine
  remote --close                    end this client and remove its keys
USAGE
}

case "\${1:-}" in
--close)
	"\$SSH" -O exit "\$HOST" >/dev/null 2>&1 || true
	cd /
	rm -rf "\$WORK"
	echo "tunnel client removed"
	exit 0
	;;
--help | -h)
	usage
	exit 0
	;;
--put)
	[ \$# -eq 3 ] || {
		usage
		exit 2
	}
	exec "\$SSH" "\$HOST" "cat > \$(printf '%q' "\$3")" <"\$2"
	;;
--get)
	[ \$# -eq 3 ] || {
		usage
		exit 2
	}
	exec "\$SSH" "\$HOST" "cat \$(printf '%q' "\$2")" >"\$3"
	;;
"")
	usage
	exit 2
	;;
esac

exec "\$SSH" "\$HOST" "\$@"
REMOTE

	chmod 700 "$WORK/remote" "$WORK/ssh"
}

main() {
	# The temp directory and the trap come first: the POSIX shim may already
	# have created the directory, and every later failure has to remove it.
	if [ -n "${OPENTUNNEL_WORK_DIR:-}" ] && [ -d "${OPENTUNNEL_WORK_DIR:-}" ]; then
		WORK="$OPENTUNNEL_WORK_DIR"
	else
		WORK=$(mktemp -d "${TMPDIR:-/tmp}/opentunnel-agent.XXXXXX")
	fi
	chmod 700 "$WORK"
	trap ot_cleanup EXIT INT TERM

	# "sh -s -- <addr>" drops the separator, "sh agent.sh -- <addr>" keeps it.
	if [ "${1:-}" = "--" ]; then
		shift
	fi
	ADDR="${1:-}"
	if [ "$ADDR" = "--help" ] || [ "$ADDR" = "-h" ]; then
		ot_usage
		exit 0
	fi
	if [ -z "$ADDR" ]; then
		ot_usage
		exit 2
	fi
	if ! ot_is_addr "$ADDR"; then
		printf '[opentunnel] error: that does not look like a tunnel address\n' >&2
		ot_usage
		exit 2
	fi

	ot_require_tools
	ot_detect_platform
	CONNECT_TIMEOUT=$(ot_uint_env OPENTUNNEL_CONNECT_TIMEOUT 90)
	CONTROL_PERSIST=$(ot_uint_env OPENTUNNEL_CONTROL_PERSIST 600)
	SSH_ALIAS=opentunnel

	ot_download_tailcat
	ot_claim
	ot_finish_claim
	ot_write_helpers

	ot_log "connecting to the remote machine"
	if ! ot_wait_for_tunnel; then
		ot_die "could not reach the host; the tunnel may have ended or already been claimed"
	fi
	if ! ot_start_master; then
		ot_die "could not open the shared connection to the remote machine"
	fi

	INSTALLED=1

	printf 'remote helper: %s/remote\n' "$WORK"
	ot_log "run commands with: $WORK/remote '<command>'"
	ot_log "copy files with: $WORK/remote --put <local> <remote> and $WORK/remote --get <remote> <local>"
	ot_log "or with: rsync -av -e $WORK/ssh ./dir $SSH_ALIAS:<remote path>"
	ot_log "     and: scp -O -S $WORK/ssh <local file> $SSH_ALIAS:<remote path>"
	ot_log "the session ends after the host's idle timeout; run $WORK/remote --close when you are done"
}

# test/unit sources this script to exercise the functions above.
if [ -z "${OPENTUNNEL_SOURCE_ONLY:-}" ]; then
	main "$@"
fi
