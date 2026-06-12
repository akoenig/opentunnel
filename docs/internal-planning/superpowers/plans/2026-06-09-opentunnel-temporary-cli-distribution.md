# OpenTunnel Temporary CLI Distribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Milestone 4 from `docs/superpowers/specs/2026-05-22-opentunnel-v1-milestone-design.md`: `curl -fsSL <relay>/cli | sh -s -- create` works against a self-contained relay that serves a compatible temporary binary with private temp caching and checksum validation.

**Architecture:** Add a small `internal/artifact` package for platform keys, checksums, cache paths, and bootstrap script rendering. Extend `relay.Server` with optional artifact-serving configuration for `/cli`, `/cli/bin/...`, and checksum endpoints while keeping `/tunnel` opaque. Update CLI relay to accept an artifact path/version, update create to infer relay origin from `OPENTUNNEL_RELAY_ORIGIN`, and update generated prompts to use the public curl command shape. No strong supply-chain claims, signed manifests, package-manager install, accounts, dashboard, `/cli` public hardening beyond same-origin checksum corruption detection, or system installation is added.

**Tech Stack:** Go, existing relay/tunnel packages, standard `crypto/sha256`, `runtime`, `os`, `path/filepath`, `text/template` or direct string builder, POSIX `sh` bootstrapper, Go tests and shell-based manual verification.

---

## File Structure

- Create: `internal/artifact/platform.go`, `internal/artifact/platform_test.go`
  - Normalize GOOS/GOARCH to artifact path components and cache key material.
- Create: `internal/artifact/bootstrap.go`, `internal/artifact/bootstrap_test.go`
  - Render deterministic POSIX `sh` bootstrapper with relay origin, version, checksum URL, binary URL, private cache path, checksum verification, chmod, and exec.
- Create: `internal/artifact/checksum.go`, `internal/artifact/checksum_test.go`
  - SHA-256 helpers for artifact files.
- Modify: `internal/relay/server.go`, `internal/relay/server_test.go`
  - Add optional artifact serving while preserving `/tunnel` behavior.
- Modify: `cmd/opentunnel/main.go`, `cmd/opentunnel/main_test.go`
  - Add relay artifact flags, `OPENTUNNEL_RELAY_ORIGIN` inference for create, and public prompt command shape.

## Task 1: Add Artifact Platform And Checksum Helpers

**Files:**
- Create: `internal/artifact/platform.go`
- Create: `internal/artifact/platform_test.go`
- Create: `internal/artifact/checksum.go`
- Create: `internal/artifact/checksum_test.go`

- [ ] **Step 1: Write failing tests**

