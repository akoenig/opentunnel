#!/usr/bin/env bash
# Turns scripts/host.sh and scripts/agent.sh into the self-contained scripts
# served at https://beta.opentunnel.sh:
#
#   - "# @@VERSION@@"   becomes the released version
#   - "# @@BASE_URL@@"  becomes the base URL the scripts download from
#   - "# @@CHECKSUMS@@" becomes the pinned per-target sha256 of tailcat
#   - "# @@OT_EXEC@@"   becomes an inline copy of scripts/ot-exec.sh (host only)
#
# The result is wrapped in a POSIX shim so "curl ... | sh" works: the shim
# writes the bash body to a private temp dir and re-executes it with bash.
#
# Environment:
#   OPENTUNNEL_DEFAULT_BASE_URL   base URL baked into the scripts
#   OPENTUNNEL_SKIP_CHECKSUMS     build without pinned checksums (tests only)

set -euo pipefail

repo_root=$(unset CDPATH && cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

VERSION=$(tr -d '[:space:]' <VERSION)
DEFAULT_BASE_URL=${OPENTUNNEL_DEFAULT_BASE_URL:-https://beta.opentunnel.sh}
SUMS="dist/bin/$VERSION/SHA256SUMS"
BODY_EOF=OPENTUNNEL_BODY_EOF
EXEC_EOF=OPENTUNNEL_OT_EXEC_EOF

log() {
	printf '[embed] %s\n' "$*" >&2
}

fail() {
	printf '[embed] error: %s\n' "$*" >&2
	exit 1
}

if [ -z "${OPENTUNNEL_SKIP_CHECKSUMS:-}" ] && [ ! -f "$SUMS" ]; then
	fail "missing $SUMS; run build/build-tailcat.sh first (or set OPENTUNNEL_SKIP_CHECKSUMS=1)"
fi

emit_version() {
	printf 'VERSION=%s\n' "$(printf '%q' "$VERSION")"
}

# The generated lines are shell source, so the single quotes are deliberate.
# shellcheck disable=SC2016
emit_base_url() {
	printf 'BASE_URL="${OPENTUNNEL_BASE_URL:-%s}"\n' "$DEFAULT_BASE_URL"
}

# shellcheck disable=SC2016
emit_checksums() {
	local name sum target
	printf 'ot_expected_sha256() {\n\tcase "$1" in\n'
	if [ -n "${OPENTUNNEL_SKIP_CHECKSUMS:-}" ]; then
		log "warning: building without pinned checksums"
	else
		while read -r sum name; do
			[ -n "${name:-}" ] || continue
			target=${name#tailcat_}
			printf '\t%s) printf %s ;;\n' "$target" "$(printf '%q' "$sum")"
		done <"$SUMS"
	fi
	printf '\t*) printf '"''"' ;;\n\tesac\n}\n'
}

# shellcheck disable=SC2016
emit_ot_exec() {
	if grep -q "^$EXEC_EOF\$" scripts/ot-exec.sh; then
		fail "scripts/ot-exec.sh contains the heredoc delimiter $EXEC_EOF"
	fi
	printf 'ot_write_exec_wrapper() {\n\tcat >"$WORK/ot-exec.sh" <<'"'"'%s'"'"'\n' "$EXEC_EOF"
	cat scripts/ot-exec.sh
	printf '%s\n}\n' "$EXEC_EOF"
}

# expand <source> <with_ot_exec>
expand() {
	local src=$1 with_ot_exec=$2 line
	local seen_version=0 seen_base=0 seen_sums=0 seen_exec=0
	while IFS= read -r line || [ -n "$line" ]; do
		case "$line" in
		'# @@VERSION@@')
			emit_version
			seen_version=1
			;;
		'# @@BASE_URL@@')
			emit_base_url
			seen_base=1
			;;
		'# @@CHECKSUMS@@')
			emit_checksums
			seen_sums=1
			;;
		'# @@OT_EXEC@@')
			[ "$with_ot_exec" = yes ] || fail "$src has an unexpected @@OT_EXEC@@ marker"
			emit_ot_exec
			seen_exec=1
			;;
		*)
			printf '%s\n' "$line"
			;;
		esac
	done <"$src"
	[ "$seen_version" -eq 1 ] || fail "$src is missing the @@VERSION@@ marker"
	[ "$seen_base" -eq 1 ] || fail "$src is missing the @@BASE_URL@@ marker"
	[ "$seen_sums" -eq 1 ] || fail "$src is missing the @@CHECKSUMS@@ marker"
	if [ "$with_ot_exec" = yes ] && [ "$seen_exec" -ne 1 ]; then
		fail "$src is missing the @@OT_EXEC@@ marker"
	fi
}

# wrap <name> <body file> <output>
# shellcheck disable=SC2016
wrap() {
	local name=$1 body=$2 out=$3
	if grep -q "^$BODY_EOF\$" "$body"; then
		fail "$name body contains the heredoc delimiter $BODY_EOF"
	fi
	{
		printf '%s\n' '#!/bin/sh'
		printf '# OpenTunnel %s script, version %s. https://opentunnel.sh\n' "$name" "$VERSION"
		printf '%s\n' '#'
		printf '%s\n' '# POSIX shim: this file may be piped into any /bin/sh. It writes the bash'
		printf '%s\n' '# body below into a private temp directory and re-executes it with bash.'
		printf '%s\n' 'set -eu'
		printf '%s\n' 'if ! command -v bash >/dev/null 2>&1; then'
		printf '%s\n' '	echo "[opentunnel] error: bash is required" >&2'
		printf '%s\n' '	exit 1'
		printf '%s\n' 'fi'
		printf 'OPENTUNNEL_WORK_DIR=$(mktemp -d "${TMPDIR:-/tmp}/opentunnel-%s.XXXXXX") || exit 1\n' "$name"
		printf '%s\n' 'chmod 700 "$OPENTUNNEL_WORK_DIR"'
		printf '%s\n' 'export OPENTUNNEL_WORK_DIR'
		printf 'cat >"$OPENTUNNEL_WORK_DIR/self.sh" <<'"'"'%s'"'"'\n' "$BODY_EOF"
		cat "$body"
		printf '%s\n' "$BODY_EOF"
		printf '%s\n' 'exec bash "$OPENTUNNEL_WORK_DIR/self.sh" "$@"'
	} >"$out"
	chmod 755 "$out"
}

mkdir -p dist
tmp=$(mktemp -d "${TMPDIR:-/tmp}/opentunnel-embed.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

expand scripts/host.sh yes >"$tmp/host-body.sh"
expand scripts/agent.sh no >"$tmp/agent-body.sh"

for f in "$tmp/host-body.sh" "$tmp/agent-body.sh"; do
	if grep -q '@@[A-Z_]*@@' "$f"; then
		fail "unreplaced marker left in $(basename "$f"): $(grep -o '@@[A-Z_]*@@' "$f" | head -n 1)"
	fi
done

bash -n "$tmp/host-body.sh"
bash -n "$tmp/agent-body.sh"

wrap host "$tmp/host-body.sh" dist/host.sh
wrap agent "$tmp/agent-body.sh" dist/agent.sh

bash -n dist/host.sh
bash -n dist/agent.sh

log "wrote dist/host.sh and dist/agent.sh (version $VERSION, base URL $DEFAULT_BASE_URL)"
