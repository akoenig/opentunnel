package artifact

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderBootstrapRendersPOSIXBootstrapScript(t *testing.T) {
	script, err := RenderBootstrap(BootstrapConfig{
		RelayOrigin: "http://relay.example",
		Version:     "dev",
		PlatformKey: "linux-amd64",
		Checksum:    "abc123",
	})
	if err != nil {
		t.Fatalf("RenderBootstrap returned error: %v", err)
	}

	wants := []string{
		"relay_origin='http://relay.example'",
		"/cli/bin/opentunnel-dev-linux-amd64",
		"/cli/bin/opentunnel-dev-linux-amd64.sha256",
		"umask 077",
		"${TMPDIR:-/tmp}",
		"mktemp -d \"${TMPDIR:-/tmp}/opentunnel-cli.XXXXXX\"",
		"sha256sum",
		"shasum -a 256",
		"chmod 700",
		"OPENTUNNEL_RELAY_ORIGIN",
		"exec \"$bin\" \"$@\"",
	}
	for _, want := range wants {
		if !strings.Contains(script, want) {
			t.Fatalf("RenderBootstrap() missing %q in script:\n%s", want, script)
		}
	}
}

func TestRenderBootstrapRejectsMissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  BootstrapConfig
	}{
		{
			name: "relay origin",
			cfg: BootstrapConfig{
				Version:     "dev",
				PlatformKey: "linux-amd64",
				Checksum:    "abc123",
			},
		},
		{
			name: "version",
			cfg: BootstrapConfig{
				RelayOrigin: "http://relay.example",
				PlatformKey: "linux-amd64",
				Checksum:    "abc123",
			},
		},
		{
			name: "platform key",
			cfg: BootstrapConfig{
				RelayOrigin: "http://relay.example",
				Version:     "dev",
				Checksum:    "abc123",
			},
		},
		{
			name: "checksum",
			cfg: BootstrapConfig{
				RelayOrigin: "http://relay.example",
				Version:     "dev",
				PlatformKey: "linux-amd64",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := RenderBootstrap(tt.cfg); err == nil {
				t.Fatal("RenderBootstrap returned nil error")
			}
		})
	}
}

func TestRenderBootstrapRejectsInvalidRelayOrigins(t *testing.T) {
	tests := []struct {
		name        string
		relayOrigin string
	}{
		{name: "leading dash", relayOrigin: "-http://relay.example"},
		{name: "missing scheme", relayOrigin: "relay.example"},
		{name: "missing host", relayOrigin: "https://"},
		{name: "non http scheme", relayOrigin: "file://relay.example"},
		{name: "path", relayOrigin: "https://relay.example/cli"},
		{name: "query", relayOrigin: "https://relay.example?token=abc"},
		{name: "fragment", relayOrigin: "https://relay.example#cli"},
		{name: "userinfo", relayOrigin: "https://user:pass@relay.example"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RenderBootstrap(BootstrapConfig{
				RelayOrigin: tt.relayOrigin,
				Version:     "dev",
				PlatformKey: "linux-amd64",
				Checksum:    "abc123",
			})
			if err == nil {
				t.Fatal("RenderBootstrap returned nil error")
			}
			if !strings.Contains(err.Error(), "relay origin") {
				t.Fatalf("RenderBootstrap error %q does not mention relay origin", err)
			}
		})
	}
}

