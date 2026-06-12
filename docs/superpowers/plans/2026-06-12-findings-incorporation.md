# OpenTunnel Findings Incorporation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Incorporate the actionable `findings.md` hardening, cleanup, dependency, CI, documentation, and repository-hygiene recommendations without changing OpenTunnel v1's product shape.

**Architecture:** Keep the relay as an opaque in-memory WebSocket router, but add resource limits and move routing metadata into custom headers. Keep one production secure-channel handshake path and make the host loop tolerant of bad pre-command client connections. Keep `--invite` compatibility while making env-var/stdin invite sources the recommended safer DX.

**Tech Stack:** Go 1.26 module, standard library HTTP, `github.com/gorilla/websocket`, `github.com/flynn/noise`, GitHub Actions, Docker, `govulncheck`, `golangci-lint`.

---

## File Structure

- Modify `go.mod` and `go.sum`: bump `go` directive and refresh `golang.org/x/crypto` / `golang.org/x/sys`.
- Modify `.github/workflows/ci.yml`: update Go version and add `govulncheck` / `golangci-lint` checks.
- Modify `deploy/docker/Dockerfile`: update builder image from `golang:1.23` to the selected supported Go version.
- Create `internal/originurl/originurl.go`: shared HTTP(S) origin validation used by CLI and bootstrap rendering.
- Create `internal/originurl/originurl_test.go`: shared validation tests.
- Modify `cmd/opentunnel/main.go`: use shared origin validation, set relay HTTP server timeouts, support `OPENTUNNEL_INVITE` and `--invite-stdin`, and print the env-var invite command shape.
- Modify `cmd/opentunnel/main_test.go`: update prompt assertions and add invite source tests.
- Modify `internal/artifact/bootstrap.go`: use shared origin validation and add the optional ordering comment for checksum-dependent cache path.
- Modify `internal/artifact/bootstrap_test.go`: update origin-validation error expectations if shared validation changes exact wording.
- Modify `internal/securechannel/channel.go`: remove duplicate one-shot handshake helpers.
- Modify `internal/securechannel/types.go`: remove `PatternXXpsk3`.
- Modify `internal/securechannel/channel_test.go`: rewrite helper-dependent tests against split handshake and remove XX availability test.
- Modify `internal/tunnel/host.go`: introduce tunnel header construction, extract a focused `hostRuntime` owner for relay/session loop state, and harden host-loop error handling.
- Modify `internal/tunnel/exec.go`: update client dialing to use tunnel headers.
- Modify `internal/tunnel/tunnel_test.go`: update helpers for tunnel headers and add rogue-client survival coverage.
- Modify `internal/relay/server.go`: read tunnel metadata from headers, reject browser `Origin`, add read limits, session caps, and reservation TTL.
- Modify `internal/relay/server_test.go`: update dialing helpers and add relay resource-limit/origin tests.
- Modify `internal/command/process_other.go`: make unsupported non-unix builds fail at compile time instead of silently degrading process cleanup.
- Modify `README.md` and `docs/public-v1/*.md`: update command examples, security notes, relay metadata notes, and invite handling guidance.
- Move `plan.md` to `docs/internal-planning/plan.md`; leave `docs/superpowers/` in place because the active workflow requires specs and plans there, and document that exception in the final summary.

Do not commit during execution unless the user explicitly asks for commits.

---

### Task 1: Dependency And Toolchain Update

**Files:**
- Modify: `go.mod`
- Modify: `go.sum`
- Modify: `.github/workflows/ci.yml`
- Modify: `deploy/docker/Dockerfile`

- [ ] **Step 1: Update Go directive and direct toolchain references**

Change `go.mod` line 3 from:

```go
go 1.23
```

to:

```go
go 1.26
```

Change `.github/workflows/ci.yml` Go setup versions from:

```yaml
go-version: '1.23.0'
```

to:

```yaml
go-version: '1.26.0'
```

Change `deploy/docker/Dockerfile` from:

```dockerfile
FROM golang:1.23 AS builder
```

to:

```dockerfile
FROM golang:1.26 AS builder
```

- [ ] **Step 2: Refresh stale indirect dependencies**

Run:

```sh
go get -u golang.org/x/crypto golang.org/x/sys
go mod tidy
```

Expected: `go.mod` and `go.sum` update only for dependency/toolchain metadata. If `go get` upgrades unrelated direct dependencies, inspect and revert unrelated upgrades manually with minimal edits.

- [ ] **Step 3: Verify dependency group**

Run:

```sh
go test ./... -count=1
go vet ./...
go mod tidy -diff
```

Expected: all commands pass and `go mod tidy -diff` prints no diff.

---

### Task 2: Shared Origin Validation

**Files:**
- Create: `internal/originurl/originurl.go`
- Create: `internal/originurl/originurl_test.go`
- Modify: `cmd/opentunnel/main.go`
- Modify: `cmd/opentunnel/main_test.go`
- Modify: `internal/artifact/bootstrap.go`

- [ ] **Step 1: Write shared validator tests**

Create `internal/originurl/originurl_test.go`:

```go
package originurl

import "testing"

func TestValidateAcceptsHTTPAndHTTPSOrigins(t *testing.T) {
	for _, raw := range []string{
		"http://localhost:8080",
		"https://relay.example.com",
		"https://[::1]:8443",
	} {
		t.Run(raw, func(t *testing.T) {
			if err := Validate(raw, "relay origin"); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestValidateRejectsNonOriginsAndUnsafeHosts(t *testing.T) {
	tests := []string{
		"http://example.test/$(id)",
		"http://example.test/path",
		"http://example.test?download=true",
		"http://example.test#fragment",
		"http://user@example.test",
		"ftp://example.test",
		"http:///missing-host",
		"-http://example.test",
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			if err := Validate(raw, "relay origin"); err == nil {
				t.Fatal("Validate() error = nil, want error")
			}
		})
	}
}
```

