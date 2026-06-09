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

func TestRenderBootstrapFailsClosedWhenChecksumToolsAreMissing(t *testing.T) {
	script := renderBootstrapForTest(t)
	runDir := t.TempDir()
	toolsDir := t.TempDir()
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
  *.sha256.*) printf 'expected-checksum  opentunnel\n' > "$out" ;;
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
		"TMPDIR=" + filepath.Join(runDir, "tmp"),
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("bootstrap exited successfully; output:\n%s", output)
	}
	if strings.Contains(string(output), "EXECUTED_WITHOUT_HASH") {
		t.Fatalf("bootstrap executed downloaded binary without a checksum tool; output:\n%s", output)
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

func writeExecutable(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}
