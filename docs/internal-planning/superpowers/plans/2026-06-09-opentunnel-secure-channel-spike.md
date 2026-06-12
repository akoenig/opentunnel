# OpenTunnel Secure Channel Spike Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove Milestone 1 from `docs/superpowers/specs/2026-05-22-opentunnel-v1-milestone-design.md`: Go can cleanly implement OpenTunnel's Noise-based secure channel, failure semantics, and narrow package boundary.

**Architecture:** Create a minimal Go module with one internal `securechannel` package. The package owns handshake configuration, canonical prologue construction, Noise session setup, encrypted frame send/receive helpers, and spike documentation. No relay, CLI, command execution, or `/cli` distribution is implemented in this plan.

**Tech Stack:** Go, `github.com/flynn/noise`, standard `testing`, standard `crypto/rand`, standard `encoding/binary`, standard `bytes`, standard `errors`.

---

## File Structure

- Create: `go.mod`
  - Defines the Go module and pins `github.com/flynn/noise`.
- Create: `internal/securechannel/doc.go`
  - Documents package purpose and explicitly states the spike boundary.
- Create: `internal/securechannel/types.go`
  - Defines stable input types, selected pattern constants, and semantic errors.
- Create: `internal/securechannel/prologue.go`
  - Builds canonical length-prefixed prologue bytes.
- Create: `internal/securechannel/prologue_test.go`
  - Tests prologue determinism and field sensitivity.
- Create: `internal/securechannel/channel.go`
  - Implements client/host handshake helpers and encrypted frame helpers.
- Create: `internal/securechannel/channel_test.go`
  - Tests happy-path encrypted multi-frame exchange and required failure cases.
- Create: `docs/superpowers/spikes/opentunnel-secure-channel-decision.md`
  - Records selected Noise pattern, rationale, rejected alternative, and gate result.

## Task 1: Initialize Minimal Go Module

**Files:**
- Create: `go.mod`

- [ ] **Step 1: Create the Go module file**

Create `go.mod`:

```go
module opentunnel

go 1.23

require github.com/flynn/noise v1.1.0
```

- [ ] **Step 2: Download module dependencies**

Run:

```bash
go mod download
```

Expected: command exits 0 and creates `go.sum`.

- [ ] **Step 3: Verify module metadata**

Run:

```bash
go list -m all
```

Expected: output includes `opentunnel` and `github.com/flynn/noise v1.1.0`.

- [ ] **Step 4: Commit module initialization**

Run:

```bash
git add go.mod go.sum
git commit -m "chore: initialize Go module"
```

If the workspace is not a Git repository, record that in the task notes and continue without committing.

## Task 2: Add Secure Channel Package Boundary

**Files:**
- Create: `internal/securechannel/doc.go`
- Create: `internal/securechannel/types.go`

- [ ] **Step 1: Create package documentation**

Create `internal/securechannel/doc.go`:

```go
// Package securechannel contains the OpenTunnel v1 secure-channel spike.
//
// This package is intentionally narrow. It proves the Noise handshake,
// prologue binding, PSK handling, host key verification, and encrypted frame
// behavior required before relay or command-execution work begins.
//
// The package must not import relay, CLI, command runner, or artifact-serving
// code. Higher-level product code should depend on this package through small
// functions and data types rather than knowing Noise library details.
package securechannel
```

- [ ] **Step 2: Create initial package types**

Create `internal/securechannel/types.go`:

