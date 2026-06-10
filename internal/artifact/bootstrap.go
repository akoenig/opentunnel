package artifact

import (
	"fmt"
	"net/url"
	"strings"
)

// BootstrapArtifact contains the checksum for one supported CLI artifact.
type BootstrapArtifact struct {
	PlatformKey string
	Checksum    string
}

// BootstrapConfig contains the immutable artifact coordinates for a CLI bootstrap script.
type BootstrapConfig struct {
	RelayOrigin string
	Version     string
	Artifacts   []BootstrapArtifact
}

// RenderBootstrap renders a POSIX sh script that installs and executes the CLI binary.
func RenderBootstrap(cfg BootstrapConfig) (string, error) {
	if cfg.RelayOrigin == "" {
		return "", fmt.Errorf("relay origin is required")
	}
	if err := validateRelayOrigin(cfg.RelayOrigin); err != nil {
		return "", err
	}
	if cfg.Version == "" {
		return "", fmt.Errorf("version is required")
	}

	checksums, err := validateBootstrapArtifacts(cfg.Artifacts)
	if err != nil {
		return "", err
	}

	script := fmt.Sprintf(`#!/bin/sh
set -eu
umask 077

relay_origin=%s
version=%s
os_name=$(uname -s)
arch_name=$(uname -m)
case "$os_name/$arch_name" in
  Linux/x86_64|Linux/amd64) platform=linux-amd64 ;;
  Linux/aarch64|Linux/arm64) platform=linux-arm64 ;;
  Darwin/x86_64|Darwin/amd64) platform=darwin-amd64 ;;
  Darwin/arm64|Darwin/aarch64) platform=darwin-arm64 ;;
  *)
    printf 'opentunnel: unsupported platform %%s/%%s\n' "$os_name" "$arch_name" >&2
    exit 1
    ;;
esac
case "$platform" in
%s  *)
    printf 'opentunnel: no checksum for platform %%s\n' "$platform" >&2
    exit 1
    ;;
esac
binary_path="/cli/bin/opentunnel-${version}-${platform}"
checksum_path="${binary_path}.sha256"
binary_url="${relay_origin}${binary_path}"
checksum_url="${relay_origin}${checksum_path}"
if ! uid=$(id -u 2>/dev/null); then
  printf 'opentunnel: cannot determine current user id\n' >&2
  exit 1
fi
cache_base="${TMPDIR:-/tmp}/opentunnel-cli-${uid}"
if [ -e "$cache_base" ] && [ ! -d "$cache_base" ]; then
  printf 'opentunnel: cache root is not a directory\n' >&2
  exit 1
fi
mkdir -p "$cache_base"
chmod 700 "$cache_base"
cache_dir="${cache_base}/${platform}/${version}/${expected_checksum}"
bin="${cache_dir}/opentunnel"
download_root=$(mktemp -d "${TMPDIR:-/tmp}/opentunnel-cli-download.XXXXXX")
tmp_bin="${download_root}/opentunnel.download"
tmp_checksum="${download_root}/opentunnel.sha256"
trap 'rm -rf "$download_root"' EXIT HUP INT TERM

checksum_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    printf 'opentunnel: sha256sum or shasum is required\n' >&2
    return 1
  fi
}

if [ -f "$bin" ]; then
  if actual_checksum=$(checksum_file "$bin") && [ "$actual_checksum" = "$expected_checksum" ]; then
    export OPENTUNNEL_RELAY_ORIGIN="$relay_origin"
    exec "$bin" "$@"
  fi
  rm -f "$bin"
fi

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

if ! actual_checksum=$(checksum_file "$tmp_bin"); then
  exit 1
fi

if [ "$actual_checksum" != "$expected_checksum" ]; then
  printf 'opentunnel: checksum mismatch\n' >&2
  exit 1
fi

chmod 700 "$tmp_bin"
mkdir -p "$cache_dir"
mv "$tmp_bin" "$bin"
rm -f "$tmp_checksum"

export OPENTUNNEL_RELAY_ORIGIN="$relay_origin"
exec "$bin" "$@"
`, shellQuote(cfg.RelayOrigin), shellQuote(cfg.Version), renderChecksumCases(checksums))

	return script, nil
}

func validateBootstrapArtifacts(artifacts []BootstrapArtifact) (map[string]string, error) {
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("artifacts are required")
	}

	checksums := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		if !IsSupportedPlatform(artifact.PlatformKey) {
			return nil, fmt.Errorf("unsupported platform %q", artifact.PlatformKey)
		}
		if !isSHA256Hex(artifact.Checksum) {
			return nil, fmt.Errorf("checksum must be 64 hex characters for platform %q", artifact.PlatformKey)
		}
		if _, ok := checksums[artifact.PlatformKey]; ok {
			return nil, fmt.Errorf("duplicate checksum for platform %q", artifact.PlatformKey)
		}
		checksums[artifact.PlatformKey] = artifact.Checksum
	}

	for _, platform := range SupportedPlatforms() {
		if _, ok := checksums[platform]; !ok {
			return nil, fmt.Errorf("checksum is required for platform %q", platform)
		}
	}

	return checksums, nil
}

func isSHA256Hex(checksum string) bool {
	if len(checksum) != 64 {
		return false
	}

	for _, char := range checksum {
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') {
			continue
		}
		return false
	}

	return true
}

func renderChecksumCases(checksums map[string]string) string {
	var builder strings.Builder
	for _, platform := range SupportedPlatforms() {
		builder.WriteString("  ")
		builder.WriteString(platform)
		builder.WriteString(") expected_checksum=")
		builder.WriteString(shellQuote(checksums[platform]))
		builder.WriteString(" ;;\n")
	}
	return builder.String()
}

func validateRelayOrigin(relayOrigin string) error {
	if strings.HasPrefix(relayOrigin, "-") {
		return fmt.Errorf("relay origin must be an http or https origin")
	}

	parsed, err := url.Parse(relayOrigin)
	if err != nil {
		return fmt.Errorf("relay origin must be an http or https origin: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("relay origin must use http or https")
	}
	if parsed.Host == "" {
		return fmt.Errorf("relay origin must include a host")
	}
	if parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
		return fmt.Errorf("relay origin must not include userinfo, path, query, or fragment")
	}

	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
