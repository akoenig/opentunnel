# OpenTunnel Multi-Arch Relay Artifacts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a self-contained Docker relay image that serves Linux/macOS `amd64`/`arm64` temporary CLI binaries from one artifact directory selected by `/cli` at runtime.

**Architecture:** Add a repo-root `VERSION` file and a tiny build-info package whose `Version` variable defaults to `dev` and can be set by `-ldflags`. Replace single-artifact relay serving with a strict artifact directory containing the four supported platform binaries. Render one POSIX bootstrapper that detects `uname` OS/arch, maps to a supported platform key, downloads the matching binary and checksum, verifies it, caches it, and executes it.

**Tech Stack:** Go 1.23, Gorilla WebSocket, POSIX `sh`, Docker multi-stage builds, GitHub Actions.

---

## File Structure

- Create: `VERSION` - repository version source, initially `dev`.
- Create: `internal/buildinfo/version.go` - embedded/default binary version variable.
- Create: `internal/buildinfo/version_test.go` - verifies the default development version.
- Modify: `internal/artifact/platform.go` - define supported platform keys and artifact name/path helpers.
- Modify: `internal/artifact/platform_test.go` - cover supported platforms and artifact naming.
- Modify: `internal/artifact/bootstrap.go` - render platform-selecting POSIX bootstrapper.
- Modify: `internal/artifact/bootstrap_test.go` - update bootstrap tests for platform detection and checksum maps.
- Modify: `internal/relay/server.go` - validate and serve artifact directories instead of one binary path.
- Modify: `internal/relay/server_test.go` - update relay artifact tests for all supported platforms.
- Modify: `cmd/opentunnel/main.go` - replace `--artifact-path` with `--artifact-dir`, require `--public-url`, and default version from `buildinfo.Version`.
- Modify: `cmd/opentunnel/main_test.go` - update CLI parsing and option tests.
- Modify: `deploy/docker/Dockerfile` - cross-build relay and four artifacts into the image.
- Modify: `deploy/docker/README.md` - document two-step Docker operation.
- Modify: `deploy/systemd/opentunnel-relay.service` - use `--artifact-dir`.
- Modify: `deploy/systemd/opentunnel-relay.env.example` - use `OPENTUNNEL_ARTIFACT_DIR` and unprefixed versions.
- Modify: `deploy/systemd/README.md` - update native deployment artifact directory guidance.
- Modify: `docs/public-v1/self-hosting.md` - update Docker-first self-hosting and artifact flag docs.
- Modify: `docs/public-v1/operations.md` - update release/build verification and artifact directory docs.
- Modify: `README.md` - update quickstart commands and version/artifact wording.
- Modify: `.github/workflows/ci.yml` - align artifact matrix with supported platforms and versioned filenames.

---

### Task 1: Add Repository Version Source

**Files:**
- Create: `VERSION`
- Create: `internal/buildinfo/version.go`
- Create: `internal/buildinfo/version_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/buildinfo/version_test.go`:

```go
package buildinfo

import "testing"

func TestVersionDefaultsToDev(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("Version = %q, want %q", Version, "dev")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/buildinfo -count=1`

Expected: FAIL with an error that package `opentunnel/internal/buildinfo` has no non-test Go files or `Version` is undefined.

- [ ] **Step 3: Add version source files**

Create `VERSION`:

```text
dev
```

Create `internal/buildinfo/version.go`:

```go
package buildinfo

// Version is the repository version embedded into release builds with -ldflags.
var Version = "dev"
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/buildinfo -count=1`

Expected: PASS.

- [ ] **Step 5: Run full test loop for this task**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add VERSION internal/buildinfo/version.go internal/buildinfo/version_test.go
git commit -m "feat: add repository version source"
```

---

### Task 2: Define Supported Artifact Platforms

**Files:**
- Modify: `internal/artifact/platform.go`
- Modify: `internal/artifact/platform_test.go`

- [ ] **Step 1: Add failing platform tests**

Append these tests to `internal/artifact/platform_test.go`:

```go
func TestSupportedPlatformsContainsV1UnixAndDarwinPlatforms(t *testing.T) {
	want := []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"}
	got := SupportedPlatforms()
	if len(got) != len(want) {
		t.Fatalf("SupportedPlatforms length = %d, want %d: %v", len(got), len(want), got)
	}
	for index, platform := range want {
		if got[index] != platform {
			t.Fatalf("SupportedPlatforms()[%d] = %q, want %q", index, got[index], platform)
		}
	}
}

func TestIsSupportedPlatform(t *testing.T) {
	if !IsSupportedPlatform("linux-amd64") {
		t.Fatal("linux-amd64 should be supported")
	}
	if IsSupportedPlatform("windows-amd64") {
		t.Fatal("windows-amd64 should not be supported in v1")
	}
}