```go
package securechannel

import "errors"

const (
	PatternNKpsk0 = "Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s"
	PatternXXpsk3 = "Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s"

	SelectedPattern = PatternNKpsk0

	ClientSecretSize = 32
	CommandTimeoutSeconds = 120
	MaxOutputBytes = 10 * 1024 * 1024
	IdleSessionTimeoutSeconds = 1800
)

var (
	ErrInvalidClientSecret = errors.New("invalid client secret")
	ErrHostKeyMismatch = errors.New("host public key mismatch")
	ErrHandshakeFailed = errors.New("handshake failed")
	ErrProtocol = errors.New("protocol error")
)

type CommandDefaults struct {
	TimeoutSeconds int
	MaxOutputBytes int
	PTY bool
	IdleSessionTimeoutSeconds int
}

type PrologueConfig struct {
	App string
	InviteVersion byte
	NoiseProtocol string
	SessionID string
	RelayOrigin string
	PermissionMode string
	CommandDefaults CommandDefaults
	Features []string
}

type HandshakeConfig struct {
	SessionID string
	RelayOrigin string
	ClientSecret [ClientSecretSize]byte
	PermissionMode string
	Features []string
}

func DefaultCommandDefaults() CommandDefaults {
	return CommandDefaults{
		TimeoutSeconds: CommandTimeoutSeconds,
		MaxOutputBytes: MaxOutputBytes,
		PTY: false,
		IdleSessionTimeoutSeconds: IdleSessionTimeoutSeconds,
	}
}

func NewPrologueConfig(cfg HandshakeConfig) PrologueConfig {
	return PrologueConfig{
		App: "OpenTunnel",
		InviteVersion: 1,
		NoiseProtocol: SelectedPattern,
		SessionID: cfg.SessionID,
		RelayOrigin: cfg.RelayOrigin,
		PermissionMode: cfg.PermissionMode,
		CommandDefaults: DefaultCommandDefaults(),
		Features: append([]string(nil), cfg.Features...),
	}
}
```

- [ ] **Step 3: Run package tests before tests exist**

Run:

```bash
go test ./internal/securechannel
```

Expected: PASS with output similar to `? opentunnel/internal/securechannel [no test files]`.

- [ ] **Step 4: Commit package boundary**

Run:

```bash
git add internal/securechannel/doc.go internal/securechannel/types.go
git commit -m "feat: define secure channel package boundary"
```

If the workspace is not a Git repository, record that in the task notes and continue without committing.

## Task 3: Implement Canonical Prologue Binding

**Files:**
- Create: `internal/securechannel/prologue.go`
- Create: `internal/securechannel/prologue_test.go`

- [ ] **Step 1: Write failing prologue tests**

Create `internal/securechannel/prologue_test.go`:

```go
package securechannel

import (
	"bytes"
	"testing"
)

func TestBuildPrologueDeterministic(t *testing.T) {
	cfg := PrologueConfig{
		App: "OpenTunnel",
		InviteVersion: 1,
		NoiseProtocol: PatternNKpsk0,
		SessionID: "stn_test",
		RelayOrigin: "https://relay.example",
		PermissionMode: "yolo",
		CommandDefaults: DefaultCommandDefaults(),
		Features: []string{"exec.v1", "stdoutStderr.v1"},
	}

	first, err := BuildPrologue(cfg)
	if err != nil {
		t.Fatalf("BuildPrologue first call: %v", err)
	}

	second, err := BuildPrologue(cfg)
	if err != nil {
		t.Fatalf("BuildPrologue second call: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Fatalf("prologue is not deterministic")
	}
}

func TestBuildPrologueChangesWhenSecurityContextChanges(t *testing.T) {
	base := PrologueConfig{
		App: "OpenTunnel",
		InviteVersion: 1,
		NoiseProtocol: PatternNKpsk0,
		SessionID: "stn_test",
		RelayOrigin: "https://relay.example",
		PermissionMode: "yolo",
		CommandDefaults: DefaultCommandDefaults(),
		Features: []string{"exec.v1", "stdoutStderr.v1"},
	}

	baseBytes, err := BuildPrologue(base)
	if err != nil {
		t.Fatalf("BuildPrologue base: %v", err)
	}

	changed := base
	changed.SessionID = "stn_other"

	changedBytes, err := BuildPrologue(changed)
	if err != nil {
		t.Fatalf("BuildPrologue changed: %v", err)
	}

	if bytes.Equal(baseBytes, changedBytes) {
		t.Fatalf("prologue did not change when session id changed")
	}
}

func TestBuildPrologueRejectsMissingFields(t *testing.T) {
	_, err := BuildPrologue(PrologueConfig{
		App: "OpenTunnel",
		InviteVersion: 1,
		NoiseProtocol: PatternNKpsk0,
		SessionID: "",
		RelayOrigin: "https://relay.example",
		PermissionMode: "yolo",
		CommandDefaults: DefaultCommandDefaults(),
		Features: []string{"exec.v1"},
	})

	if err == nil {
		t.Fatalf("expected missing session id error")
	}
}
```