func TestRenderBootstrapFailsClosedWhenChecksumToolsAreMissing(t *testing.T) {
	script := renderBootstrapForTest(t)
	runDir := t.TempDir()
	toolsDir := t.TempDir()
	writeExecutable(t, filepath.Join(toolsDir, "mktemp"), `#!/bin/sh
exec /usr/bin/mktemp "$@"
`)
	writeExecutable(t, filepath.Join(toolsDir, "curl"), `#!/bin/sh
out=
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    shift
    out=$1
  fi
  shift
done
case "$out" in
  *.sha256) printf 'expected-checksum  opentunnel\n' > "$out" ;;
  *) printf '#!/bin/sh\nprintf '\''EXECUTED_WITHOUT_HASH\n'\''\n' > "$out" ;;
esac
`)
	writeExecutable(t, filepath.Join(toolsDir, "awk"), `#!/bin/sh
if [ "$#" -gt 1 ]; then
  read -r line < "$2"
else
  read -r line
fi
set -- $line
printf '%s\n' "$1"
`)
	writeExecutable(t, filepath.Join(toolsDir, "mkdir"), `#!/bin/sh
exec /usr/bin/mkdir "$@"
`)
	writeExecutable(t, filepath.Join(toolsDir, "chmod"), `#!/bin/sh
exec /usr/bin/chmod "$@"
`)
	writeExecutable(t, filepath.Join(toolsDir, "mv"), `#!/bin/sh
exec /usr/bin/mv "$@"
`)
	writeExecutable(t, filepath.Join(toolsDir, "rm"), `#!/bin/sh
exec /usr/bin/rm "$@"
`)

	cmd := exec.Command("/bin/sh", script)
	cmd.Dir = runDir
	cmd.Env = []string{
		"PATH=" + toolsDir,
		"TMPDIR=" + runDir,
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("bootstrap exited successfully; output:\n%s", output)
	}
	if strings.Contains(string(output), "EXECUTED_WITHOUT_HASH") {
		t.Fatalf("bootstrap executed downloaded binary without a checksum tool; output:\n%s", output)
	}
	if !strings.Contains(string(output), "sha256sum or shasum is required") {
		t.Fatalf("bootstrap failed before checksum tool selection; output:\n%s", output)
	}
}

func TestRenderBootstrapDoesNotExecutePoisonedCache(t *testing.T) {
	script := renderBootstrapForTest(t)
	runDir := t.TempDir()
	tmpDir := filepath.Join(runDir, "tmp")
	bin := filepath.Join(tmpDir, "opentunnel-cli", "linux-amd64", "dev", "expected-checksum", "opentunnel")
	writeExecutable(t, bin, `#!/bin/sh
printf 'POISON_CACHE_EXECUTED\n'
`)

	cmd := exec.Command("/bin/sh", script)
	cmd.Dir = runDir
	cmd.Env = []string{
		"PATH=" + t.TempDir(),
		"TMPDIR=" + tmpDir,
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("bootstrap exited successfully; output:\n%s", output)
	}
	if strings.Contains(string(output), "POISON_CACHE_EXECUTED") {
		t.Fatalf("bootstrap executed poisoned cache binary; output:\n%s", output)
	}
}

func TestRenderBootstrapDoesNotExecuteCommandSubstitutionInArtifactCoordinates(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		platformKey string
	}{
		{
			name:        "version",
			version:     "dev$(/usr/bin/touch MARKER)",
			platformKey: "linux-amd64",
		},
		{
			name:        "platform key",
			version:     "dev",
			platformKey: "linux$(/usr/bin/touch MARKER)-amd64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runDir := t.TempDir()
			marker := filepath.Join(runDir, "command-substitution-executed")
			script, err := RenderBootstrap(BootstrapConfig{
				RelayOrigin: "http://relay.example",
				Version:     strings.ReplaceAll(tt.version, "MARKER", marker),
				PlatformKey: strings.ReplaceAll(tt.platformKey, "MARKER", marker),
				Checksum:    "expected-checksum",
			})
			if err != nil {
				t.Fatalf("RenderBootstrap returned error: %v", err)
			}
			path := filepath.Join(t.TempDir(), "bootstrap.sh")
			writeExecutable(t, path, script)

			toolsDir := t.TempDir()
			writeExecutable(t, filepath.Join(toolsDir, "mktemp"), `#!/bin/sh
exec /usr/bin/mktemp "$@"
`)

			cmd := exec.Command("/bin/sh", path)
			cmd.Dir = runDir
			cmd.Env = []string{
				"PATH=" + toolsDir,
				"TMPDIR=" + filepath.Join(runDir, "tmp"),
			}
			_, _ = cmd.CombinedOutput()

			if _, err := os.Stat(marker); err == nil {
				t.Fatalf("bootstrap executed command substitution for %s", tt.name)
			} else if !os.IsNotExist(err) {
				t.Fatalf("Stat returned error: %v", err)
			}
		})
	}
}

