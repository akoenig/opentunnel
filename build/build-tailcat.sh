#!/usr/bin/env bash
# Builds the tailcat binaries OpenTunnel serves, from the pinned upstream
# commit in build/TAILCAT_COMMIT.
#
#   build/build-tailcat.sh
#
# Output: dist/bin/<VERSION>/tailcat_<os>_<arch>, SHA256SUMS, LICENSE.tailcat.
#
# Environment:
#   TAILCAT_SRC   reuse an existing checkout instead of cloning
#   TARGETS       space separated os/arch list (default: the four we ship)

set -euo pipefail

repo_root=$(unset CDPATH && cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_root"

VERSION=$(tr -d '[:space:]' <VERSION)
COMMIT=$(tr -d '[:space:]' <build/TAILCAT_COMMIT)
TARGETS=${TARGETS:-"linux/amd64 linux/arm64 darwin/amd64 darwin/arm64"}
OUT="$repo_root/dist/bin/$VERSION"

log() {
	printf '[build-tailcat] %s\n' "$*" >&2
}

command -v go >/dev/null 2>&1 || {
	log "error: go is required"
	exit 1
}
command -v git >/dev/null 2>&1 || {
	log "error: git is required"
	exit 1
}

src=${TAILCAT_SRC:-}
if [ -z "$src" ]; then
	src="$repo_root/dist/tailcat-src"
	if [ ! -d "$src/.git" ]; then
		log "cloning tailcat"
		rm -rf "$src"
		mkdir -p "$(dirname "$src")"
		git clone --quiet https://github.com/tailscale/tailcat "$src"
	fi
fi

if [ "$(git -C "$src" rev-parse HEAD)" != "$COMMIT" ]; then
	log "checking out $COMMIT"
	git -C "$src" fetch --quiet origin "$COMMIT" 2>/dev/null || git -C "$src" fetch --quiet origin
	git -C "$src" checkout --quiet "$COMMIT"
fi
actual=$(git -C "$src" rev-parse HEAD)
[ "$actual" = "$COMMIT" ] || {
	log "error: checkout is at $actual, expected $COMMIT"
	exit 1
}

rm -rf "$OUT"
mkdir -p "$OUT"

for target in $TARGETS; do
	os=${target%%/*}
	arch=${target##*/}
	log "building tailcat_${os}_${arch}"
	(
		cd "$src"
		CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
			go build -trimpath -ldflags="-s -w" \
			-o "$OUT/tailcat_${os}_${arch}" ./cmd/tailcat
	)
done

cp "$src/LICENSE" "$OUT/LICENSE.tailcat"

(
	cd "$OUT"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum tailcat_* >SHA256SUMS
	else
		shasum -a 256 tailcat_* >SHA256SUMS
	fi
)

# Smoke test the native binary when one was built for this machine.
host_os=$(uname -s | tr '[:upper:]' '[:lower:]')
host_arch=$(uname -m)
case "$host_arch" in
x86_64 | amd64) host_arch=amd64 ;;
aarch64 | arm64) host_arch=arm64 ;;
esac
native="$OUT/tailcat_${host_os}_${host_arch}"
if [ -x "$native" ]; then
	log "smoke testing $(basename "$native")"
	"$native" version >/dev/null
	"$native" serve --help 2>&1 | grep -q -- '-- <command>' ||
		{
			log "error: the built tailcat does not support ForceCommand (-- <command>)"
			exit 1
		}
fi

log "wrote $OUT"
cat "$OUT/SHA256SUMS" >&2