func TestArtifactNameIncludesVersionAndPlatform(t *testing.T) {
	name, err := ArtifactName("1.0.0", "darwin-arm64")
	if err != nil {
		t.Fatalf("ArtifactName returned error: %v", err)
	}
	if name != "opentunnel-1.0.0-darwin-arm64" {
		t.Fatalf("ArtifactName = %q", name)
	}
}

func TestArtifactNameRejectsUnsupportedPlatform(t *testing.T) {
	if _, err := ArtifactName("1.0.0", "windows-amd64"); err == nil {
		t.Fatal("ArtifactName returned nil error for unsupported platform")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/artifact -count=1`

Expected: FAIL because `SupportedPlatforms`, `IsSupportedPlatform`, and `ArtifactName` are undefined.

- [ ] **Step 3: Implement supported platform helpers**

Update `internal/artifact/platform.go` to:

```go
package artifact

import (
	"fmt"
	"path/filepath"
	"runtime"
)

var supportedPlatforms = []string{
	"linux-amd64",
	"linux-arm64",
	"darwin-amd64",
	"darwin-arm64",
}

// PlatformKey returns the canonical artifact platform key for a Go OS and arch.
func PlatformKey(goos, goarch string) (string, error) {
	if goos == "" {
		return "", fmt.Errorf("goos is required")
	}
	if goarch == "" {
		return "", fmt.Errorf("goarch is required")
	}

	return goos + "-" + goarch, nil
}

// CurrentPlatformKey returns the artifact platform key for the current runtime.
func CurrentPlatformKey() (string, error) {
	return PlatformKey(runtime.GOOS, runtime.GOARCH)
}

// SupportedPlatforms returns the v1 artifact platforms in stable order.
func SupportedPlatforms() []string {
	platforms := make([]string, len(supportedPlatforms))
	copy(platforms, supportedPlatforms)
	return platforms
}

// IsSupportedPlatform reports whether platform is in the v1 artifact set.
func IsSupportedPlatform(platform string) bool {
	for _, supported := range supportedPlatforms {
		if platform == supported {
			return true
		}
	}
	return false
}

// ArtifactName returns the filename for a versioned supported platform artifact.
func ArtifactName(version string, platform string) (string, error) {
	if version == "" {
		return "", fmt.Errorf("version is required")
	}
	if !IsSupportedPlatform(platform) {
		return "", fmt.Errorf("unsupported platform %q", platform)
	}
	return "opentunnel-" + version + "-" + platform, nil
}

// ArtifactPath returns the filesystem path for a versioned supported platform artifact.
func ArtifactPath(dir string, version string, platform string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("artifact dir is required")
	}
	name, err := ArtifactName(version, platform)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/artifact -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/artifact/platform.go internal/artifact/platform_test.go
git commit -m "feat: define supported artifact platforms"
```

---

### Task 3: Render Multi-Platform Bootstrapper

**Files:**
- Modify: `internal/artifact/bootstrap.go`
- Modify: `internal/artifact/bootstrap_test.go`

- [ ] **Step 1: Replace bootstrap tests for multi-platform rendering**

Update `TestRenderBootstrapRendersPOSIXBootstrapScript` in `internal/artifact/bootstrap_test.go` to use checksums by platform:

```go
func TestRenderBootstrapRendersPOSIXBootstrapScript(t *testing.T) {
	script, err := RenderBootstrap(BootstrapConfig{
		RelayOrigin: "http://relay.example",
		Version:     "dev",
		Artifacts: []BootstrapArtifact{
			{PlatformKey: "linux-amd64", Checksum: "linux-amd64-checksum"},
			{PlatformKey: "linux-arm64", Checksum: "linux-arm64-checksum"},
			{PlatformKey: "darwin-amd64", Checksum: "darwin-amd64-checksum"},
			{PlatformKey: "darwin-arm64", Checksum: "darwin-arm64-checksum"},
		},
	})
	if err != nil {
		t.Fatalf("RenderBootstrap returned error: %v", err)
	}

	wants := []string{
		"relay_origin='http://relay.example'",
		"version='dev'",
		"os_name=$(uname -s)",
		"arch_name=$(uname -m)",
		"platform=linux-amd64",
		"platform=linux-arm64",
		"platform=darwin-amd64",
		"platform=darwin-arm64",
		"linux-amd64-checksum",
		"darwin-arm64-checksum",
		"/cli/bin/opentunnel-${version}-${platform}",
		"umask 077",
		"${TMPDIR:-/tmp}",
		"id -u",
		"cache_base=\"${TMPDIR:-/tmp}/opentunnel-cli-${uid}\"",
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
```

Update missing-field tests so the valid config uses `Artifacts` instead of `PlatformKey` and `Checksum`:

```go
func testBootstrapArtifacts() []BootstrapArtifact {
	return []BootstrapArtifact{
		{PlatformKey: "linux-amd64", Checksum: "expected-checksum"},
		{PlatformKey: "linux-arm64", Checksum: "linux-arm64-checksum"},
		{PlatformKey: "darwin-amd64", Checksum: "darwin-amd64-checksum"},
		{PlatformKey: "darwin-arm64", Checksum: "darwin-arm64-checksum"},
	}
}
```

Add this unsupported-platform script test:

```go
func TestRenderBootstrapFailsForUnsupportedRuntimePlatform(t *testing.T) {
	script := renderBootstrapForTest(t)
	path := filepath.Join(t.TempDir(), "bootstrap.sh")
	writeExecutable(t, path, script)

	toolsDir := t.TempDir()
	writeExecutable(t, filepath.Join(toolsDir, "uname"), `#!/bin/sh
if [ "$1" = "-s" ]; then
  printf 'SunOS\n'
else
  printf 'sparc\n'
fi
`)
	writeExecutable(t, filepath.Join(toolsDir, "id"), `#!/bin/sh
printf '1000\n'
`)

	cmd := exec.Command("/bin/sh", path)
	cmd.Env = []string{"PATH=" + toolsDir, "TMPDIR=" + t.TempDir()}
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("bootstrap exited successfully; output:\n%s", output)
	}
	if !strings.Contains(string(output), "unsupported platform") {
		t.Fatalf("bootstrap output missing unsupported platform message:\n%s", output)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/artifact -count=1`

Expected: FAIL because `BootstrapArtifact` and `BootstrapConfig.Artifacts` are undefined and old fields are still present.

- [ ] **Step 3: Implement multi-platform bootstrap config and rendering**

Update `internal/artifact/bootstrap.go` around the config types:

```go
// BootstrapArtifact contains one downloadable CLI artifact coordinate.
type BootstrapArtifact struct {
	PlatformKey string
	Checksum    string
}

// BootstrapConfig contains immutable artifact coordinates for a CLI bootstrap script.
type BootstrapConfig struct {
	RelayOrigin string
	Version     string
	Artifacts   []BootstrapArtifact
}
```

In `RenderBootstrap`, replace the single `PlatformKey` / `Checksum` validation with:

```go
	if len(cfg.Artifacts) == 0 {
		return "", fmt.Errorf("artifacts are required")
	}
	checksumCases, err := renderChecksumCases(cfg.Artifacts)
	if err != nil {
		return "", err
	}
```

Render the platform-detecting script with this core block before cache setup:

```sh
os_name=$(uname -s)
arch_name=$(uname -m)
case "${os_name}:${arch_name}" in
  Linux:x86_64|Linux:amd64) platform=linux-amd64 ;;
  Linux:aarch64|Linux:arm64) platform=linux-arm64 ;;
  Darwin:x86_64|Darwin:amd64) platform=darwin-amd64 ;;
  Darwin:arm64|Darwin:aarch64) platform=darwin-arm64 ;;
  *)
    printf 'opentunnel: unsupported platform %s/%s\n' "$os_name" "$arch_name" >&2
    exit 1
    ;;