- [ ] **Step 2: Run prologue tests to verify they fail**

Run:

```bash
go test ./internal/securechannel -run TestBuildPrologue -count=1
```

Expected: FAIL because `BuildPrologue` is undefined.

- [ ] **Step 3: Implement prologue construction**

Create `internal/securechannel/prologue.go`:

```go
package securechannel

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

func BuildPrologue(cfg PrologueConfig) ([]byte, error) {
	if cfg.App == "" {
		return nil, fmt.Errorf("build prologue: app is required")
	}
	if cfg.InviteVersion == 0 {
		return nil, fmt.Errorf("build prologue: invite version is required")
	}
	if cfg.NoiseProtocol == "" {
		return nil, fmt.Errorf("build prologue: noise protocol is required")
	}
	if cfg.SessionID == "" {
		return nil, fmt.Errorf("build prologue: session id is required")
	}
	if cfg.RelayOrigin == "" {
		return nil, fmt.Errorf("build prologue: relay origin is required")
	}
	if cfg.PermissionMode == "" {
		return nil, fmt.Errorf("build prologue: permission mode is required")
	}
	if len(cfg.Features) == 0 {
		return nil, fmt.Errorf("build prologue: at least one feature is required")
	}

	var buf bytes.Buffer
	writeField(&buf, []byte(cfg.App))
	writeField(&buf, []byte{cfg.InviteVersion})
	writeField(&buf, []byte(cfg.NoiseProtocol))
	writeField(&buf, []byte(cfg.SessionID))
	writeField(&buf, []byte(cfg.RelayOrigin))
	writeField(&buf, []byte(cfg.PermissionMode))
	writeUint32(&buf, uint32(cfg.CommandDefaults.TimeoutSeconds))
	writeUint32(&buf, uint32(cfg.CommandDefaults.MaxOutputBytes))
	if cfg.CommandDefaults.PTY {
		writeField(&buf, []byte{1})
	} else {
		writeField(&buf, []byte{0})
	}
	writeUint32(&buf, uint32(cfg.CommandDefaults.IdleSessionTimeoutSeconds))
	writeUint32(&buf, uint32(len(cfg.Features)))
	for _, feature := range cfg.Features {
		if feature == "" {
			return nil, fmt.Errorf("build prologue: feature cannot be empty")
		}
		writeField(&buf, []byte(feature))
	}

	return buf.Bytes(), nil
}

func writeField(buf *bytes.Buffer, value []byte) {
	writeUint32(buf, uint32(len(value)))
	buf.Write(value)
}

func writeUint32(buf *bytes.Buffer, value uint32) {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	buf.Write(encoded[:])
}
```

- [ ] **Step 4: Run prologue tests to verify they pass**

Run:

```bash
go test ./internal/securechannel -run TestBuildPrologue -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit prologue binding**

Run:

```bash
git add internal/securechannel/prologue.go internal/securechannel/prologue_test.go
git commit -m "feat: add secure channel prologue binding"
```

If the workspace is not a Git repository, record that in the task notes and continue without committing.

## Task 4: Prove Happy-Path Noise Handshake And Multi-Frame Transport

**Files:**
- Create: `internal/securechannel/channel.go`
- Create: `internal/securechannel/channel_test.go`

- [ ] **Step 1: Write failing happy-path test**

Create `internal/securechannel/channel_test.go`:

```go
package securechannel

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/flynn/noise"
)