- [ ] **Step 2: Run shared validator tests to verify they fail**

Run:

```sh
go test ./internal/originurl -count=1
```

Expected: FAIL because package or `Validate` is not defined.

- [ ] **Step 3: Add shared validator implementation**

Create `internal/originurl/originurl.go`:

```go
package originurl

import (
	"fmt"
	"net/url"
	"strings"
)

// Validate checks that raw is an HTTP(S) origin safe to embed in shell snippets.
func Validate(raw string, name string) error {
	if strings.HasPrefix(raw, "-") {
		return fmt.Errorf("%s must not start with '-'", name)
	}
	origin, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse %s: %w", name, err)
	}
	if origin.Scheme != "http" && origin.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", name)
	}
	if origin.Host == "" {
		return fmt.Errorf("%s host is required", name)
	}
	if origin.User != nil {
		return fmt.Errorf("%s must not include userinfo", name)
	}
	if origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return fmt.Errorf("%s must be an origin without path, query, or fragment", name)
	}
	if !isShellSafeHost(origin.Host) {
		return fmt.Errorf("%s host contains unsafe characters", name)
	}
	return nil
}

func isShellSafeHost(host string) bool {
	for _, char := range host {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		switch char {
		case '.', '-', ':', '[', ']', '%':
			continue
		default:
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Use shared validator in CLI**

In `cmd/opentunnel/main.go`, add import:

```go
"opentunnel/internal/originurl"
```

Remove the `net/url` import only if still unused after keeping `websocketRelayURL`.

Replace `validatePublicURL`, `validateRelayOrigin`, and `isShellSafeURLHost` with:

```go
func validateRelayOrigin(raw string, name string) error {
	return originurl.Validate(raw, name)
}
```

Change `parseRelayArgs` validation from:

```go
if err := validatePublicURL(cmd.publicURL); err != nil {
```

to:

```go
if err := validateRelayOrigin(cmd.publicURL, "public url"); err != nil {
```

- [ ] **Step 5: Use shared validator in bootstrap renderer**

In `internal/artifact/bootstrap.go`, add import:

```go
"opentunnel/internal/originurl"
```

Remove the `net/url` import.

Replace local `validateRelayOrigin` with:

```go
func validateRelayOrigin(relayOrigin string) error {
	return originurl.Validate(relayOrigin, "relay origin")
}
```

Add this comment immediately before the `cache_dir=` line in the rendered script:

```sh
# expected_checksum is assigned by the platform case above and scopes the cache by artifact content.
```

- [ ] **Step 6: Verify shared validation group**

Run:

```sh
go test ./internal/originurl ./cmd/opentunnel ./internal/artifact -count=1
go test ./... -count=1
```

Expected: all tests pass.

---

### Task 3: Secure-Channel Duplicate Path Cleanup

**Files:**
- Modify: `internal/securechannel/channel.go`
- Modify: `internal/securechannel/types.go`
- Modify: `internal/securechannel/channel_test.go`
- Modify: `internal/tunnel/tunnel_test.go`

- [ ] **Step 1: Add split-handshake test helper in securechannel tests**

In `internal/securechannel/channel_test.go`, remove the `github.com/flynn/noise` import and add this helper before `testHandshakeConfig`:

```go
func testSplitChannels(t *testing.T, clientCfg HandshakeConfig, hostCfg HandshakeConfig, hostKey HostKeypair, expectedHostPublic []byte) (*Channel, *Channel, error) {
	t.Helper()

	clientHS, err := NewClientHandshake(clientCfg, expectedHostPublic)
	if err != nil {
		return nil, nil, err
	}
	hostHS, err := NewHostHandshake(hostCfg, hostKey)
	if err != nil {
		return nil, nil, err
	}
	msg1, err := clientHS.WriteMessage()
	if err != nil {
		return nil, nil, err
	}
	msg2, host, err := hostHS.ReadMessage(msg1)
	if err != nil {
		return nil, nil, err
	}
	client, err := clientHS.ReadMessage(msg2)
	if err != nil {
		return nil, nil, err
	}
	return client, host, nil
}
```

- [ ] **Step 2: Rewrite tests to use helper**

Replace every `EstablishChannelWithHostKey(...)`, `establishNKpsk0(...)`, and `establishNKpsk0WithConfigs(...)` call in `internal/securechannel/channel_test.go` with `testSplitChannels(...)`.

For the wrong-host-public-key test, assert `NewClientHandshake` succeeds but `clientHS.ReadMessage(msg2)` fails when the host response is from a different key. Use this shape:

```go
clientHS, err := NewClientHandshake(cfg, otherHostKey.Public)
if err != nil {
	t.Fatalf("NewClientHandshake: %v", err)
}
hostHS, err := NewHostHandshake(cfg, hostKey)
if err != nil {
	t.Fatalf("NewHostHandshake: %v", err)
}
msg1, err := clientHS.WriteMessage()
if err != nil {
	t.Fatalf("client write message: %v", err)
}
msg2, _, err := hostHS.ReadMessage(msg1)
if err != nil {
	t.Fatalf("host read message: %v", err)
}
if _, err := clientHS.ReadMessage(msg2); err == nil {
	t.Fatal("expected wrong host public key to fail")
}
```

- [ ] **Step 3: Delete XX availability test and constant**

Delete `TestXXpsk3PatternIsAvailableForFallbackEvaluation` from `internal/securechannel/channel_test.go`.

Delete this constant from `internal/securechannel/types.go`:

```go
// PatternXXpsk3 is the Noise XX pattern with PSK in message position 3.
PatternXXpsk3 = "Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s"
```

- [ ] **Step 4: Delete duplicate production helpers**

Remove `crypto/subtle` from `internal/securechannel/channel.go` imports.

Delete these functions from `internal/securechannel/channel.go`:

```go
func EstablishChannelWithHostKey(cfg HandshakeConfig, hostKey HostKeypair, expectedHostPublic []byte) (*Channel, *Channel, error)
func establishNKpsk0(cfg HandshakeConfig, hostKey noise.DHKey, expectedHostPublic []byte) (*Channel, *Channel, error)
func establishNKpsk0WithConfigs(clientCfg HandshakeConfig, hostCfg HandshakeConfig, hostKey noise.DHKey, expectedHostPublic []byte) (*Channel, *Channel, error)
```

- [ ] **Step 5: Update tunnel test helper**

Replace `testChannels` in `internal/tunnel/tunnel_test.go` with:

```go
func testChannels(t *testing.T) (*securechannel.Channel, *securechannel.Channel) {
	t.Helper()

	var clientSecret [securechannel.ClientSecretSize]byte
	for i := range clientSecret {
		clientSecret[i] = byte(i + 1)
	}
	hostKey, err := securechannel.GenerateHostKeypair(strings.NewReader(strings.Repeat("a", 64)))
	if err != nil {
		t.Fatalf("generate host keypair: %v", err)
	}
	cfg := handshakeConfig("test-session", "ws://relay.example", clientSecret)
	clientHS, err := securechannel.NewClientHandshake(cfg, hostKey.Public)
	if err != nil {
		t.Fatalf("new client handshake: %v", err)
	}
	hostHS, err := securechannel.NewHostHandshake(cfg, hostKey)
	if err != nil {
		t.Fatalf("new host handshake: %v", err)
	}
	msg1, err := clientHS.WriteMessage()
	if err != nil {
		t.Fatalf("write client handshake: %v", err)
	}
	msg2, host, err := hostHS.ReadMessage(msg1)
	if err != nil {
		t.Fatalf("read host handshake: %v", err)
	}
	client, err := clientHS.ReadMessage(msg2)
	if err != nil {
		t.Fatalf("read client handshake: %v", err)
	}
	return client, host
}
```

- [ ] **Step 6: Verify secure-channel cleanup**

Run:

```sh
go test ./internal/securechannel ./internal/tunnel -count=1
```

Expected: all tests pass and `grep` for removed helpers returns no production references.

---

### Task 4: Tunnel Metadata Headers

**Files:**
- Modify: `internal/tunnel/host.go`
- Modify: `internal/tunnel/exec.go`
- Modify: `internal/tunnel/tunnel_test.go`
- Modify: `internal/relay/server.go`
- Modify: `internal/relay/server_test.go`
- Modify: `docs/public-v1/security.md`

- [ ] **Step 1: Add relay header constants and parser tests**

In `internal/relay/server.go`, add constants near imports:

```go
const (
	tunnelRoleHeader    = "OpenTunnel-Role"
	tunnelSessionHeader = "OpenTunnel-Session"
)
```

In `internal/relay/server_test.go`, update `tunnelURL` to return only the static path:

```go
func tunnelURL(serverURL string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + "/tunnel"
}
```

Add helper:

```go
func tunnelHeader(role, session string) http.Header {
	header := http.Header{}
	header.Set(tunnelRoleHeader, role)
	header.Set(tunnelSessionHeader, session)
	return header
}
```

Update all `websocket.DefaultDialer.Dial(tunnelURL(...), nil)` calls in relay tests to pass `tunnelHeader(role, session)`.

Update `dialTunnel` to:

```go
func dialTunnel(t *testing.T, serverURL, role, session string) *websocket.Conn {
	t.Helper()

	conn, response, err := websocket.DefaultDialer.Dial(tunnelURL(serverURL), tunnelHeader(role, session))
	if err != nil {
		if response != nil {
			defer response.Body.Close()
			t.Fatalf("dial %s: %v status=%s", role, err, response.Status)
		}
		t.Fatalf("dial %s: %v", role, err)
	}
	return conn
}
```

Update the `CheckOrigin` test hook role read from:

```go
r.URL.Query().Get("role")
```

to:

```go
r.Header.Get(tunnelRoleHeader)
```

- [ ] **Step 2: Update relay to read headers**

In `internal/relay/server.go`, replace:

```go
role := r.URL.Query().Get("role")
sessionID := r.URL.Query().Get("session")
```

with:

```go
role := r.Header.Get(tunnelRoleHeader)
sessionID := r.Header.Get(tunnelSessionHeader)
```

- [ ] **Step 3: Update tunnel dialer endpoint/header API**

In `internal/tunnel/host.go`, replace `tunnelEndpoint` with:

```go
const (
	tunnelRoleHeader    = "OpenTunnel-Role"
	tunnelSessionHeader = "OpenTunnel-Session"
)

func tunnelEndpoint(relayURL *url.URL) string {
	endpoint := *relayURL
	endpoint.Path = "/tunnel"
	endpoint.RawQuery = ""
	endpoint.Fragment = ""
	return endpoint.String()
}

func tunnelHeader(role string, sessionID string) http.Header {
	header := http.Header{}
	header.Set(tunnelRoleHeader, role)
	header.Set(tunnelSessionHeader, sessionID)
	return header
}
```

Update `StartHost` initial dial to:

```go
endpoint := tunnelEndpoint(relayURL)
header := tunnelHeader("host", sessionID)
conn, _, err := websocket.DefaultDialer.Dial(endpoint, header)
```

Update `runHost` endpoint setup to use `tunnelEndpoint(relayURL)` and pass `header` to `dialHostRelay`.

Update `dialHostRelay` signature and body:

```go
func dialHostRelay(ctx context.Context, endpoint string, header http.Header) (*websocket.Conn, error) {
	for {
		conn, response, err := websocket.DefaultDialer.DialContext(ctx, endpoint, header)
```

- [ ] **Step 4: Update client dialer**

In `internal/tunnel/exec.go`, update the WebSocket dial call from query endpoint to:

```go
conn, _, err := websocket.DefaultDialer.Dial(tunnelEndpoint(relayURL), tunnelHeader("client", payload.SessionID))
```

- [ ] **Step 5: Update tunnel tests**

In `internal/tunnel/tunnel_test.go`, update `connectTestClient` dial to:

```go
conn, _, err := websocket.DefaultDialer.Dial(tunnelEndpoint(relayURL), tunnelHeader("client", payload.SessionID))
```

- [ ] **Step 6: Verify metadata header group**

Run:

```sh
go test ./internal/relay ./internal/tunnel -count=1
```

Expected: all tests pass. Manual code search should show no `?role=` tunnel URLs remain.

---

### Task 5: Relay Browser-Origin And Resource Limits

**Files:**
- Modify: `internal/relay/server.go`
- Modify: `internal/relay/server_test.go`
- Modify: `cmd/opentunnel/main.go`

- [ ] **Step 1: Add limit fields and defaults**

In `internal/relay/server.go`, add constants:

```go
const (
	defaultMaxSessions       = 1024
	defaultReservationTTL    = 30 * time.Second
	defaultMaxFrameBytes int64 = 1 * 1024 * 1024
)
```

Add `time` to imports.

Extend `ServerOptions`:

```go
MaxSessions    int
ReservationTTL time.Duration
MaxFrameBytes  int64
```

Extend `Server`:

```go
maxSessions    int
reservationTTL time.Duration
maxFrameBytes  int64
```

Extend `session`:

```go
reservedAt time.Time
```

Add helpers:

```go
func effectiveMaxSessions(value int) int {
	if value == 0 {
		return defaultMaxSessions
	}
	return value
}

func effectiveReservationTTL(value time.Duration) time.Duration {
	if value == 0 {
		return defaultReservationTTL
	}
	return value
}

func effectiveMaxFrameBytes(value int64) int64 {
	if value == 0 {
		return defaultMaxFrameBytes
	}
	return value
}
```

- [ ] **Step 2: Wire options and browser Origin rejection**

Change `newServer` to accept options:

```go
func newServer(options ServerOptions) *Server {
	return &Server{
		sessions:       make(map[string]*session),
		maxSessions:    effectiveMaxSessions(options.MaxSessions),
		reservationTTL: effectiveReservationTTL(options.ReservationTTL),
		maxFrameBytes:  effectiveMaxFrameBytes(options.MaxFrameBytes),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return r.Header.Get("Origin") == ""
			},
		},
	}
}
```

Update callers:

```go
func NewServer() *Server {
	return newServer(ServerOptions{})
}

func NewServerWithOptions(options ServerOptions) (*Server, error) {
	server := newServer(options)
```

- [ ] **Step 3: Add reservation reaping and caps**

At the start of `reserve`, after locking, call:

```go
s.reapExpiredReservationsLocked(time.Now())
```

When creating a new host session, reject if cap is reached:

```go
if tunnelSession == nil {
	if len(s.sessions) >= s.maxSessions {
		return false
	}
	tunnelSession = &session{}
	s.sessions[sessionID] = tunnelSession
}
tunnelSession.reservedAt = time.Now()
```

Add method:

```go
func (s *Server) reapExpiredReservationsLocked(now time.Time) {
	if s.reservationTTL <= 0 {
		return
	}
	for sessionID, tunnelSession := range s.sessions {
		if tunnelSession.host == nil && tunnelSession.client == nil && tunnelSession.hostReserved && !tunnelSession.clientReserved && now.Sub(tunnelSession.reservedAt) > s.reservationTTL {
			delete(s.sessions, sessionID)
		}
	}
}
```

- [ ] **Step 4: Apply WebSocket read limit**

After upgrade succeeds in `Handler`, before `attach`, add:

```go
conn.SetReadLimit(s.maxFrameBytes)
```

- [ ] **Step 5: Add relay tests**

Add tests to `internal/relay/server_test.go`:

```go
func TestWebSocketWithOriginIsRejected(t *testing.T) {
	server := NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	header := tunnelHeader("host", "s1")
	header.Set("Origin", "https://evil.example")
	conn, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL), header)
	if err == nil {
		conn.Close()
		t.Fatal("dial succeeded, want origin rejection")
	}
	if response == nil {
		t.Fatal("response = nil, want rejection response")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want non-101", response.StatusCode)
	}
}

func TestWebSocketWithoutOriginUpgrades(t *testing.T) {
	server := NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	host := dialTunnel(t, httpServer.URL, "host", "s1")
	defer host.Close()
}

func TestSessionCapRejectsNewHostSessions(t *testing.T) {
	server, err := NewServerWithOptions(ServerOptions{MaxSessions: 1})
	if err != nil {
		t.Fatalf("NewServerWithOptions: %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	first := dialTunnel(t, httpServer.URL, "host", "s1")
	defer first.Close()

	_, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL), tunnelHeader("host", "s2"))
	if err == nil {
		t.Fatal("second host dial succeeded, want cap rejection")
	}
	if response == nil {
		t.Fatal("response = nil, want rejection response")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusConflict)
	}
}