esac
case "$platform" in
%s
  *)
    printf 'opentunnel: unsupported artifact platform %s\n' "$platform" >&2
    exit 1
    ;;
esac
binary_path="/cli/bin/opentunnel-${version}-${platform}"
checksum_path="${binary_path}.sha256"
binary_url="${relay_origin}${binary_path}"
checksum_url="${relay_origin}${checksum_path}"
```

Add this helper to render checksum cases in stable supported-platform order:

```go
func renderChecksumCases(artifacts []BootstrapArtifact) (string, error) {
	checksums := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		if !IsSupportedPlatform(artifact.PlatformKey) {
			return "", fmt.Errorf("unsupported platform %q", artifact.PlatformKey)
		}
		if artifact.Checksum == "" {
			return "", fmt.Errorf("checksum is required for %s", artifact.PlatformKey)
		}
		checksums[artifact.PlatformKey] = artifact.Checksum
	}

	var builder strings.Builder
	for _, platform := range SupportedPlatforms() {
		checksum, ok := checksums[platform]
		if !ok {
			return "", fmt.Errorf("checksum is required for %s", platform)
		}
		fmt.Fprintf(&builder, "  %s) expected_checksum=%s ;;\n", platform, shellQuote(checksum))
	}
	return builder.String(), nil
}
```

- [ ] **Step 4: Update test helper config**

Update `renderBootstrapForTest` in `internal/artifact/bootstrap_test.go` so it calls:

```go
script, err := RenderBootstrap(BootstrapConfig{
	RelayOrigin: "http://relay.example",
	Version:     "dev",
	Artifacts:   testBootstrapArtifacts(),
})
```

Update tests that construct `BootstrapConfig` directly to use `Artifacts: testBootstrapArtifacts()` unless they intentionally verify missing artifacts.

- [ ] **Step 5: Run tests to verify pass**

Run: `go test ./internal/artifact -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/artifact/bootstrap.go internal/artifact/bootstrap_test.go
git commit -m "feat: render multi-platform cli bootstrap"
```

---

### Task 4: Serve Artifact Directories From Relay

**Files:**
- Modify: `internal/relay/server.go`
- Modify: `internal/relay/server_test.go`

- [ ] **Step 1: Replace single-artifact relay tests**

In `internal/relay/server_test.go`, replace `TestNewServerWithOptionsServesCLIArtifacts` with:

```go
func TestNewServerWithOptionsServesMultiPlatformCLIArtifacts(t *testing.T) {
	artifactDir := writeTestArtifactDir(t, "4.5.6")
	server, err := NewServerWithOptions(ServerOptions{
		PublicURL:   "https://relay.example.com",
		Version:     "4.5.6",
		ArtifactDir: artifactDir,
	})
	if err != nil {
		t.Fatalf("NewServerWithOptions() error = %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	bootstrapResponse, bootstrapBody := getRelayPath(t, httpServer.URL, "/cli")
	defer bootstrapResponse.Body.Close()
	if bootstrapResponse.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap status mismatch: got %d want %d", bootstrapResponse.StatusCode, http.StatusOK)
	}
	if !strings.Contains(string(bootstrapBody), "version='4.5.6'") {
		t.Fatalf("bootstrap missing configured version in:\n%s", bootstrapBody)
	}
	if !strings.Contains(string(bootstrapBody), "platform=linux-amd64") {
		t.Fatalf("bootstrap missing platform detection in:\n%s", bootstrapBody)
	}

	for _, platform := range []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"} {
		body := []byte("binary " + platform)
		path := "/cli/bin/opentunnel-4.5.6-" + platform
		binaryResponse, binaryBody := getRelayPath(t, httpServer.URL, path)
		defer binaryResponse.Body.Close()
		if binaryResponse.StatusCode != http.StatusOK {
			t.Fatalf("%s status mismatch: got %d want %d", platform, binaryResponse.StatusCode, http.StatusOK)
		}
		if !bytes.Equal(binaryBody, body) {
			t.Fatalf("%s body mismatch: got %q want %q", platform, binaryBody, body)
		}

		checksumResponse, checksumBody := getRelayPath(t, httpServer.URL, path+".sha256")
		defer checksumResponse.Body.Close()
		if checksumResponse.StatusCode != http.StatusOK {
			t.Fatalf("%s checksum status mismatch: got %d want %d", platform, checksumResponse.StatusCode, http.StatusOK)
		}
		if got := strings.TrimSpace(string(checksumBody)); got != testSHA256(body) {
			t.Fatalf("%s checksum mismatch: got %q want %q", platform, got, testSHA256(body))
		}
	}
}
```

Add helper:

```go
func writeTestArtifactDir(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	for _, platform := range []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"} {
		path := filepath.Join(dir, "opentunnel-"+version+"-"+platform)
		if err := os.WriteFile(path, []byte("binary "+platform), 0o600); err != nil {
			t.Fatalf("write artifact %s: %v", platform, err)
		}
	}
	return dir
}
```

- [ ] **Step 2: Update validation tests**

In `TestNewServerWithOptionsRejectsInvalidCLIArtifactOptions`, replace `ArtifactPath` / `PlatformKey` cases with:

```go
{
	name: "public url without artifact dir",
	options: ServerOptions{
		PublicURL: "https://relay.example.com",
		Version:   "1.2.3",
	},
},
{
	name: "public url without version",
	options: ServerOptions{
		PublicURL:   "https://relay.example.com",
		ArtifactDir: artifactDir,
	},
},
{
	name: "missing artifact dir",
	options: ServerOptions{
		PublicURL:   "https://relay.example.com",
		Version:     "1.2.3",
		ArtifactDir: filepath.Join(t.TempDir(), "missing-artifacts"),
	},
},
{
	name: "incomplete artifact dir",
	options: ServerOptions{
		PublicURL:   "https://relay.example.com",
		Version:     "1.2.3",
		ArtifactDir: incompleteArtifactDir,
	},
},
```

Set `artifactDir := writeTestArtifactDir(t, "1.2.3")` and `incompleteArtifactDir := t.TempDir()` before the table, and write only `opentunnel-1.2.3-linux-amd64` into `incompleteArtifactDir`.

- [ ] **Step 3: Run tests to verify failure**

Run: `go test ./internal/relay -count=1`

Expected: FAIL because `ServerOptions.ArtifactDir` is undefined and relay still serves one artifact.

- [ ] **Step 4: Implement relay artifact directory model**

Change types in `internal/relay/server.go`:

```go
type CLIArtifact struct {
	PlatformKey string
	BinaryPath  string
	Checksum    string
}