func TestNKpsk0HandshakeEncryptsMultipleFrames(t *testing.T) {
	cfg := testHandshakeConfig(t)
	hostKey, err := GenerateHostKeypair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateHostKeypair: %v", err)
	}

	client, host, err := EstablishChannelWithHostKey(cfg, hostKey, hostKey.Public)
	if err != nil {
		t.Fatalf("EstablishChannelWithHostKey: %v", err)
	}

	frames := [][]byte{
		[]byte("commandRequest:hostname"),
		[]byte("stdoutData:api-staging"),
		[]byte("exitStatus:0"),
	}

	for _, frame := range frames {
		ciphertext, err := client.Encrypt(frame)
		if err != nil {
			t.Fatalf("client encrypt: %v", err)
		}
		if bytes.Contains(ciphertext, frame) {
			t.Fatalf("ciphertext contains plaintext frame %q", frame)
		}

		plaintext, err := host.Decrypt(ciphertext)
		if err != nil {
			t.Fatalf("host decrypt: %v", err)
		}
		if !bytes.Equal(plaintext, frame) {
			t.Fatalf("plaintext mismatch: got %q want %q", plaintext, frame)
		}
	}
}

func testHandshakeConfig(t *testing.T) HandshakeConfig {
	t.Helper()

	var secret [ClientSecretSize]byte
	if _, err := rand.Read(secret[:]); err != nil {
		t.Fatalf("generate client secret: %v", err)
	}

	return HandshakeConfig{
		SessionID: "stn_test",
		RelayOrigin: "https://relay.example",
		ClientSecret: secret,
		PermissionMode: "yolo",
		Features: []string{"exec.v1", "stdoutStderr.v1"},
	}
}

var _ = noise.DH25519
```

- [ ] **Step 2: Run happy-path test to verify it fails**

Run:

```bash
go test ./internal/securechannel -run TestNKpsk0HandshakeEncryptsMultipleFrames -count=1
```

Expected: FAIL because `GenerateHostKeypair`, `EstablishChannelWithHostKey`, `Encrypt`, and `Decrypt` are undefined.

- [ ] **Step 3: Implement minimal secure channel helpers**

Create `internal/securechannel/channel.go`:

```go
package securechannel

import (
	"crypto/subtle"
	"fmt"
	"io"

	"github.com/flynn/noise"
)

type HostKeypair struct {
	Public []byte
	private noise.DHKey
}

type Channel struct {
	send *noise.CipherState
	recv *noise.CipherState
}

func GenerateHostKeypair(r io.Reader) (HostKeypair, error) {
	key, err := noise.DH25519.GenerateKeypair(r)
	if err != nil {
		return HostKeypair{}, fmt.Errorf("generate host keypair: %w", err)
	}

	return HostKeypair{
		Public: append([]byte(nil), key.Public...),
		private: key,
	}, nil
}

func EstablishChannelWithHostKey(cfg HandshakeConfig, hostKey HostKeypair, expectedHostPublic []byte) (*Channel, *Channel, error) {
	return establishNKpsk0(cfg, hostKey.private, expectedHostPublic)
}

