# OpenTunnel Root README And CI Artifacts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a root GitHub README and ensure every push/PR CI run builds Linux x86_64 and Apple Silicon binaries.

**Architecture:** Keep runtime code unchanged. Add a root `README.md` that links to existing public v1 docs, and extend `.github/workflows/ci.yml` with a cross-build artifact job using Go's `GOOS`/`GOARCH` environment variables.

**Tech Stack:** Markdown, GitHub Actions, Go cross-compilation.

---

## File Structure

- Create `README.md`: repository landing page.
- Modify `.github/workflows/ci.yml`: add cross-build artifact generation for Linux x86_64 and Apple Silicon.
- Commit after each task.

## Task 1: Root README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Create README**

Create `README.md` with:

```markdown
# OpenTunnel

OpenTunnel is an ephemeral, relay-routed, end-to-end encrypted remote command tunnel for AI agents. A host starts one foreground session, copies the generated prompt into an agent, and the agent can run one-off commands through an opaque relay without SSH, inbound firewall rules, accounts, or persistent relay state.

## What It Is

OpenTunnel v1 is built around one temporary host process, one client, one command at a time, and a temporary CLI downloaded from the relay's `/cli` endpoint. The relay routes encrypted packets and stores only active in-memory connection state.

## Status

This repository contains the public v1 self-hosted implementation and release-operations artifacts. It is designed for operators who run their own relay and serve a compatible temporary `opentunnel` binary from that relay.

## Quick Start

Build the CLI:

```bash
go build -o opentunnel ./cmd/opentunnel
```

Run a local relay:

```bash
./opentunnel relay \
  --listen 127.0.0.1:8080 \
  --public-url http://127.0.0.1:8080 \
  --artifact-path ./opentunnel \
  --version dev
```

Start a host session through the public `/cli` path:

```bash
curl -fsSL http://127.0.0.1:8080/cli | sh -s -- create
```

## Public Command Shape

The host prints an agent prompt containing commands like:

```bash
curl -fsSL https://relay.example.com/cli | sh -s -- exec \
  --invite '<invite>' \
  -- '<COMMAND>'
```

The invite is bearer-secret material. Anyone with a valid invite can attempt to connect while the foreground host session is active.

## Verification

Run the standard local verification commands:

```bash
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
rm -f ./opentunnel
```

CI also builds downloadable binaries for:

- `linux-amd64`
- `darwin-arm64` Apple Silicon

## Documentation

- [Self-hosting](docs/public-v1/self-hosting.md)
- [Operations](docs/public-v1/operations.md)
- [Security model](docs/public-v1/security.md)
- [Non-goals](docs/public-v1/non-goals.md)
- [Acceptance mapping](docs/public-v1/acceptance.md)

## Security Notes

The relay sees routing/session, timing, size, and network metadata, but command traffic is end-to-end encrypted. Same-origin checksums for `/cli` artifacts detect corruption or mismatch within the trusted relay-origin model; they are not a strong supply-chain security boundary.

## Non-Goals

OpenTunnel v1 does not include accounts, dashboards, package-manager distribution, install-to-system flows, MCP, raw SSH, PTY, interactive stdin, file transfer, approval workflows, multiple clients for one tunnel, concurrent commands, persistent relay state, or persistent audit logs.
```

- [ ] **Step 2: Verify README links and command snippets**

Run:

```bash
test -s README.md
grep -n "Self-hosting\|Operations\|Security model\|Non-goals\|Acceptance mapping" README.md
grep -n "go test ./... -count=1\|go vet ./...\|go mod tidy -diff\|go test -race ./... -count=1\|go build ./cmd/opentunnel" README.md
grep -n "not a strong supply-chain security boundary\|bearer-secret" README.md
```

Expected: all commands pass and grep prints matching lines.

- [ ] **Step 3: Commit README**

Run:

```bash
git add README.md
git commit -m "docs: add root readme"
```

## Task 2: CI Cross-Build Artifacts

**Files:**
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Add cross-build artifact job**

Modify `.github/workflows/ci.yml` so it contains the existing `verify` job plus this additional job:

```yaml
  build-artifacts:
    runs-on: ubuntu-24.04
    strategy:
      matrix:
        include:
          - goos: linux
            goarch: amd64
            name: opentunnel-linux-amd64
          - goos: darwin
            goarch: arm64
            name: opentunnel-darwin-arm64
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23.0'
          cache: true

      - name: Build artifact
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: '0'
        run: go build -o dist/${{ matrix.name }} ./cmd/opentunnel

      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: ${{ matrix.name }}
          path: dist/${{ matrix.name }}
```

Keep the existing `verify` job unchanged except for YAML formatting needed to add the second job. Do not add release publishing, registry publishing, signing, or package-manager steps.

- [ ] **Step 2: Verify workflow contains both build targets**

Run:

```bash
grep -n "build-artifacts\|opentunnel-linux-amd64\|opentunnel-darwin-arm64\|actions/upload-artifact@v4" .github/workflows/ci.yml
```

Expected: grep prints matching lines for the job, both artifact names, and upload action.

- [ ] **Step 3: Verify cross-build commands locally**

Run:

```bash
mkdir -p /tmp/opencode/opentunnel-ci-artifacts
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/opencode/opentunnel-ci-artifacts/opentunnel-linux-amd64 ./cmd/opentunnel
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/opencode/opentunnel-ci-artifacts/opentunnel-darwin-arm64 ./cmd/opentunnel
test -s /tmp/opencode/opentunnel-ci-artifacts/opentunnel-linux-amd64
test -s /tmp/opencode/opentunnel-ci-artifacts/opentunnel-darwin-arm64
rm -rf /tmp/opencode/opentunnel-ci-artifacts
```

Expected: all commands pass.

- [ ] **Step 4: Run standard Go verification**

Run:

```bash
go test ./... -count=1
go vet ./...
go mod tidy -diff
```

Expected: all commands pass.

- [ ] **Step 5: Commit CI artifact build**

Run:

```bash
git add .github/workflows/ci.yml
git commit -m "ci: build release binaries"
```

## Task 3: Final README And Artifact Verification

**Files:**
- No committed source changes expected.

- [ ] **Step 1: Run full verification**

Run:

```bash
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
rm -f ./opentunnel
```

Expected: all commands pass and no root `opentunnel` binary remains.

- [ ] **Step 2: Re-run local cross-build verification**

Run:

```bash
mkdir -p /tmp/opencode/opentunnel-ci-artifacts
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/opencode/opentunnel-ci-artifacts/opentunnel-linux-amd64 ./cmd/opentunnel
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/opencode/opentunnel-ci-artifacts/opentunnel-darwin-arm64 ./cmd/opentunnel
test -s /tmp/opencode/opentunnel-ci-artifacts/opentunnel-linux-amd64
test -s /tmp/opencode/opentunnel-ci-artifacts/opentunnel-darwin-arm64
rm -rf /tmp/opencode/opentunnel-ci-artifacts
```

Expected: all commands pass.

- [ ] **Step 3: Check final status**

Run:

```bash
git status --short
```

Expected: only unrelated untracked `.agents/` and `skills-lock.json`, unless the user has changed additional files concurrently.

- [ ] **Step 4: Do not create an empty verification commit**

If verification required no file changes, leave the commit history unchanged. If verification changed files unexpectedly, stop and inspect the diff before proceeding.