type CLIArtifacts struct {
	RelayOrigin string
	Version     string
	Artifacts   map[string]CLIArtifact
}

type ServerOptions struct {
	PublicURL   string
	Version     string
	ArtifactDir string
}
```

Replace `NewServerWithOptions` artifact setup with:

```go
	if options.PublicURL == "" && options.Version == "" && options.ArtifactDir == "" {
		return server, nil
	}
	artifacts, err := loadCLIArtifacts(options.PublicURL, options.Version, options.ArtifactDir)
	if err != nil {
		return nil, fmt.Errorf("validate cli artifacts: %w", err)
	}
	server.cliArtifacts = artifacts
	return server, nil
```

Add helper:

```go
func loadCLIArtifacts(relayOrigin string, version string, artifactDir string) (*CLIArtifacts, error) {
	if relayOrigin == "" {
		return nil, fmt.Errorf("public url is required")
	}
	if version == "" {
		return nil, fmt.Errorf("version is required")
	}
	if artifactDir == "" {
		return nil, fmt.Errorf("artifact dir is required")
	}

	artifacts := make(map[string]CLIArtifact)
	bootstrapArtifacts := make([]artifact.BootstrapArtifact, 0, len(artifact.SupportedPlatforms()))
	for _, platform := range artifact.SupportedPlatforms() {
		path, err := artifact.ArtifactPath(artifactDir, version, platform)
		if err != nil {
			return nil, err
		}
		checksum, err := artifact.SHA256File(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", platform, err)
		}
		artifacts[platform] = CLIArtifact{PlatformKey: platform, BinaryPath: path, Checksum: checksum}
		bootstrapArtifacts = append(bootstrapArtifacts, artifact.BootstrapArtifact{PlatformKey: platform, Checksum: checksum})
	}
	if _, err := artifact.RenderBootstrap(artifact.BootstrapConfig{RelayOrigin: relayOrigin, Version: version, Artifacts: bootstrapArtifacts}); err != nil {
		return nil, err
	}
	return &CLIArtifacts{RelayOrigin: relayOrigin, Version: version, Artifacts: artifacts}, nil
}
```

Replace path matching in `handleCLIArtifact` with explicit artifact lookup:

```go
	if r.Method != http.MethodGet || (r.URL.Path != "/cli" && !strings.HasPrefix(r.URL.Path, "/cli/bin/")) {
		return false
	}