func establishNKpsk0(cfg HandshakeConfig, hostKey noise.DHKey, expectedHostPublic []byte) (*Channel, *Channel, error) {
	if len(expectedHostPublic) == 0 {
		return nil, nil, fmt.Errorf("%w: expected host public key is required", ErrHostKeyMismatch)
	}

	prologue, err := BuildPrologue(NewPrologueConfig(cfg))
	if err != nil {
		return nil, nil, err
	}

	noiseCfg := noise.Config{
		CipherSuite: noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s),
		Pattern: noise.HandshakeNK,
		Initiator: true,
		Prologue: prologue,
		PresharedKey: cfg.ClientSecret[:],
		PresharedKeyPlacement: 0,
		PeerStatic: expectedHostPublic,
	}

	clientHS, err := noise.NewHandshakeState(noiseCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: create client handshake: %w", ErrHandshakeFailed, err)
	}

	hostCfg := noiseCfg
	hostCfg.Initiator = false
	hostCfg.StaticKeypair = hostKey
	hostCfg.PeerStatic = nil

	hostHS, err := noise.NewHandshakeState(hostCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: create host handshake: %w", ErrHandshakeFailed, err)
	}

	msg1, _, _, err := clientHS.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: client write handshake: %w", ErrHandshakeFailed, err)
	}

	if _, _, _, err := hostHS.ReadMessage(nil, msg1); err != nil {
		return nil, nil, fmt.Errorf("%w: host read handshake: %w", ErrHandshakeFailed, err)
	}

	msg2, hostSend, hostRecv, err := hostHS.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: host write handshake: %w", ErrHandshakeFailed, err)
	}

	_, clientSend, clientRecv, err := clientHS.ReadMessage(nil, msg2)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: client read handshake: %w", ErrHandshakeFailed, err)
	}

	if subtle.ConstantTimeCompare(hostKey.Public, expectedHostPublic) != 1 {
		return nil, nil, ErrHostKeyMismatch
	}

	return &Channel{send: clientSend, recv: clientRecv}, &Channel{send: hostSend, recv: hostRecv}, nil
}

func (c *Channel) Encrypt(plaintext []byte) ([]byte, error) {
	if c == nil || c.send == nil {
		return nil, fmt.Errorf("%w: send cipher is not initialized", ErrProtocol)
	}
	return c.send.Encrypt(nil, nil, plaintext), nil
}

func (c *Channel) Decrypt(ciphertext []byte) ([]byte, error) {
	if c == nil || c.recv == nil {
		return nil, fmt.Errorf("%w: receive cipher is not initialized", ErrProtocol)
	}
	plaintext, err := c.recv.Decrypt(nil, nil, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("%w: decrypt frame: %w", ErrProtocol, err)
	}
	return plaintext, nil
}
```

- [ ] **Step 4: Run happy-path test to verify it passes**

Run:

```bash
go test ./internal/securechannel -run TestNKpsk0HandshakeEncryptsMultipleFrames -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit happy-path secure channel spike**

Run:

```bash
git add internal/securechannel/channel.go internal/securechannel/channel_test.go
git commit -m "feat: prove secure channel happy path"
```

If the workspace is not a Git repository, record that in the task notes and continue without committing.

## Task 5: Prove Required Handshake Failure Cases

**Files:**
- Modify: `internal/securechannel/channel_test.go`
- Modify: `internal/securechannel/channel.go`

- [ ] **Step 1: Add failing failure-case tests**

Append to `internal/securechannel/channel_test.go`:

```go
func TestHandshakeFailsWithWrongClientSecret(t *testing.T) {
	cfg := testHandshakeConfig(t)
	hostKey, err := GenerateHostKeypair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateHostKeypair: %v", err)
	}

	wrong := cfg
	wrong.ClientSecret[0] ^= 0xff

	_, _, err = EstablishMismatchedTestChannel(wrong, cfg, hostKey)
	if err == nil {
		t.Fatalf("expected wrong client secret to fail")
	}
}

func TestHandshakeFailsWithWrongHostPublicKey(t *testing.T) {
	cfg := testHandshakeConfig(t)
	hostKey, err := GenerateHostKeypair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateHostKeypair: %v", err)
	}
	otherHostKey, err := GenerateHostKeypair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateHostKeypair other: %v", err)
	}

	_, _, err = EstablishMismatchedHostKeyTestChannel(cfg, hostKey, otherHostKey.Public)
	if err == nil {
		t.Fatalf("expected wrong host public key to fail")
	}
}

func TestHandshakeFailsWithWrongPrologue(t *testing.T) {
	clientCfg := testHandshakeConfig(t)
	hostCfg := clientCfg
	hostCfg.RelayOrigin = "https://other-relay.example"

	hostKey, err := GenerateHostKeypair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateHostKeypair: %v", err)
	}

	_, _, err = EstablishMismatchedTestChannel(clientCfg, hostCfg, hostKey)
	if err == nil {
		t.Fatalf("expected wrong prologue to fail")
	}
}

func TestDecryptRejectsReplayedCiphertext(t *testing.T) {
	cfg := testHandshakeConfig(t)
	hostKey, err := GenerateHostKeypair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateHostKeypair: %v", err)
	}

	client, host, err := EstablishChannelWithHostKey(cfg, hostKey, hostKey.Public)
	if err != nil {
		t.Fatalf("EstablishChannelWithHostKey: %v", err)
	}

	ciphertext, err := client.Encrypt([]byte("stdoutData:first"))
	if err != nil {
		t.Fatalf("client encrypt: %v", err)
	}

	if _, err := host.Decrypt(ciphertext); err != nil {
		t.Fatalf("first decrypt: %v", err)
	}

	if _, err := host.Decrypt(ciphertext); err == nil {
		t.Fatalf("expected replayed ciphertext to fail")
	}
}
```