func TestRenderBootstrapDoesNotUsePrecreatedPredictableCache(t *testing.T) {
	script := renderBootstrapForTest(t)
	runDir := t.TempDir()
	tmpDir := filepath.Join(runDir, "tmp")
	bin := filepath.Join(tmpDir, "opentunnel-cli", "linux-amd64", "dev", "expected-checksum", "opentunnel")
	writeExecutable(t, bin, `#!/bin/sh
printf 'PREDICTABLE_CACHE_EXECUTED\n'
`)

	toolsDir := t.TempDir()
	writeExecutable(t, filepath.Join(toolsDir, "sha256sum"), `#!/bin/sh
printf 'expected-checksum  %s\n' "$1"
`)
	writeExecutable(t, filepath.Join(toolsDir, "awk"), `#!/bin/sh
if [ "$#" -gt 1 ]; then
  read -r line < "$2"
else
  read -r line
fi
set -- $line
printf '%s\n' "$1"
`)
	writeExecutable(t, filepath.Join(toolsDir, "mktemp"), `#!/bin/sh
exec /usr/bin/mktemp "$@"
`)

	cmd := exec.Command("/bin/sh", script)
	cmd.Dir = runDir
	cmd.Env = []string{
		"PATH=" + toolsDir,
		"TMPDIR=" + tmpDir,
	}
	output, _ := cmd.CombinedOutput()
	if strings.Contains(string(output), "PREDICTABLE_CACHE_EXECUTED") {
		t.Fatalf("bootstrap executed precreated predictable cache binary; output:\n%s", output)
	}
}

func TestRenderBootstrapReusesVerifiedPrivateCache(t *testing.T) {
	script := renderBootstrapForTest(t)
	runDir := t.TempDir()
	toolsDir := t.TempDir()
	cacheDir := filepath.Join(runDir, "cache")
	tmpDir := filepath.Join(runDir, "tmp")
	binaryDownloads := filepath.Join(runDir, "binary-downloads")
	checksumCalls := filepath.Join(runDir, "checksum-calls")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	writeExecutable(t, filepath.Join(toolsDir, "curl"), `#!/bin/sh
out=
url=
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    shift
    out=$1
  else
    url=$1
  fi
  shift
done
case "$url" in
  *.sha256) printf 'expected-checksum  opentunnel\n' > "$out" ;;
  *)
    count=0
    if [ -f "$BINARY_DOWNLOADS" ]; then
      count=$(cat "$BINARY_DOWNLOADS")
    fi
    count=$((count + 1))
    printf '%s\n' "$count" > "$BINARY_DOWNLOADS"
    printf '#!/bin/sh\nprintf '\''CACHED_BINARY_EXECUTED\\n'\''\n' > "$out"
    ;;
esac
`)
	writeExecutable(t, filepath.Join(toolsDir, "sha256sum"), `#!/bin/sh
count=0
if [ -f "$CHECKSUM_CALLS" ]; then
  count=$(cat "$CHECKSUM_CALLS")
fi
count=$((count + 1))
printf '%s\n' "$count" > "$CHECKSUM_CALLS"
printf 'expected-checksum  %s\n' "$1"
`)

	for i := 0; i < 2; i++ {
		cmd := exec.Command("/bin/sh", script)
		cmd.Dir = runDir
		cmd.Env = []string{
			"PATH=" + toolsDir + ":/usr/bin:/bin",
			"XDG_CACHE_HOME=" + cacheDir,
			"HOME=",
			"TMPDIR=" + tmpDir,
			"BINARY_DOWNLOADS=" + binaryDownloads,
			"CHECKSUM_CALLS=" + checksumCalls,
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("bootstrap run %d failed: %v\n%s", i+1, err, output)
		}
		if !strings.Contains(string(output), "CACHED_BINARY_EXECUTED") {
			t.Fatalf("bootstrap run %d did not execute binary; output:\n%s", i+1, output)
		}
	}

	if got := strings.TrimSpace(readTestFile(t, binaryDownloads)); got != "1" {
		t.Fatalf("binary download count = %q, want 1", got)
	}
	if got := strings.TrimSpace(readTestFile(t, checksumCalls)); got != "2" {
		t.Fatalf("checksum verification count = %q, want 2", got)
	}
}

func renderBootstrapForTest(t *testing.T) string {
	t.Helper()
	script, err := RenderBootstrap(BootstrapConfig{
		RelayOrigin: "http://relay.example",
		Version:     "dev",
		PlatformKey: "linux-amd64",
		Checksum:    "expected-checksum",
	})
	if err != nil {
		t.Fatalf("RenderBootstrap returned error: %v", err)
	}
	path := filepath.Join(t.TempDir(), "bootstrap.sh")
	writeExecutable(t, path, script)
	return path
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	return string(contents)
}

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