```

Import `strings` and replace binary/checksum serving with helpers that parse paths like `/cli/bin/opentunnel-4.5.6-linux-amd64` and `/cli/bin/opentunnel-4.5.6-linux-amd64.sha256`.

Add:

```go
func (s *Server) artifactForPath(path string) (CLIArtifact, bool, bool) {
	if s.cliArtifacts == nil {
		return CLIArtifact{}, false, false
	}
	prefix := "/cli/bin/opentunnel-" + s.cliArtifacts.Version + "-"
	name, ok := strings.CutPrefix(path, prefix)
	if !ok {
		return CLIArtifact{}, false, false
	}
	checksum := false
	if trimmed, ok := strings.CutSuffix(name, ".sha256"); ok {
		name = trimmed
		checksum = true
	}
	artifact, ok := s.cliArtifacts.Artifacts[name]
	return artifact, checksum, ok
}
```

Update `serveBootstrap` to build `[]artifact.BootstrapArtifact` from `artifact.SupportedPlatforms()` and stored checksums.

- [ ] **Step 5: Run relay tests**

Run: `go test ./internal/relay -count=1`

Expected: PASS.

- [ ] **Step 6: Run package tests**

Run: `go test ./internal/artifact ./internal/relay -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/relay/server.go internal/relay/server_test.go
git commit -m "feat: serve cli artifacts from directory"
```

---

### Task 5: Update Relay CLI Arguments

**Files:**
- Modify: `cmd/opentunnel/main.go`
- Modify: `cmd/opentunnel/main_test.go`

- [ ] **Step 1: Add failing CLI parse tests**

In `cmd/opentunnel/main_test.go`, update or add these tests:

```go
func TestParseRelayArgsRequiresPublicURL(t *testing.T) {
	_, err := parseArgs([]string{"relay"})
	if err == nil {
		t.Fatal("parseArgs returned nil error")
	}
	if !strings.Contains(err.Error(), "public url") {
		t.Fatalf("error = %q, want public url", err)
	}
}