- [ ] **Step 2: Run failure-case tests to verify they fail**

Run:

```bash
go test ./internal/securechannel -run 'TestHandshakeFails|TestDecryptRejectsReplayedCiphertext' -count=1
```

Expected: FAIL because `EstablishMismatchedTestChannel` and `EstablishMismatchedHostKeyTestChannel` are undefined.

- [ ] **Step 3: Add mismatch establishment helpers for failure tests**

Modify `internal/securechannel/channel.go` so it includes these helpers:

```go
func EstablishMismatchedTestChannel(clientCfg HandshakeConfig, hostCfg HandshakeConfig, hostKey HostKeypair) (*Channel, *Channel, error) {
	return establishNKpsk0WithConfigs(clientCfg, hostCfg, hostKey.private, hostKey.Public)
}

func EstablishMismatchedHostKeyTestChannel(cfg HandshakeConfig, hostKey HostKeypair, expectedHostPublic []byte) (*Channel, *Channel, error) {
	return establishNKpsk0(cfg, hostKey.private, expectedHostPublic)
}
```

Refactor the existing `establishNKpsk0` to call this helper:

```go
func establishNKpsk0(cfg HandshakeConfig, hostKey noise.DHKey, expectedHostPublic []byte) (*Channel, *Channel, error) {
	return establishNKpsk0WithConfigs(cfg, cfg, hostKey, expectedHostPublic)
}
```

Add this helper below `establishNKpsk0`:

```go
func establishNKpsk0WithConfigs(clientCfg HandshakeConfig, hostCfg HandshakeConfig, hostKey noise.DHKey, expectedHostPublic []byte) (*Channel, *Channel, error) {
	if len(expectedHostPublic) == 0 {
		return nil, nil, fmt.Errorf("%w: expected host public key is required", ErrHostKeyMismatch)
	}

	clientPrologue, err := BuildPrologue(NewPrologueConfig(clientCfg))
	if err != nil {
		return nil, nil, err
	}
	hostPrologue, err := BuildPrologue(NewPrologueConfig(hostCfg))
	if err != nil {
		return nil, nil, err
	}

	clientNoiseCfg := noise.Config{
		CipherSuite: noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s),
		Pattern: noise.HandshakeNK,
		Initiator: true,
		Prologue: clientPrologue,
		PresharedKey: clientCfg.ClientSecret[:],
		PresharedKeyPlacement: 0,
		PeerStatic: expectedHostPublic,
	}

	clientHS, err := noise.NewHandshakeState(clientNoiseCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: create client handshake: %w", ErrHandshakeFailed, err)
	}

	hostNoiseCfg := clientNoiseCfg
	hostNoiseCfg.Initiator = false
	hostNoiseCfg.Prologue = hostPrologue
	hostNoiseCfg.PresharedKey = hostCfg.ClientSecret[:]
	hostNoiseCfg.StaticKeypair = hostKey
	hostNoiseCfg.PeerStatic = nil

	hostHS, err := noise.NewHandshakeState(hostNoiseCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: create host handshake: %w", ErrHandshakeFailed, err)
	}

	msg1, _, _, err := clientHS.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: client write handshake: %w", ErrHandshakeFailed, err)
	}

	if _, _, _, err := hostHS.ReadMessage(nil, msg1); err != nil {
		return nil, nil, fmt.Errorf("%w: host read handshake: %w", ErrHandshakeFailed, err)
	}

	msg2, hostSend, hostRecv, err := hostHS.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: host write handshake: %w", ErrHandshakeFailed, err)
	}

	_, clientSend, clientRecv, err := clientHS.ReadMessage(nil, msg2)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: client read handshake: %w", ErrHandshakeFailed, err)
	}

	if subtle.ConstantTimeCompare(hostKey.Public, expectedHostPublic) != 1 {
		return nil, nil, ErrHostKeyMismatch
	}

	return &Channel{send: clientSend, recv: clientRecv}, &Channel{send: hostSend, recv: hostRecv}, nil
}
```