Test `PlatformKey("linux", "amd64") == "linux-amd64"`, unsupported empty values fail, and `SHA256File` returns the expected lowercase hex digest for a temp file containing `hello`.

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/artifact -count=1
```

Expected: FAIL because package implementation is missing.

- [ ] **Step 3: Implement helpers**

Implement exported `PlatformKey(goos, goarch string) (string, error)`, `CurrentPlatformKey() (string, error)`, and `SHA256File(path string) (string, error)` with doc comments.

- [ ] **Step 4: Verify green**

Run:

```bash
gofmt -w internal/artifact/*.go
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/artifact/platform.go internal/artifact/platform_test.go internal/artifact/checksum.go internal/artifact/checksum_test.go
```

## Task 2: Render POSIX `/cli` Bootstrapper

**Files:**
- Create: `internal/artifact/bootstrap.go`
- Create: `internal/artifact/bootstrap_test.go`

- [ ] **Step 1: Write failing tests**

Test `RenderBootstrap` output contains `relay_origin='http://relay.example'`, immutable binary URL, checksum URL, `umask 077`, `mktemp`, private cache directory under `${TMPDIR:-/tmp}`, `sha256sum` or `shasum -a 256`, `chmod 700`, and `exec "$bin" "$@"`. Test it rejects missing relay origin, version, or checksum.

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/artifact -run TestRenderBootstrap -count=1
```

Expected: FAIL because `RenderBootstrap` is missing.

- [ ] **Step 3: Implement bootstrap rendering**

Implement exported `BootstrapConfig` and `RenderBootstrap(cfg BootstrapConfig) (string, error)`. The script must be POSIX `sh`, avoid `eval`, quote variables, create private cache dirs with `umask 077`, download to temp file, verify checksum when `sha256sum` or `shasum` is available, chmod, atomic move, set `OPENTUNNEL_RELAY_ORIGIN`, then `exec "$bin" "$@"`. Same-origin checksums are corruption detection only; do not claim supply-chain security in script comments.

- [ ] **Step 4: Verify green**

Run:

```bash
gofmt -w internal/artifact/*.go
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/artifact/bootstrap.go internal/artifact/bootstrap_test.go
```

## Task 3: Serve `/cli` Artifacts From Relay

**Files:**
- Modify: `internal/relay/server.go`
- Modify: `internal/relay/server_test.go`

- [ ] **Step 1: Write failing relay artifact tests**

Add tests for `NewServerWithArtifacts` or `ServerOptions` proving:
- `GET /cli` returns shell script containing configured public URL.
- `GET /cli/bin/opentunnel-<version>-<platform>` returns the configured artifact bytes.
- `GET /cli/bin/opentunnel-<version>-<platform>.sha256` returns matching SHA-256 hex.
- `/tunnel` tests still pass and artifact serving does not inspect tunnel payloads.

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/relay -run 'TestCLIBootstrap|TestCLIArtifact' -count=1
```

Expected: FAIL because artifact server config is missing.

- [ ] **Step 3: Implement relay artifact serving**

Add `ServerOptions{PublicURL, Version, ArtifactPath, PlatformKey}` and `NewServerWithOptions(options ServerOptions) (*Server, error)`. Keep `NewServer()` for tests and no-artifact relay. Handler routes `/cli`, `/cli/bin/...`, and `/cli/bin/...sha256`; `/tunnel` remains unchanged. If artifact options are incomplete, `/cli` returns clear 404 or 500 instead of serving wrong origin.

- [ ] **Step 4: Verify green**

Run:

```bash
gofmt -w internal/relay/*.go
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/relay/server.go internal/relay/server_test.go
```

## Task 4: Infer Relay Origin In `create` And Generate Public Prompt

**Files:**
- Modify: `cmd/opentunnel/main.go`
- Modify: `cmd/opentunnel/main_test.go`

- [ ] **Step 1: Write failing CLI tests**

Add tests proving:
- `create` can parse with no `--relay` when `OPENTUNNEL_RELAY_ORIGIN` is set.
- `create` still accepts developer `--relay` as override for local testing.
- generated prompt contains `curl -fsSL <relay>/cli | sh -s -- exec --invite '<invite>' -- '<COMMAND>'`.
- prompt does not require user-facing `--relay`.

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./cmd/opentunnel -run 'TestParseCreate|TestCreatePrompt' -count=1
```

Expected: FAIL because env relay inference/public prompt is missing.

- [ ] **Step 3: Implement CLI changes**

Add relay command flags `--artifact-path` and `--version` for self-hosted local artifact serving. `create` should use `--relay` if provided, otherwise `OPENTUNNEL_RELAY_ORIGIN`, otherwise error. Generated prompt should use public curl command shape with the inferred relay origin and no `--relay` flag.

- [ ] **Step 4: Verify green**

Run:

```bash
gofmt -w cmd/opentunnel/*.go
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/opentunnel/main.go cmd/opentunnel/main_test.go
```

## Task 5: Final `/cli` Vertical Slice Verification

**Files:**
- Verify all files modified by this plan.

- [ ] **Step 1: Run full verification**

Run:

```bash
gofmt -w internal/artifact/*.go internal/relay/*.go cmd/opentunnel/*.go
```

Expected: all pass.

- [ ] **Step 2: Run manual `/cli` flow**

Build a temp `opentunnel` binary, start relay with `--public-url http://127.0.0.1:<port> --artifact-path <temp-binary> --version dev`, run `curl -fsSL http://127.0.0.1:<port>/cli | sh -s -- create`, extract invite, then run the generated curl exec command for `printf hello`. Assert stdout `hello`, exit 0, and repeated exec uses cached binary path if observable from logs or filesystem.

- [ ] **Step 3: Commit cleanup if needed**

If formatting/tidy/doc changes exist:

```bash
git add internal/artifact internal/relay cmd/opentunnel docs/superpowers/plans/2026-06-09-opentunnel-temporary-cli-distribution.md go.mod go.sum
```

Do not create an empty commit.

## Self-Review Checklist

- Milestone 4 gate is covered: `/cli` works, generated prompt uses compatible artifact, temp cache is private and checksum-validated, no user-facing relay flag is required for public create/exec UX, same-origin checksum is treated as corruption detection only.
- Out-of-scope items remain absent: signatures, external verification tools, accounts, install-to-system, package managers, dashboard.
- Final verification includes normal tests, race tests, vet, module tidy, build, and a manual curl-piped `/cli` flow.
