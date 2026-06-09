# OpenTunnel Local Binary Vertical Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Milestone 2 from `docs/superpowers/specs/2026-05-22-opentunnel-v1-milestone-design.md`: local `opentunnel relay`, `opentunnel create --relay ...`, and `opentunnel exec --invite ... -- '<command>'` execute one remote command through a relay-blind encrypted tunnel.

**Architecture:** Keep the product split into small internal packages: `securechannel` owns Noise handshakes and encrypted frames, `invite` owns opaque invite encoding/decoding, `relay` owns in-memory opaque WebSocket routing, `tunnel` owns encrypted command protocol flow, `command` owns local process execution, and `cmd/opentunnel` wires subcommands. Milestone 2 deliberately omits `/cli`, binary cache, lifecycle hardening, advanced abuse controls, PTY/stdin/file transfer, and polished deployment.

**Tech Stack:** Go, `github.com/flynn/noise`, `github.com/gorilla/websocket`, standard `net/http`, standard `encoding/json`, standard `encoding/base64`, standard `os/exec`, standard `context`, standard `testing`.

---

## File Structure

- Modify: `go.mod`, `go.sum`
  - Add `github.com/gorilla/websocket` for local relay transport.
- Modify: `internal/securechannel/channel.go`
  - Add explicit client/host handshake state helpers for real transport flow.
- Modify: `internal/securechannel/channel_test.go`
  - Prove split handshake messages produce working bidirectional encrypted channels.
- Create: `internal/invite/invite.go`
  - Encode/decode `ot1_` invite strings locally.
- Create: `internal/invite/invite_test.go`
  - Test round trip, malformed prefix, and malformed payload.
- Create: `internal/relay/server.go`
  - HTTP/WebSocket relay with in-memory host/client routes and opaque frame copying.
- Create: `internal/relay/server_test.go`
  - Test host/client pairing and second-client rejection.
- Create: `internal/command/runner.go`
  - Run one non-interactive shell command and stream stdout/stderr chunks.
- Create: `internal/command/runner_test.go`
  - Test stdout/stderr capture and exit-code propagation.
- Create: `internal/tunnel/protocol.go`
  - Encrypted JSON command protocol message definitions and helpers.
- Create: `internal/tunnel/host.go`
  - Host-side encrypted command handling over a WebSocket.
- Create: `internal/tunnel/client.go`
  - Client-side encrypted command request and streamed output handling.
- Create: `internal/tunnel/tunnel_test.go`
  - In-process relay/host/client vertical-slice test for `hostname`-style command behavior.
- Create: `cmd/opentunnel/main.go`
  - Local `relay`, `create --relay`, and `exec --invite -- <command>` entry points.
- Create: `cmd/opentunnel/main_test.go`
  - CLI argument parsing tests.

## Task 1: Add Transport Handshake API

**Files:**
- Modify: `internal/securechannel/channel.go`
- Modify: `internal/securechannel/channel_test.go`

- [ ] **Step 1: Write failing split-handshake test**

Add `TestSplitNKpsk0HandshakeMatchesInMemoryChannel` to `internal/securechannel/channel_test.go`. The test must create `HandshakeConfig`, generate a host key, call a wished-for client handshake constructor, call a wished-for host handshake constructor, exchange `msg1` and `msg2`, and verify client-to-host plus host-to-client encrypted frame round trips.

Use these wished-for APIs:

```go
clientHS, err := NewClientHandshake(cfg, hostKey.Public)
hostHS, err := NewHostHandshake(cfg, hostKey)
msg1, err := clientHS.WriteMessage()
msg2, host, err := hostHS.ReadMessage(msg1)
client, err := clientHS.ReadMessage(msg2)
```

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/securechannel -run TestSplitNKpsk0HandshakeMatchesInMemoryChannel -count=1
```

Expected: FAIL because `NewClientHandshake` and `NewHostHandshake` are undefined.

- [ ] **Step 3: Implement minimal split-handshake API**

Add exported `ClientHandshake` and `HostHandshake` types with doc comments. Implement the wished-for methods by extracting the current NKpsk0 handshake construction from `establishNKpsk0WithConfigs`. Keep existing tests passing.

Required signatures:

```go
type ClientHandshake struct { /* unexported fields */ }
type HostHandshake struct { /* unexported fields */ }