- [ ] **Step 4: Run failure-case tests to verify they pass**

Run:

```bash
go test ./internal/securechannel -run 'TestHandshakeFails|TestDecryptRejectsReplayedCiphertext' -count=1
```

Expected: PASS.

- [ ] **Step 5: Run all securechannel tests**

Run:

```bash
go test ./internal/securechannel -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit failure-case coverage**

Run:

```bash
git add internal/securechannel/channel.go internal/securechannel/channel_test.go
git commit -m "test: cover secure channel failure cases"
```

If the workspace is not a Git repository, record that in the task notes and continue without committing.

## Task 6: Compare XXpsk3 And Record Viability

**Files:**
- Modify: `internal/securechannel/channel_test.go`

- [ ] **Step 1: Add an explicit XXpsk3 viability test**

Append to `internal/securechannel/channel_test.go`:

```go
func TestXXpsk3PatternIsAvailableForFallbackEvaluation(t *testing.T) {
	if noise.HandshakeXX.Name != "XX" {
		t.Fatalf("noise.HandshakeXX is not available")
	}

	cfg := noise.Config{
		CipherSuite: noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s),
		Pattern: noise.HandshakeXX,
		Initiator: true,
		Prologue: []byte("OpenTunnel XXpsk3 availability check"),
		PresharedKey: bytes.Repeat([]byte{7}, ClientSecretSize),
		PresharedKeyPlacement: 3,
	}

	if _, err := noise.NewHandshakeState(cfg); err != nil {
		t.Fatalf("XXpsk3 handshake state should be constructible: %v", err)
	}
}
```

- [ ] **Step 2: Run XXpsk3 availability test**

Run:

```bash
go test ./internal/securechannel -run TestXXpsk3PatternIsAvailableForFallbackEvaluation -count=1
```

Expected: PASS if `github.com/flynn/noise` can construct XX with PSK placement 3. If this fails, document the failure text in the decision document in Task 7.

- [ ] **Step 3: Commit fallback comparison test**

Run:

```bash
git add internal/securechannel/channel_test.go
git commit -m "test: check XXpsk3 fallback viability"
```

If the workspace is not a Git repository, record that in the task notes and continue without committing.

## Task 7: Document Secure Channel Decision

**Files:**
- Create: `docs/superpowers/spikes/opentunnel-secure-channel-decision.md`

- [ ] **Step 1: Create spike documentation directory**

Run:

```bash
mkdir -p docs/superpowers/spikes
```

Expected: command exits 0.

- [ ] **Step 2: Write the decision document**

Create `docs/superpowers/spikes/opentunnel-secure-channel-decision.md`:

```markdown
# OpenTunnel Secure Channel Spike Decision

## Decision

OpenTunnel v1 will use `Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s` for the first implementation milestone if the test suite in `internal/securechannel` passes against `github.com/flynn/noise`.