func TestReservationTTLReapsUnattachedHostReservation(t *testing.T) {
	server, err := NewServerWithOptions(ServerOptions{MaxSessions: 1, ReservationTTL: time.Nanosecond})
	if err != nil {
		t.Fatalf("NewServerWithOptions: %v", err)
	}
	server.sessions["expired"] = &session{hostReserved: true, reservedAt: time.Now().Add(-time.Second)}
	if !server.reserve("host", "fresh") {
		t.Fatal("reserve fresh host = false, want true after TTL reap")
	}
}

func TestOversizedFrameIsRejected(t *testing.T) {
	server, err := NewServerWithOptions(ServerOptions{MaxFrameBytes: 8})
	if err != nil {
		t.Fatalf("NewServerWithOptions: %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	host := dialTunnel(t, httpServer.URL, "host", "s1")
	defer host.Close()
	client := dialTunnel(t, httpServer.URL, "client", "s1")
	defer client.Close()

	if err := client.WriteMessage(websocket.BinaryMessage, []byte("0123456789abcdef")); err != nil {
		t.Fatalf("write oversized frame: %v", err)
	}
	_, _, err = readMessage(t, host)
	if err == nil {
		t.Fatal("host read oversized frame = nil error, want connection close")
	}
}
```

- [ ] **Step 6: Add HTTP server timeouts**

In `cmd/opentunnel/main.go`, change server construction to:

```go
server := &http.Server{
	Addr:              cmd.listen,
	Handler:           relayServer.Handler(),
	ReadHeaderTimeout: 10 * time.Second,
	IdleTimeout:       60 * time.Second,
}
```

Add `time` to imports.

- [ ] **Step 7: Verify relay hardening**

Run:

```sh
go test ./internal/relay ./cmd/opentunnel -count=1
```

Expected: all tests pass.

---

### Task 6: Focused Host Runtime Refactor

**Files:**
- Modify: `internal/tunnel/host.go`
- Test: `internal/tunnel/tunnel_test.go`

- [ ] **Step 1: Run existing host tests before refactor**

Run:

```sh
go test ./internal/tunnel -run 'TestStartHostSessionRunsSequentialExecsWithSameInvite|TestHostIdleTimeout|TestHostIdleTimeoutRestartsAfterCommandWhenClientKeepsSocketOpen|TestClientDisconnectCancelsSilentCommandPromptly' -count=1
```

Expected: all selected tests pass before refactoring.

- [ ] **Step 2: Add a hostRuntime type that owns session loop configuration**

In `internal/tunnel/host.go`, add this type near `HostSession`:

```go
type hostRuntime struct {
	conn           *websocket.Conn
	hostKey        securechannel.HostKeypair
	clientSecret   [securechannel.ClientSecretSize]byte
	relayURL       *url.URL
	relay          string
	sessionID      string
	commandTimeout time.Duration
	idleTimeout    time.Duration
	maxOutputBytes int
	logger         *hostLogger
	endpoint       string
	header         http.Header
}
```

- [ ] **Step 3: Build hostRuntime in StartHost**

After generating the invite in `StartHost`, replace the direct dial setup with:

```go
endpoint := tunnelEndpoint(relayURL)
header := tunnelHeader("host", sessionID)
conn, _, err := websocket.DefaultDialer.Dial(endpoint, header)
if err != nil {
	return HostSession{}, fmt.Errorf("connect host relay websocket: %w", err)
}
```

Replace the `go runHost(...)` call with:

```go
runtime := hostRuntime{
	conn:           conn,
	hostKey:        hostKey,
	clientSecret:   clientSecret,
	relayURL:       relayURL,
	relay:          relayURL.String(),
	sessionID:      sessionID,
	commandTimeout: cfg.CommandTimeout,
	idleTimeout:    effectiveIdleTimeout(cfg.IdleTimeout),
	maxOutputBytes: effectiveMaxOutputBytes(cfg.MaxOutputBytes),
	logger:         &logger,
	endpoint:       endpoint,
	header:         header,
}
go runtime.run(ctx, done)
```

- [ ] **Step 4: Convert runHost into a hostRuntime method**

Replace `runHost(...)` with:

```go
func (h *hostRuntime) run(ctx context.Context, done chan<- error) {
	h.logger.log("sessionOpen")
	defer func() {
		h.logger.log("sessionClose")
		close(done)
	}()

	hostCtx, cancelHost := context.WithCancelCause(ctx)
	defer cancelHost(nil)
	idleTimer := time.NewTimer(h.idleTimeout)
	defer idleTimer.Stop()
	go func() {
		select {
		case <-idleTimer.C:
			h.logger.log("idleTimeout")
			cancelHost(errIdleSessionTimeout)
		case <-hostCtx.Done():
		}
	}()

	for {
		h.logger.log("waiting")
		err := handleOneHostConnection(hostCtx, h.conn, h.hostKey, h.clientSecret, h.relay, h.sessionID, h.commandTimeout, h.maxOutputBytes, h.logger, func() {
			stopTimer(idleTimer)
		})
		if err != nil && ctx.Err() == nil {
			if errors.Is(context.Cause(hostCtx), errIdleSessionTimeout) {
				done <- fmt.Errorf("%w: session idle timeout", errIdleSessionTimeout)
				return
			}
			done <- err
			return
		}
		if ctx.Err() != nil {
			return
		}
		resetTimer(idleTimer, h.idleTimeout)

		conn, err := dialHostRelay(hostCtx, h.endpoint, h.header)
		if err != nil {
			if errors.Is(context.Cause(hostCtx), errIdleSessionTimeout) {
				done <- fmt.Errorf("%w: session idle timeout", errIdleSessionTimeout)
				return
			}
			if hostCtx.Err() != nil {
				return
			}
			done <- fmt.Errorf("connect host relay websocket: %w", err)
			return
		}
		h.conn = conn
	}
}
```

This is a mechanical move only; the rogue-client resilience change comes in the next task.

- [ ] **Step 5: Verify behavior after refactor**

Run:

```sh
go test ./internal/tunnel -count=1
```

Expected: all tunnel tests pass with no behavior change.

---

### Task 7: Host Loop Rogue Client Resilience

**Files:**
- Modify: `internal/tunnel/host.go`
- Modify: `internal/tunnel/tunnel_test.go`

- [ ] **Step 1: Add failing rogue-client survival test**

Add to `internal/tunnel/tunnel_test.go`:

```go
func TestRogueClientHandshakeFailureDoesNotStopHostSession(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{RelayURL: relayURL(server.URL)})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	payload, err := invite.Decode(session.Invite)
	if err != nil {
		t.Fatalf("decode invite: %v", err)
	}
	parsedRelayURL, err := parseRelayURL(payload.Relay)
	if err != nil {
		t.Fatalf("parse relay url: %v", err)
	}
	rogue, _, err := websocket.DefaultDialer.Dial(tunnelEndpoint(parsedRelayURL), tunnelHeader("client", payload.SessionID))
	if err != nil {
		t.Fatalf("connect rogue client: %v", err)
	}
	if err := rogue.WriteMessage(websocket.BinaryMessage, []byte("not a noise handshake")); err != nil {
		rogue.Close()
		t.Fatalf("write rogue handshake: %v", err)
	}
	rogue.Close()

	var stdout bytes.Buffer
	deadline := time.Now().Add(time.Second)
	for {
		stdout.Reset()
		_, err = Exec(ctx, ExecConfig{Invite: session.Invite, Command: "printf alive", Stdout: &stdout})
		if err == nil {
			break
		}
		select {
		case hostErr := <-session.Done:
			t.Fatalf("host stopped after rogue client: %v", hostErr)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("legitimate exec after rogue client: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if stdout.String() != "alive" {
		t.Fatalf("stdout = %q, want alive", stdout.String())
	}
}
```

- [ ] **Step 2: Run test to verify current failure**

Run:

```sh
go test ./internal/tunnel -run TestRogueClientHandshakeFailureDoesNotStopHostSession -count=1
```

Expected before implementation: FAIL because host session stops or the legitimate exec cannot reconnect.

- [ ] **Step 3: Introduce non-fatal pre-command error marker**

In `internal/tunnel/host.go`, add:

```go
var errPreCommandClientFailure = errors.New("pre-command client failure")
```

In `handleOneCommand`, wrap handshake read and host handshake failures:

```go
_, msg1, err := conn.ReadMessage()
if err != nil {
	return fmt.Errorf("%w: read client handshake: %v", errPreCommandClientFailure, err)
}
logger.log("clientConnected")

handshake, err := securechannel.NewHostHandshake(handshakeConfig(sessionID, relay, clientSecret), hostKey)
if err != nil {
	return err
}
msg2, channel, err := handshake.ReadMessage(msg1)
if err != nil {
	return fmt.Errorf("%w: host read handshake: %v", errPreCommandClientFailure, err)
}
```

- [ ] **Step 4: Handle pre-command failures in host loop**

In `runHost`, replace the error branch after `handleOneHostConnection` with:

```go
if err != nil && ctx.Err() == nil {
	if errors.Is(context.Cause(hostCtx), errIdleSessionTimeout) {
		done <- fmt.Errorf("%w: session idle timeout", errIdleSessionTimeout)
		return
	}
	if errors.Is(err, errPreCommandClientFailure) {
		logger.log("clientRejected")
	} else {
		done <- err
		return
	}
}
```

Keep the reconnect path after this branch so rejected clients cause a re-dial.

- [ ] **Step 5: Verify host resilience**

Run:

```sh
go test ./internal/tunnel -run TestRogueClientHandshakeFailureDoesNotStopHostSession -count=1
go test ./internal/tunnel -count=1
```

Expected: all tunnel tests pass.

---

### Task 8: Invite Environment And Stdin DX

**Files:**
- Modify: `cmd/opentunnel/main.go`
- Modify: `cmd/opentunnel/main_test.go`

- [ ] **Step 1: Add parse tests for env fallback and stdin flag**

Add to `cmd/opentunnel/main_test.go`:

```go
func TestParseArgsExecUsesInviteFromEnvironment(t *testing.T) {
	t.Setenv("OPENTUNNEL_INVITE", "ot1_from_env")

	cmd, err := parseArgs([]string{"exec", "--", "hostname"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	exec, ok := cmd.(execCommand)
	if !ok {
		t.Fatalf("parseArgs() command = %T, want execCommand", cmd)
	}
	if exec.invite != "ot1_from_env" {
		t.Fatalf("exec.invite = %q, want env invite", exec.invite)
	}
}

func TestParseArgsExecInviteFlagWinsOverEnvironment(t *testing.T) {
	t.Setenv("OPENTUNNEL_INVITE", "ot1_from_env")

	cmd, err := parseArgs([]string{"exec", "--invite", "ot1_from_flag", "--", "hostname"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	exec, ok := cmd.(execCommand)
	if !ok {
		t.Fatalf("parseArgs() command = %T, want execCommand", cmd)
	}
	if exec.invite != "ot1_from_flag" {
		t.Fatalf("exec.invite = %q, want flag invite", exec.invite)
	}
}

func TestParseArgsExecRequiresInviteSource(t *testing.T) {
	t.Setenv("OPENTUNNEL_INVITE", "")

	_, err := parseArgs([]string{"exec", "--", "hostname"})
	if err == nil {
		t.Fatal("parseArgs() error = nil, want error")
	}
	for _, want := range []string{"--invite", "OPENTUNNEL_INVITE", "--invite-stdin"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("parseArgs() error = %q, want %q", err.Error(), want)
		}
	}
}
```

- [ ] **Step 2: Extend exec command configuration**

In `cmd/opentunnel/main.go`, change `execCommand` to:

```go
type execCommand struct {
	invite      string
	inviteStdin bool
	command     string
}
```

In `parseExecArgs`, add:

```go
flags.BoolVar(&cmd.inviteStdin, "invite-stdin", false, "read invite code from stdin")
```

After flag parsing, replace the invite-required check with:

```go
if cmd.invite == "" && !cmd.inviteStdin {
	cmd.invite = os.Getenv("OPENTUNNEL_INVITE")
}
if cmd.invite == "" && !cmd.inviteStdin {
	return execCommand{}, errors.New("exec requires --invite, OPENTUNNEL_INVITE, or --invite-stdin")
}
```

- [ ] **Step 3: Read stdin invite at runtime**

In `run`, change exec command invocation to give access to stdin by adding a package-level helper if needed. Minimal approach: change `main` to call a new function:

```go
func main() {
	os.Exit(runWithStdin(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	return runWithStdin(ctx, args, os.Stdin, stdout, stderr)
}

func runWithStdin(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	cmd, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "opentunnel: %v\n", err)
		return 2
	}
	if exec, ok := cmd.(execCommand); ok {
		return exec.runWithStdin(ctx, stdin, stdout, stderr)
	}
	return cmd.run(ctx, stdout, stderr)
}
```

Add method:

```go
func (cmd execCommand) runWithStdin(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if cmd.inviteStdin {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "exec: read invite from stdin: %v\n", err)
			return 1
		}
		cmd.invite = strings.TrimSpace(string(data))
		if cmd.invite == "" {
			fmt.Fprintln(stderr, "exec: invite from stdin is empty")
			return 1
		}
	}
	return cmd.run(ctx, stdout, stderr)
}
```

- [ ] **Step 4: Update create prompt**

In `writeCreateReady`, replace both command examples with env-var form:

```go
curl -fsSL %[1]s/cli | OPENTUNNEL_INVITE='%[2]s' sh -s -- exec \
  -- '<COMMAND>'
```

and:

```go
curl -fsSL %[1]s/cli | OPENTUNNEL_INVITE='%[2]s' sh -s -- exec \
  -- 'hostname && uname -a && pwd'
```

Add a note line:

```text
- For shared machines, prefer --invite-stdin or shell-history controls because typed environment assignments can still be saved by your shell.
```

- [ ] **Step 5: Update prompt test assertions**

In `TestWriteCreateReadyPrintsPublicAgentPrompt`, replace wants for `--invite` with:

```go
"curl -fsSL http://localhost:8080/cli | OPENTUNNEL_INVITE='" + invite + "' sh -s -- exec \\",
"  -- '<COMMAND>'",
"For shared machines, prefer --invite-stdin",
```

Keep `strings.Count(output, invite) == 2` because the invite still appears once in each example.

- [ ] **Step 6: Verify invite DX group**

Run:

```sh
go test ./cmd/opentunnel -count=1
```

Expected: all CLI tests pass.

---

### Task 9: Documentation Updates

**Files:**
- Modify: `README.md`
- Modify: `docs/public-v1/security.md`
- Modify: `docs/public-v1/self-hosting.md`
- Modify: `docs/public-v1/operations.md`
- Modify: `docs/public-v1/acceptance.md` if it contains command-shape assumptions

- [ ] **Step 1: Search for stale command shape and query metadata references**

Run:

```sh
rg "--invite|role=|session=|Origin|OPENTUNNEL_INVITE|/tunnel" README.md docs/public-v1
```

Expected: identify every doc location requiring update.

- [ ] **Step 2: Update README public command shape**

Replace the public command example in `README.md` with:

```md
curl -fsSL https://relay.example.com/cli | OPENTUNNEL_INVITE='<invite>' sh -s -- exec \
  -- '<COMMAND>'
```

Add below the invite paragraph:

```md
The generated prompt uses `OPENTUNNEL_INVITE` so the long-lived `opentunnel exec` process does not expose the invite in process listings. On shared machines, shell history can still capture typed environment assignments; use `--invite-stdin` or shell-history controls when that matters.
```

- [ ] **Step 3: Update security model**

In `docs/public-v1/security.md`, add under Trust Boundaries:

```md
The relay routes by session ID. The session ID is a sensitive routing token carried in the `OpenTunnel-Session` WebSocket request header, with `OpenTunnel-Role` identifying host or client. Standard access logs record request paths by default, so the tunnel endpoint is a static `/tunnel` path and operators must not add these custom headers to logged-header allowlists.
```

Add under Invite Handling:

```md
Prefer `OPENTUNNEL_INVITE` or `--invite-stdin` over `--invite` on shared systems. The `--invite` flag is supported for compatibility, but it places bearer-secret material in process argv. Environment assignments avoid exposing the invite from the long-lived `opentunnel exec` process, though typed shell commands may still be saved in shell history.
```

Add relay browser stance:

```md
The relay intentionally rejects WebSocket upgrade requests that include an `Origin` header. Browser WebSocket handshakes include this header, while the supported Go CLI clients do not. This keeps the public relay endpoint focused on non-browser clients and reduces drive-by browser DoS exposure.
```

- [ ] **Step 4: Update remaining docs**

For every command example in `docs/public-v1/*.md`, prefer:

```sh
curl -fsSL https://relay.example.com/cli | OPENTUNNEL_INVITE='<invite>' sh -s -- exec -- '<COMMAND>'
```

Do not remove mentions that `--invite` exists if they are compatibility notes. Reword them as compatibility fallback, not preferred usage.

- [ ] **Step 5: Verify docs search**

Run:

```sh
rg "--invite '<invite>'|role=|session=" README.md docs/public-v1
```

Expected: no stale preferred command examples and no tunnel query-string docs remain.

---

### Task 10: CI Vulnerability And Lint Checks

**Files:**
- Modify: `.github/workflows/ci.yml`
- Create: `.golangci.yml`

- [ ] **Step 1: Add golangci-lint configuration**

Create `.golangci.yml`:

```yaml
version: "2"

linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
    - gofmt
    - goimports
    - misspell
    - unconvert
    - unparam
```

- [ ] **Step 2: Add CI steps**

In `.github/workflows/ci.yml`, after `Vet`, add:

```yaml
      - name: Install govulncheck
        run: go install golang.org/x/vuln/cmd/govulncheck@latest

      - name: Vulnerability check
        run: govulncheck ./...

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v8
        with:
          version: latest
```

- [ ] **Step 3: Run local equivalents if installed**

Run:

```sh
go test ./... -count=1
go vet ./...
```

Then attempt:

```sh
govulncheck ./...
golangci-lint run
```

Expected: if tools are not installed locally, record that CI wiring was added and local execution was unavailable. If tools are installed and report issues, fix issues before continuing.

---

### Task 11: Unsupported Platform And Repository Hygiene

**Files:**
- Modify: `internal/command/process_other.go`
- Move: `plan.md` to `docs/internal-planning/plan.md`

- [ ] **Step 1: Make non-unix command cleanup builds fail clearly**

Replace `internal/command/process_other.go` with:

```go
//go:build !unix

package command

import "os/exec"

// OpenTunnel's command cancellation relies on Unix process groups. Release
// targets are linux and darwin; non-unix builds fail instead of silently
// degrading child-process cleanup.
var _ = unsupportedNonUnixOpenTunnelBuild

func configureCommandCleanup(cmd *exec.Cmd) {
	_ = cmd
}
```

- [ ] **Step 2: Verify non-unix build fails and unix tests still pass**

Run:

```sh
GOOS=windows GOARCH=amd64 go test ./internal/command
go test ./internal/command -count=1
```

Expected: the Windows-targeted command fails with an `undefined: unsupportedNonUnixOpenTunnelBuild` compile error, and the host-platform command test passes.

- [ ] **Step 3: Move root planning artifact out of the repository root**

Create `docs/internal-planning/` and move the large root `plan.md` there:

```sh
mkdir -p docs/internal-planning
mv plan.md docs/internal-planning/plan.md
```

Keep `docs/superpowers/` in place because the active Superpowers workflow stores reviewed specs and plans there. Record this as the D3 exception in the final summary instead of relocating the active workflow files.

- [ ] **Step 4: Verify planning artifact move**

Run:

```sh
test -f docs/internal-planning/plan.md
test ! -f plan.md
```

Expected: both commands exit successfully.

---

### Task 12: Full Verification

**Files:**
- Verify all modified files

- [ ] **Step 1: Run standard verification suite**

Run:

```sh
go test ./... -count=1
go vet ./...
go mod tidy -diff
go test -race ./... -count=1
go build ./cmd/opentunnel
rm -f ./opentunnel
```

Expected: all commands pass and the built binary is removed.

- [ ] **Step 2: Inspect worktree**

Run:

```sh
git status --short
git diff --stat
```

Expected: only intended files changed. `findings.md` may remain untracked because it existed before this implementation work and should not be modified unless explicitly requested.

- [ ] **Step 3: Summarize results**

Report:

```text
Implemented: D1, D2, M1, M2, M3, M4, M5, S1, S2, S3, S4, S5, S6, docs updates, and root planning-artifact relocation.
Deferred: S7/M6 no action by design. D3 exception: `docs/superpowers/` remains because the active workflow stores specs and plans there.
Verification: list each command and whether it passed or why it could not run.
```
