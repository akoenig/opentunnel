package artifact

import (
	"fmt"
	"strings"
)

// BootstrapConfig contains the immutable artifact coordinates for a CLI bootstrap script.
type BootstrapConfig struct {
	RelayOrigin string
	Version     string
	PlatformKey string
	Checksum    string
}

// RenderBootstrap renders a POSIX sh script that installs and executes the CLI binary.
func RenderBootstrap(cfg BootstrapConfig) (string, error) {
	if cfg.RelayOrigin == "" {
		return "", fmt.Errorf("relay origin is required")
	}
	if cfg.Version == "" {
		return "", fmt.Errorf("version is required")
	}
	if cfg.PlatformKey == "" {
		return "", fmt.Errorf("platform key is required")
	}
	if cfg.Checksum == "" {
		return "", fmt.Errorf("checksum is required")
	}

	artifactName := "opentunnel-" + cfg.Version + "-" + cfg.PlatformKey
	binaryPath := "/cli/bin/" + artifactName
	checksumPath := binaryPath + ".sha256"

	script := fmt.Sprintf(`#!/bin/sh
set -eu
umask 077

relay_origin=%s
version=%s
platform=%s
expected_checksum=%s
binary_url="${relay_origin}%s"
checksum_url="${relay_origin}%s"
cache_dir="${TMPDIR:-/tmp}/opentunnel-cli/${platform}/${version}/${expected_checksum}"
bin="${cache_dir}/opentunnel"

if [ ! -x "$bin" ]; then
  mkdir -p "$cache_dir"
  tmp_bin="${cache_dir}/opentunnel.$$"
  tmp_checksum="${cache_dir}/opentunnel.sha256.$$"
  trap 'rm -f "$tmp_bin" "$tmp_checksum"' EXIT HUP INT TERM

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$binary_url" -o "$tmp_bin"
    curl -fsSL "$checksum_url" -o "$tmp_checksum"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$tmp_bin" "$binary_url"
    wget -q -O "$tmp_checksum" "$checksum_url"
  else
    printf 'opentunnel: curl or wget is required\n' >&2
    exit 1
  fi

  actual_expected=$(awk '{print $1}' "$tmp_checksum")
  if [ "$actual_expected" != "$expected_checksum" ]; then
    printf 'opentunnel: checksum metadata mismatch\n' >&2
    exit 1
  fi

  if command -v sha256sum >/dev/null 2>&1; then
    actual_checksum=$(sha256sum "$tmp_bin" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    actual_checksum=$(shasum -a 256 "$tmp_bin" | awk '{print $1}')
  else
    actual_checksum=$expected_checksum
  fi

  if [ "$actual_checksum" != "$expected_checksum" ]; then
    printf 'opentunnel: checksum mismatch\n' >&2
    exit 1
  fi

  chmod 700 "$tmp_bin"
  mv "$tmp_bin" "$bin"
  rm -f "$tmp_checksum"
  trap - EXIT HUP INT TERM
fi

export OPENTUNNEL_RELAY_ORIGIN="$relay_origin"
exec "$bin" "$@"
`, shellQuote(cfg.RelayOrigin), shellQuote(cfg.Version), shellQuote(cfg.PlatformKey), shellQuote(cfg.Checksum), binaryPath, checksumPath)

	return script, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