## Rationale

The client already receives the host session public key inside the opaque invite code, and the client has no durable identity in v1. `NKpsk0` maps directly to that model:

- client is anonymous,
- host has a per-session static key,
- client knows the host public key before the handshake,
- the invite's 32-byte `clientSecret` is mixed as the PSK,
- canonical OpenTunnel session context is bound through the prologue.

## Required Properties Verified By Tests

- 32-byte `clientSecret` PSK is accepted.
- Host session public key is verified against invite material.
- Prologue binding changes when session security context changes.
- Multiple encrypted frames can be exchanged after handshake.
- Wrong `clientSecret` fails.
- Wrong `hostPubKey` fails.
- Wrong prologue fails.
- Replayed ciphertext fails.

## XXpsk3 Fallback Evaluation

`Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s` remains the fallback only if `NKpsk0` cannot be implemented cleanly with the Go library. The fallback is less direct for v1 because the client already knows the host session public key from the invite, but it may be acceptable if Go library support is materially clearer.

## Gate Result

Gate 1 passes only when `go test ./internal/securechannel -count=1` passes and this document reflects the actual selected pattern.
```

If the spike selected `XXpsk3` instead, change only the decision and rationale text to say why `NKpsk0` was rejected. Keep the verified-property list aligned with the actual tests.

- [ ] **Step 3: Commit decision document**

Run:

```bash
git add docs/superpowers/spikes/opentunnel-secure-channel-decision.md
git commit -m "docs: record secure channel spike decision"
```

If the workspace is not a Git repository, record that in the task notes and continue without committing.

## Task 8: Final Gate Verification

**Files:**
- Verify: `go.mod`
- Verify: `go.sum`
- Verify: `internal/securechannel/doc.go`
- Verify: `internal/securechannel/types.go`
- Verify: `internal/securechannel/prologue.go`
- Verify: `internal/securechannel/prologue_test.go`
- Verify: `internal/securechannel/channel.go`
- Verify: `internal/securechannel/channel_test.go`
- Verify: `docs/superpowers/spikes/opentunnel-secure-channel-decision.md`

- [ ] **Step 1: Format Go files**

Run:

```bash
gofmt -w internal/securechannel/*.go
```

Expected: command exits 0.

- [ ] **Step 2: Tidy module files**

Run:

```bash
go mod tidy
```

Expected: command exits 0 and keeps `go.mod` plus `go.sum` consistent with imports.

- [ ] **Step 3: Run package tests**

Run:

```bash
go test ./internal/securechannel -count=1
```

Expected: PASS.

- [ ] **Step 4: Run all module tests**

Run:

```bash
go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 5: Run race detector for securechannel package**

Run:

```bash
go test -race ./internal/securechannel -count=1
```

Expected: PASS.

- [ ] **Step 6: Verify the decision document matches the tests**

Open `docs/superpowers/spikes/opentunnel-secure-channel-decision.md` and confirm it states the same selected pattern as `SelectedPattern` in `internal/securechannel/types.go`.

Expected: both name `Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s`, unless the spike explicitly failed NKpsk0 and selected XXpsk3 with documented rationale.

- [ ] **Step 7: Commit final formatting or documentation fixes**

Run:

```bash
git add go.mod go.sum internal/securechannel docs/superpowers/spikes/opentunnel-secure-channel-decision.md
git commit -m "test: verify secure channel spike gate"
```

If the workspace is not a Git repository, record that in the task notes and continue without committing.

## Self-Review Checklist

- Milestone 1 scope is covered: Noise pattern comparison, PSK, host key verification, prologue, encrypted frames, failure cases, and interface boundary.
- Later milestones are excluded: no relay, CLI UX, command execution, `/cli`, caching, artifact serving, or product logging.
- The main implementation path is never plaintext.
- The plan has an explicit failure path if `github.com/flynn/noise` cannot support the required NKpsk0 behavior.
- Every test command has an expected result.
- Every code step includes concrete code.