func TestParseRelayArgsDefaultsArtifactDirAndVersion(t *testing.T) {
	parsed, err := parseArgs([]string{"relay", "--public-url", "https://relay.example.com"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}
	cmd, ok := parsed.(relayCommand)
	if !ok {
		t.Fatalf("parseArgs returned %T", parsed)
	}
	if cmd.artifactDir != "/opentunnel-artifacts" {
		t.Fatalf("artifactDir = %q", cmd.artifactDir)
	}
	if cmd.version != "dev" {
		t.Fatalf("version = %q", cmd.version)
	}
}

func TestParseRelayArgsRejectsArtifactPath(t *testing.T) {
	_, err := parseArgs([]string{"relay", "--public-url", "https://relay.example.com", "--artifact-path", "/tmp/opentunnel"})
	if err == nil {
		t.Fatal("parseArgs returned nil error")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./cmd/opentunnel -count=1`

Expected: FAIL because `artifactDir` is not in `relayCommand` and `relay` currently allows missing `--public-url`.

- [ ] **Step 3: Implement CLI changes**

Update imports in `cmd/opentunnel/main.go` to include:

```go
	"opentunnel/internal/buildinfo"
```

Change `relayCommand`:

```go
type relayCommand struct {
	listen      string
	publicURL   string
	artifactDir string
	version     string
}
```

Update `parseRelayArgs`:

```go
func parseRelayArgs(args []string) (relayCommand, error) {
	flags := flag.NewFlagSet("relay", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	cmd := relayCommand{listen: ":8080", artifactDir: "/opentunnel-artifacts", version: buildinfo.Version}
	flags.StringVar(&cmd.listen, "listen", ":8080", "HTTP listen address")
	flags.StringVar(&cmd.publicURL, "public-url", "", "public relay URL")
	flags.StringVar(&cmd.artifactDir, "artifact-dir", "/opentunnel-artifacts", "CLI artifact directory")
	flags.StringVar(&cmd.version, "version", buildinfo.Version, "CLI artifact version")
	if err := flags.Parse(args); err != nil {
		return relayCommand{}, err
	}
	if flags.NArg() != 0 {
		return relayCommand{}, fmt.Errorf("relay got unexpected argument %q", flags.Arg(0))
	}
	if cmd.publicURL == "" {
		return relayCommand{}, errors.New("relay requires --public-url")
	}
	if err := validatePublicURL(cmd.publicURL); err != nil {
		return relayCommand{}, err
	}
	return cmd, nil
}
```

Update `relayCommand.run` to pass `ArtifactDir`:

```go
	options := relay.ServerOptions{PublicURL: cmd.publicURL, ArtifactDir: cmd.artifactDir, Version: cmd.version}
	relayServer, err := relay.NewServerWithOptions(options)
	if err != nil {
		fmt.Fprintf(stderr, "start relay: %v\n", err)
		return 1
	}
```

Remove `buildRelayServerOptions` if no tests use it after updates.

- [ ] **Step 4: Run CLI tests**

Run: `go test ./cmd/opentunnel -count=1`

Expected: PASS.

- [ ] **Step 5: Run affected tests**

Run: `go test ./cmd/opentunnel ./internal/relay -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/opentunnel/main.go cmd/opentunnel/main_test.go
git commit -m "feat: use artifact directory relay flag"
```

---

### Task 6: Update Docker Image To Include All Artifacts

**Files:**
- Modify: `deploy/docker/Dockerfile`
- Modify: `deploy/docker/README.md`

- [ ] **Step 1: Update Dockerfile**

Replace `deploy/docker/Dockerfile` with:

```dockerfile
# syntax=docker/dockerfile:1

FROM golang:1.23 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN version="$(cat VERSION)" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X opentunnel/internal/buildinfo.Version=${version}" -o /out/opentunnel ./cmd/opentunnel && \
    mkdir -p /out/opentunnel-artifacts && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X opentunnel/internal/buildinfo.Version=${version}" -o /out/opentunnel-artifacts/opentunnel-${version}-linux-amd64 ./cmd/opentunnel && \
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-X opentunnel/internal/buildinfo.Version=${version}" -o /out/opentunnel-artifacts/opentunnel-${version}-linux-arm64 ./cmd/opentunnel && \
    CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-X opentunnel/internal/buildinfo.Version=${version}" -o /out/opentunnel-artifacts/opentunnel-${version}-darwin-amd64 ./cmd/opentunnel && \
    CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-X opentunnel/internal/buildinfo.Version=${version}" -o /out/opentunnel-artifacts/opentunnel-${version}-darwin-arm64 ./cmd/opentunnel

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/opentunnel /opentunnel
COPY --from=builder /out/opentunnel-artifacts /opentunnel-artifacts

USER nonroot:nonroot
EXPOSE 8080

ENTRYPOINT ["/opentunnel"]
CMD ["relay", "--listen", ":8080"]
```

Rationale: `--public-url` remains required, so the default `CMD` is intentionally incomplete for a real running container. Operators pass `relay --public-url ...` as documented.

- [ ] **Step 2: Update Docker README**

In `deploy/docker/README.md`, replace the run examples with:

```markdown
## Run

For local testing:

```bash
docker run --rm -p 8080:8080 opentunnel-relay:dev \
  relay --public-url http://localhost:8080
```

For public deployment, set `--public-url` to the HTTPS origin users will fetch:

```bash
docker run -p 8080:8080 opentunnel-relay:dev \
  relay --public-url https://relay.example.com
```

The image includes `/opentunnel-artifacts` with the supported Linux and macOS `amd64`/`arm64` temporary CLI binaries. The relay selects the correct binary at `/cli` runtime.
```

- [ ] **Step 3: Build image**

Run: `docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .`

Expected: PASS and image builds successfully.

- [ ] **Step 4: Commit**

```bash
git add deploy/docker/Dockerfile deploy/docker/README.md
git commit -m "feat: build multi-arch relay container artifacts"
```

---

### Task 7: Update Systemd And Public Documentation

**Files:**
- Modify: `deploy/systemd/opentunnel-relay.service`
- Modify: `deploy/systemd/opentunnel-relay.env.example`
- Modify: `deploy/systemd/README.md`
- Modify: `docs/public-v1/self-hosting.md`
- Modify: `docs/public-v1/operations.md`
- Modify: `README.md`

- [ ] **Step 1: Update systemd files**

Change `deploy/systemd/opentunnel-relay.service` line 11 to:

```ini
ExecStart=/usr/local/bin/opentunnel relay --listen ${OPENTUNNEL_LISTEN} --public-url ${OPENTUNNEL_PUBLIC_URL} --artifact-dir ${OPENTUNNEL_ARTIFACT_DIR} --version ${OPENTUNNEL_VERSION}
```

Change `deploy/systemd/opentunnel-relay.env.example` to:

```text
OPENTUNNEL_LISTEN=:8080
OPENTUNNEL_PUBLIC_URL=https://relay.example.com
OPENTUNNEL_ARTIFACT_DIR=/opt/opentunnel/artifacts
OPENTUNNEL_VERSION=dev
```

- [ ] **Step 2: Update self-hosting docs**

In `docs/public-v1/self-hosting.md`, replace the build/run sections with Docker-first wording:

```markdown
## Build The Relay Image

From the repository root:

```bash
docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .
```

The image includes the relay binary and all supported temporary CLI artifacts for Linux and macOS `amd64`/`arm64`.

## Run A Relay Container

For local testing:

```bash
docker run --rm -p 8080:8080 opentunnel-relay:dev \
  relay --public-url http://127.0.0.1:8080
```

For a public relay:

```bash
docker run -p 8080:8080 opentunnel-relay:dev \
  relay --public-url https://relay.example.com
```
```

Replace artifact flag bullets with:

```markdown
- `--artifact-dir` points to the directory containing all supported files named like `opentunnel-1.0.0-linux-amd64`. The container default is `/opentunnel-artifacts`.
- `--version` defaults to the build version from `VERSION`; release versions do not use a leading `v`.
- `--public-url` is required and is embedded into the bootstrapper.
```

- [ ] **Step 3: Update operations docs**

In `docs/public-v1/operations.md`, replace manual release steps 3-6 with:

```markdown
3. Build the relay image with `docker build -f deploy/docker/Dockerfile -t opentunnel-relay:1.0.0 .`.
4. Start the relay container with `relay --public-url https://relay.example.com`.
5. Verify `/cli`.
6. Verify each artifact and checksum path: `linux-amd64`, `linux-arm64`, `darwin-amd64`, and `darwin-arm64`.
```

- [ ] **Step 4: Update README quickstart**

In `README.md`, replace `--artifact-path` examples with Docker flow and `--public-url` only:

```markdown
Build and run a local self-contained relay image:

```bash
docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .
docker run --rm -p 8080:8080 opentunnel-relay:dev \
  relay --public-url http://127.0.0.1:8080
```
```

- [ ] **Step 5: Search docs for removed flag**

Run: `rg -- '--artifact-path|OPENTUNNEL_ARTIFACT_PATH|v1\.0\.0|--artifact-dir' README.md docs deploy`

Expected: no `--artifact-path` or `OPENTUNNEL_ARTIFACT_PATH` matches. `--artifact-dir` matches should be in updated docs. Any version examples should not use a leading `v`.

- [ ] **Step 6: Commit**

```bash
git add README.md docs/public-v1/self-hosting.md docs/public-v1/operations.md deploy/systemd/opentunnel-relay.service deploy/systemd/opentunnel-relay.env.example deploy/systemd/README.md
git commit -m "docs: document multi-arch relay artifacts"
```

---

### Task 8: Update CI Artifact Builds

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Update CI matrix**

Change the `build-artifacts` matrix to exactly:

```yaml
          - goos: linux
            goarch: amd64
            platform: linux-amd64
          - goos: linux
            goarch: arm64
            platform: linux-arm64
          - goos: darwin
            goarch: amd64
            platform: darwin-amd64
          - goos: darwin
            goarch: arm64
            platform: darwin-arm64
```

Change the build step to read `VERSION` and write versioned artifact names:

```yaml
      - name: Build artifact
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: '0'
        run: |
          version="$(cat VERSION)"
          mkdir -p dist
          go build -ldflags "-X opentunnel/internal/buildinfo.Version=${version}" -o dist/opentunnel-${version}-${{ matrix.platform }} ./cmd/opentunnel

      - name: Upload artifact
        uses: actions/upload-artifact@v7.0.1
        with:
          name: opentunnel-${{ matrix.platform }}
          path: dist/opentunnel-*-${{ matrix.platform }}
```

- [ ] **Step 2: Validate workflow references**

Run: `rg -- 'linux-armv7|opentunnel-linux-amd64|opentunnel-darwin-arm64|VERSION|matrix.platform' .github/workflows/ci.yml`

Expected: no `linux-armv7`; `VERSION` and `matrix.platform` are present.

- [ ] **Step 3: Run local cross-build check**

Run:

```bash
version="$(cat VERSION)" && mkdir -p /tmp/opencode/opentunnel-ci-artifacts && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X opentunnel/internal/buildinfo.Version=${version}" -o /tmp/opencode/opentunnel-ci-artifacts/opentunnel-${version}-linux-amd64 ./cmd/opentunnel && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-X opentunnel/internal/buildinfo.Version=${version}" -o /tmp/opencode/opentunnel-ci-artifacts/opentunnel-${version}-linux-arm64 ./cmd/opentunnel && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-X opentunnel/internal/buildinfo.Version=${version}" -o /tmp/opencode/opentunnel-ci-artifacts/opentunnel-${version}-darwin-amd64 ./cmd/opentunnel && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-X opentunnel/internal/buildinfo.Version=${version}" -o /tmp/opencode/opentunnel-ci-artifacts/opentunnel-${version}-darwin-arm64 ./cmd/opentunnel && rm -rf /tmp/opencode/opentunnel-ci-artifacts
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: build supported multi-arch artifacts"
```

---

### Task 9: Final Verification

**Files:**
- All files modified in previous tasks.

- [ ] **Step 1: Run Go tests**

Run: `go test ./... -count=1`

Expected: PASS.

- [ ] **Step 2: Run Go vet**

Run: `go vet ./...`

Expected: PASS with no output.

- [ ] **Step 3: Run module tidy check**

Run: `go mod tidy -diff`

Expected: PASS with no diff output.

- [ ] **Step 4: Run race tests**

Run: `go test -race ./... -count=1`

Expected: PASS.

- [ ] **Step 5: Build Docker image**

Run: `docker build -f deploy/docker/Dockerfile -t opentunnel-relay:dev .`

Expected: PASS.

- [ ] **Step 6: Smoke test `/cli` from container**

Run:

```bash
container_id=$(docker run -d --rm -p 18080:8080 opentunnel-relay:dev relay --public-url http://127.0.0.1:18080) && sleep 1 && curl -fsSL http://127.0.0.1:18080/cli >/tmp/opentunnel-cli.sh && docker stop "$container_id" && rm -f /tmp/opentunnel-cli.sh
```

Expected: PASS, `curl` exits 0, and `docker stop` exits 0.

- [ ] **Step 7: Verify removed single-artifact flag**

Run: `rg -- '--artifact-path|OPENTUNNEL_ARTIFACT_PATH' README.md docs deploy cmd internal .github`

Expected: no matches.

- [ ] **Step 8: Verify unprefixed release examples**

Run: `rg -- 'v1\.0\.0|version v1|VERSION=v1' README.md docs deploy .github`

Expected: no matches.

- [ ] **Step 9: Commit final fixes if any**

If verification required fixes, stage only the concrete files changed by those fixes. For example, if Docker documentation needed one correction, run:

```bash
git add deploy/docker/README.md
git commit -m "fix: complete multi-arch relay artifact verification"
```

If no fixes were required, do not create an empty commit.