func NewClientHandshake(cfg HandshakeConfig, expectedHostPublic []byte) (*ClientHandshake, error)
func NewHostHandshake(cfg HandshakeConfig, hostKey HostKeypair) (*HostHandshake, error)
func (h *ClientHandshake) WriteMessage() ([]byte, error)
func (h *ClientHandshake) ReadMessage(message []byte) (*Channel, error)
func (h *HostHandshake) ReadMessage(message []byte) ([]byte, *Channel, error)
```

- [ ] **Step 4: Verify green**

Run:

```bash
gofmt -w internal/securechannel/*.go
go test ./internal/securechannel -count=1
go vet ./...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/securechannel/channel.go internal/securechannel/channel_test.go
git commit -m "feat: add secure channel transport handshake API"
```

## Task 2: Add Opaque Invite Encoding

**Files:**
- Create: `internal/invite/invite.go`
- Create: `internal/invite/invite_test.go`

- [ ] **Step 1: Write failing invite tests**

Create tests for:

- `Encode` returns `ot1_` prefix.
- `Decode(Encode(payload))` round trips relay URL, session ID, host public key bytes, and 32-byte client secret.
- Decode rejects missing `ot1_` prefix.
- Decode rejects malformed base64 payload.

Use wished-for API:

```go
type Payload struct {
    Relay string
    SessionID string
    HostPublicKey []byte
    ClientSecret [32]byte
}

func Encode(payload Payload) (string, error)
func Decode(code string) (Payload, error)
```

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/invite -count=1
```

Expected: FAIL because package implementation is missing.

- [ ] **Step 3: Implement invite package**

Use JSON internally for Milestone 2 and base64url without padding externally. The invite remains opaque to users because they only see `ot1_<base64url>`. Validate relay, session ID, 32-byte host public key, and non-zero client secret bytes. Do not involve the relay in decoding.

- [ ] **Step 4: Verify green**

Run:

```bash
gofmt -w internal/invite/*.go
go test ./internal/invite -count=1
go test ./... -count=1
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/invite/invite.go internal/invite/invite_test.go
git commit -m "feat: add opaque invite encoding"
```

## Task 3: Add Command Runner

**Files:**
- Create: `internal/command/runner.go`
- Create: `internal/command/runner_test.go`

- [ ] **Step 1: Write failing command runner tests**

Create tests for:

- Running `printf hello` returns stdout chunk `hello`, exit code 0.
- Running a command that writes to stderr captures stderr separately.
- Running `exit 7` returns exit code 7 without treating it as a runner transport failure.

Use wished-for API:

```go
type OutputChunk struct { Stream string; Data []byte }
type Result struct { ExitCode int }
func Run(ctx context.Context, command string, onChunk func(OutputChunk)) (Result, error)
```

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/command -count=1
```

Expected: FAIL because package implementation is missing.

- [ ] **Step 3: Implement command runner**

Use `/bin/sh -c <command>` for Milestone 2 on Unix-like systems. Use `exec.CommandContext`, stdout/stderr pipes, goroutines with `sync.WaitGroup`, and return the process exit code from `*exec.ExitError`. Do not add timeout, process-group cleanup, PTY, stdin, or output limits in this milestone.

- [ ] **Step 4: Verify green**

Run:

```bash
gofmt -w internal/command/*.go
go test ./internal/command -count=1
go test ./... -count=1
go vet ./...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/command/runner.go internal/command/runner_test.go
git commit -m "feat: add local command runner"
```

## Task 4: Add Relay Opaque Routing

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/relay/server.go`
- Create: `internal/relay/server_test.go`

- [ ] **Step 1: Write failing relay tests**

Create tests using `httptest.Server` and `github.com/gorilla/websocket` for:

- Host connects to `/tunnel?role=host&session=s1`; client connects to `/tunnel?role=client&session=s1`; a binary message from client reaches host unchanged.
- A second client for the same session is rejected while the first client is connected.

Use wished-for API:

```go
server := relay.NewServer()
handler := server.Handler()
```

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/relay -count=1
```

Expected: FAIL because package implementation or websocket dependency is missing.

- [ ] **Step 3: Add dependency and implement relay**

Run:

```bash
go get github.com/gorilla/websocket@v1.5.3
```

Implement in-memory maps protected by `sync.Mutex`. Relay must route binary messages opaquely and never inspect payload contents. Remove routes on connection close. Reject unknown roles, missing sessions, client-before-host, and second active client with HTTP errors before WebSocket upgrade.

- [ ] **Step 4: Verify green**

Run:

```bash
gofmt -w internal/relay/*.go
go test ./internal/relay -count=1
go test ./... -count=1
go vet ./...
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/relay/server.go internal/relay/server_test.go
git commit -m "feat: add in-memory opaque relay"
```

## Task 5: Add Encrypted Tunnel Protocol

**Files:**
- Create: `internal/tunnel/protocol.go`
- Create: `internal/tunnel/host.go`
- Create: `internal/tunnel/client.go`
- Create: `internal/tunnel/tunnel_test.go`

- [ ] **Step 1: Write failing tunnel vertical-slice test**

Create an in-process test with `httptest.Server`, `relay.NewServer`, a host goroutine, and a client call. The test should execute `printf hello` and assert stdout `hello`, exit code 0, and no plaintext command is sent through a relay spy hook if such hook is added for testing.

Use wished-for APIs:

```go
session, inviteCode, err := tunnel.StartHost(ctx, tunnel.HostConfig{RelayURL: relayURL})
result, err := tunnel.Exec(ctx, tunnel.ExecConfig{Invite: inviteCode, Command: "printf hello", Stdout: &stdout, Stderr: &stderr})
```

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./internal/tunnel -count=1
```

Expected: FAIL because package implementation is missing.

- [ ] **Step 3: Implement protocol messages**

Define encrypted JSON payloads with `type`, `command`, `stream`, `data`, and `exitCode` fields. Keep the relay-visible WebSocket messages as binary Noise handshake/ciphertext frames only.

- [ ] **Step 4: Implement host flow**

`StartHost` generates session ID, client secret, host key, invite code, connects to relay as host, performs host side of Noise handshake per client connection, decrypts one command request, runs it through `internal/command`, streams encrypted output frames, sends encrypted exit frame, and keeps the host goroutine alive until context cancellation or relay disconnect.

- [ ] **Step 5: Implement client flow**

`Exec` decodes invite locally, connects to relay as client using only `sessionId`, performs client side of Noise handshake, sends encrypted command request, writes streamed stdout/stderr to configured writers, and returns remote exit code.

- [ ] **Step 6: Verify green**

Run:

```bash
gofmt -w internal/tunnel/*.go
go test ./internal/tunnel -count=1
go test ./... -count=1
go vet ./...
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/tunnel/protocol.go internal/tunnel/host.go internal/tunnel/client.go internal/tunnel/tunnel_test.go
git commit -m "feat: add encrypted tunnel command flow"
```

## Task 6: Add Local CLI Entry Points

**Files:**
- Create: `cmd/opentunnel/main.go`
- Create: `cmd/opentunnel/main_test.go`

- [ ] **Step 1: Write failing CLI parsing tests**

Test parsing for:

- `relay --listen :8080 --public-url http://localhost:8080`
- `create --relay http://localhost:8080`
- `exec --invite ot1_example -- hostname`

Use wished-for `parseArgs(args []string) (command, error)` inside package main.

- [ ] **Step 2: Verify red**

Run:

```bash
go test ./cmd/opentunnel -count=1
```

Expected: FAIL because CLI implementation is missing.

- [ ] **Step 3: Implement CLI**

Use `flag.FlagSet` per subcommand. `relay` starts `relay.NewServer` on `--listen`. `create` calls `tunnel.StartHost`, prints an agent-ready local prompt with the invite, then blocks until interrupted. `exec` calls `tunnel.Exec` and exits with the remote exit code. Keep developer-only `--relay` for Milestone 2.

- [ ] **Step 4: Verify green**

Run:

```bash
gofmt -w cmd/opentunnel/*.go
go test ./cmd/opentunnel -count=1
go test ./... -count=1
go vet ./...
go build ./cmd/opentunnel
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add cmd/opentunnel/main.go cmd/opentunnel/main_test.go
git commit -m "feat: add local opentunnel CLI"
```

## Task 7: Final Milestone 2 Verification

**Files:**
- Verify all files created or modified by this plan.

- [ ] **Step 1: Run final verification**

Run:

```bash
gofmt -w internal/securechannel/*.go internal/invite/*.go internal/command/*.go internal/relay/*.go internal/tunnel/*.go cmd/opentunnel/*.go
go mod tidy
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go build ./cmd/opentunnel
```

Expected: all pass.

- [ ] **Step 2: Run a manual local vertical slice**

Build the binary, start relay, start create in the background, extract the invite from its output, run exec for `printf hello`, and assert the client prints `hello` and exits 0. Use temporary files and clean up background processes.

- [ ] **Step 3: Commit final cleanup if needed**

If formatting or docs changed:

```bash
git add go.mod go.sum cmd internal docs/superpowers/plans/2026-06-09-opentunnel-local-binary-vertical-slice.md
git commit -m "test: verify local binary vertical slice"
```

Do not create an empty commit.

## Self-Review Checklist

- Milestone 2 gate is covered: host creates invite, client decodes invite locally, relay sees only role/session routing material, command request/output/exit status are encrypted frames, client exits with remote exit code, and host/relay disconnect ends the session.
- Explicit out-of-scope items remain absent: `/cli`, binary caching, artifact serving, public installer UX, PTY/stdin/file transfer, lifecycle hardening, advanced abuse controls, metrics.
- Every package has tests close to implementation.
- Every production behavior was introduced with a failing test first.
- Final verification includes normal tests, race tests, vet, module tidy, build, and a manual local vertical slice.
