package tunnel

import (
	"bytes"
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"opentunnel/internal/relay"
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

func relayURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}
