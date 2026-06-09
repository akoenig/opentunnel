package tunnel

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"opentunnel/internal/command"
	"opentunnel/internal/relay"
	"opentunnel/internal/securechannel"
)

func TestExecRunsCommandThroughEncryptedTunnel(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{RelayURL: relayURL(server.URL)})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}
	if session.SessionID == "" {
		t.Fatal("expected session id")
	}
	if session.Invite == "" {
		t.Fatal("expected invite")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result, err := Exec(ctx, ExecConfig{
		Invite:  session.Invite,
		Command: "printf hello",
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}

	if stdout.String() != "hello" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "hello")
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", result.ExitCode)
	}
}

func TestStartHostSessionRunsSequentialExecsWithSameInvite(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{RelayURL: relayURL(server.URL)})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	var firstStdout bytes.Buffer
	firstResult, err := Exec(ctx, ExecConfig{
		Invite:  session.Invite,
		Command: "printf one",
		Stdout:  &firstStdout,
	})
	if err != nil {
		t.Fatalf("first exec: %v", err)
	}
	if firstStdout.String() != "one" {
		t.Fatalf("first stdout = %q, want %q", firstStdout.String(), "one")
	}
	if firstResult.ExitCode != 0 {
		t.Fatalf("first exit code = %d, want 0", firstResult.ExitCode)
	}

	select {
	case err := <-session.Done:
		t.Fatalf("host stopped after first exec: %v", err)
	default:
	}

	var secondStdout bytes.Buffer
	var secondResult ExecResult
	deadline := time.Now().Add(time.Second)
	for {
		secondStdout.Reset()
		secondResult, err = Exec(ctx, ExecConfig{
			Invite:  session.Invite,
			Command: "printf two",
			Stdout:  &secondStdout,
		})
		if err == nil {
			break
		}
		select {
		case hostErr := <-session.Done:
			t.Fatalf("host stopped before second exec: %v", hostErr)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("second exec: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if secondStdout.String() != "two" {
		t.Fatalf("second stdout = %q, want %q", secondStdout.String(), "two")
	}
	if secondResult.ExitCode != 0 {
		t.Fatalf("second exit code = %d, want 0", secondResult.ExitCode)
	}
}

func TestExecReturnsNonZeroExitCodeWithoutError(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{RelayURL: relayURL(server.URL)})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	result, err := Exec(ctx, ExecConfig{
		Invite:  session.Invite,
		Command: "exit 7",
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7", result.ExitCode)
	}
}

func TestExecStreamsStderrThroughEncryptedTunnel(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{RelayURL: relayURL(server.URL)})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	result, err := Exec(ctx, ExecConfig{
		Invite:  session.Invite,
		Command: "printf err >&2",
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}

	if stdout.String() != "" {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.String() != "err" {
		t.Fatalf("stderr = %q, want %q", stderr.String(), "err")
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", result.ExitCode)
	}
}

func TestOutputSenderRecordsFirstErrorAndSkipsLaterWrites(t *testing.T) {
	channel := testHostChannel(t)
	sender := outputSender{}
	writeErr := errors.New("relay write failed")
	writes := 0

	sender.send(channel, func(int, []byte) error {
		writes++
		return writeErr
	}, command.OutputChunk{Stream: "stdout", Data: []byte("first")})
	sender.send(channel, func(int, []byte) error {
		writes++
		return nil
	}, command.OutputChunk{Stream: "stdout", Data: []byte("second")})

	if !errors.Is(sender.err(), writeErr) {
		t.Fatalf("output send error = %v, want %v", sender.err(), writeErr)
	}
	if writes != 1 {
		t.Fatalf("writes = %d, want 1", writes)
	}
}

func relayURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func testHostChannel(t *testing.T) *securechannel.Channel {
	t.Helper()

	var clientSecret [securechannel.ClientSecretSize]byte
	for i := range clientSecret {
		clientSecret[i] = byte(i + 1)
	}
	hostKey, err := securechannel.GenerateHostKeypair(strings.NewReader(strings.Repeat("a", 64)))
	if err != nil {
		t.Fatalf("generate host keypair: %v", err)
	}
	_, host, err := securechannel.EstablishChannelWithHostKey(
		handshakeConfig("test-session", "ws://relay.example", clientSecret),
		hostKey,
		hostKey.Public,
	)
	if err != nil {
		t.Fatalf("establish channel: %v", err)
	}
	return host
}
