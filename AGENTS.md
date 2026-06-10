## Project Context

- This repository contains OpenTunnel, a Go 1.23 command-line and relay implementation.
- The module name is `opentunnel`.
- The entrypoint is under `cmd/opentunnel`; reusable implementation packages live under `internal/`.
- Deployment artifacts live under `deploy/`, and operational documentation lives under `docs/`.

## Go Style

- Prefer small, direct, idiomatic Go over clever abstractions.
- Keep package names short, lowercase, and focused on what the package provides.
- Accept interfaces at package boundaries when they make testing or substitution simpler; return concrete types by default.
- Pass `context.Context` as the first parameter for operations that can block, perform I/O, or need cancellation.
- Wrap errors with useful operation context using `%w`; do not discard errors silently.
- Avoid package-level mutable state unless it is immutable configuration or a deliberate process-wide singleton.
- Use `gofmt` formatting for all Go changes.

## Testing And Verification

- Keep tests close to the implementation using `*_test.go` files.
- Prefer table-driven tests when they make cases clearer.
- Keep tests deterministic; avoid sleeps, timing assumptions, and external network dependencies unless explicitly required.
- Run the standard verification commands after implementation changes:
  - `go test ./... -count=1`
  - `go vet ./...`
  - `go mod tidy -diff`
  - `go test -race ./... -count=1`
  - `go build ./cmd/opentunnel`
- Remove the local `./opentunnel` binary after build verification when it is produced in the repository root.

## OpenTunnel Design Constraints

- Preserve the v1 shape: one temporary host process, one client, one command at a time, and a relay-served temporary CLI.
- Treat invites as bearer-secret material.
- Keep command traffic end-to-end encrypted; the relay should route opaque packets and avoid command awareness.
- Keep relay state temporary and in-memory unless a task explicitly changes the product scope.
- Do not add accounts, dashboards, package-manager distribution, install-to-system flows, MCP, raw SSH, PTY, interactive stdin, file transfer, approval workflows, multiple clients for one tunnel, concurrent commands, persistent relay state, or persistent audit logs without explicit approval.

## Dependency And Artifact Hygiene

- Keep dependencies minimal; prefer the standard library when it is sufficient.
- Do not edit generated build artifacts or committed release outputs unless the task is specifically about those artifacts.
- Use `go mod tidy -diff` to verify module files remain tidy instead of changing `go.mod` or `go.sum` speculatively.
- Do not vendor external repositories into this project unless explicitly requested.
