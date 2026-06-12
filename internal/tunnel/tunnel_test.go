package tunnel

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"opentunnel/internal/command"
	"opentunnel/internal/invite"
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
	rogueConn, _, err := websocket.DefaultDialer.Dial(tunnelEndpoint(parsedRelayURL), tunnelHeader("client", payload.SessionID))
	if err != nil {
		t.Fatalf("connect rogue client relay websocket: %v", err)
	}
	if err := rogueConn.WriteMessage(websocket.BinaryMessage, []byte("not a noise handshake")); err != nil {
		_ = rogueConn.Close()
		t.Fatalf("write rogue client handshake: %v", err)
	}
	if err := rogueConn.Close(); err != nil {
		t.Fatalf("close rogue client connection: %v", err)
	}

	var stdout bytes.Buffer
	deadline := time.Now().Add(time.Second)
	for {
		stdout.Reset()
		_, err = Exec(ctx, ExecConfig{
			Invite:  session.Invite,
			Command: "printf alive",
			Stdout:  &stdout,
		})
		if err == nil {
			break
		}
		select {
		case hostErr := <-session.Done:
			t.Fatalf("host stopped after rogue client handshake failure: %v", hostErr)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("exec after rogue client handshake failure: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if stdout.String() != "alive" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "alive")
	}

	select {
	case err := <-session.Done:
		t.Fatalf("host stopped after legitimate exec following rogue client: %v", err)
	default:
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

func TestExecCommandTimeout(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{
		RelayURL:       relayURL(server.URL),
		CommandTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	result, err := Exec(ctx, ExecConfig{
		Invite:  session.Invite,
		Command: "sleep 2",
	})
	if err == nil {
		t.Fatal("exec returned nil error, want command timeout error")
	}
	if result.ExitCode != 1 {
		t.Fatalf("exit code = %d, want 1", result.ExitCode)
	}
	if !strings.Contains(err.Error(), string(ErrorTypeCommandTimeout)) {
		t.Fatalf("exec error = %v, want %s", err, ErrorTypeCommandTimeout)
	}

	// M3 treats command timeout as a host connection failure after the encrypted
	// error frame is sent; future milestones may keep the session alive.
	select {
	case <-session.Done:
	case <-time.After(time.Second):
		t.Fatal("host did not stop after command timeout")
	}
}

func TestExecMaxOutputExceeded(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{
		RelayURL:       relayURL(server.URL),
		MaxOutputBytes: 5,
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	var stdout bytes.Buffer
	result, err := Exec(ctx, ExecConfig{
		Invite:  session.Invite,
		Command: "printf 123456789",
		Stdout:  &stdout,
	})
	if err == nil {
		t.Fatal("exec returned nil error, want max output exceeded error")
	}
	if stdout.String() != "12345" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "12345")
	}
	if result.ExitCode == 0 {
		t.Fatalf("exit code = %d, want non-zero", result.ExitCode)
	}
	if !strings.Contains(err.Error(), string(ErrorTypeMaxOutputExceeded)) {
		t.Fatalf("exec error = %v, want %s", err, ErrorTypeMaxOutputExceeded)
	}
}

func TestExecMaxOutputExceededCancelsCommandPromptly(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{
		RelayURL:       relayURL(server.URL),
		MaxOutputBytes: 5,
		CommandTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	started := time.Now()
	result, err := Exec(ctx, ExecConfig{
		Invite:  session.Invite,
		Command: "printf 123456789; sleep 2",
	})
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("exec returned nil error, want max output exceeded error")
	}
	if result.ExitCode == 0 {
		t.Fatalf("exit code = %d, want non-zero", result.ExitCode)
	}
	if !strings.Contains(err.Error(), string(ErrorTypeMaxOutputExceeded)) {
		t.Fatalf("exec error = %v, want %s", err, ErrorTypeMaxOutputExceeded)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("exec returned after %s, want prompt output-limit cancellation", elapsed)
	}

	select {
	case <-session.Done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("host command continued running after output limit was exceeded")
	}
}

func TestClientDisconnectCancelsSilentCommandPromptly(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{
		RelayURL:       relayURL(server.URL),
		CommandTimeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	marker := t.TempDir() + "/disconnected-command-marker"
	conn, channel := connectTestClient(t, session.Invite)
	request, err := encryptJSON(channel, message{Type: commandRequest, Command: "sleep 2; printf done > " + marker})
	if err != nil {
		_ = conn.Close()
		t.Fatalf("encrypt command request: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, request); err != nil {
		_ = conn.Close()
		t.Fatalf("write command request: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("close client connection: %v", err)
	}

	started := time.Now()
	var stdout bytes.Buffer
	var result ExecResult
	deadline := time.Now().Add(700 * time.Millisecond)
	for {
		stdout.Reset()
		result, err = Exec(ctx, ExecConfig{
			Invite:  session.Invite,
			Command: "printf ready",
			Stdout:  &stdout,
		})
		if err == nil {
			break
		}
		select {
		case hostErr := <-session.Done:
			t.Fatalf("host stopped before reconnect after disconnect: %v", hostErr)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("second exec after disconnect: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if elapsed := time.Since(started); elapsed > 700*time.Millisecond {
		t.Fatalf("second exec completed after %s, want prompt disconnect cancellation", elapsed)
	}
	if stdout.String() != "ready" {
		t.Fatalf("stdout = %q, want %q", stdout.String(), "ready")
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", result.ExitCode)
	}

	time.Sleep(2100 * time.Millisecond)
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker stat error = %v, want marker not created", err)
	}
}

func TestHostIdleTimeout(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{
		RelayURL:    relayURL(server.URL),
		IdleTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	select {
	case err := <-session.Done:
		if err == nil {
			t.Fatal("host stopped without error, want idle timeout error")
		}
		if !strings.Contains(err.Error(), string(ErrorTypeIdleSessionTimeout)) {
			t.Fatalf("host error = %v, want %s", err, ErrorTypeIdleSessionTimeout)
		}
	case <-time.After(time.Second):
		t.Fatal("host did not stop after idle timeout")
	}
}

func TestHostIdleTimeoutNotResetByRogueClientHandshakeFailure(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{
		RelayURL:    relayURL(server.URL),
		IdleTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	payload, err := invite.Decode(session.Invite)
	if err != nil {
		t.Fatalf("decode invite: %v", err)
	}
	parsedRelayURL, err := parseRelayURL(payload.Relay)
	if err != nil {
		t.Fatalf("parse relay url: %v", err)
	}
	rogueConn, _, err := websocket.DefaultDialer.Dial(tunnelEndpoint(parsedRelayURL), tunnelHeader("client", payload.SessionID))
	if err != nil {
		t.Fatalf("connect rogue client relay websocket: %v", err)
	}
	if err := rogueConn.WriteMessage(websocket.BinaryMessage, []byte("not a noise handshake")); err != nil {
		_ = rogueConn.Close()
		t.Fatalf("write rogue client handshake: %v", err)
	}
	if err := rogueConn.Close(); err != nil {
		t.Fatalf("close rogue client connection: %v", err)
	}

	select {
	case err := <-session.Done:
		if err == nil {
			t.Fatal("host stopped without error, want idle timeout error")
		}
		if !strings.Contains(err.Error(), string(ErrorTypeIdleSessionTimeout)) {
			t.Fatalf("host error = %v, want %s", err, ErrorTypeIdleSessionTimeout)
		}
	case <-time.After(700 * time.Millisecond):
		t.Fatal("host did not stop after original idle timeout")
	}
}

func TestHostLocalStatusLogs(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logs bytes.Buffer
	session, err := StartHost(ctx, HostConfig{
		RelayURL:  relayURL(server.URL),
		LogWriter: &logs,
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	var stdout bytes.Buffer
	result, err := Exec(ctx, ExecConfig{
		Invite:  session.Invite,
		Command: "printf logged",
		Stdout:  &stdout,
	})
	if err != nil {
		t.Fatalf("exec: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", result.ExitCode)
	}
	cancel()

	select {
	case <-session.Done:
	case <-time.After(time.Second):
		t.Fatal("host did not stop after cancel")
	}

	logText := logs.String()
	for _, want := range []string{
		"opentunnel",
		"event=sessionOpen",
		"event=waiting",
		"event=clientConnected",
		"event=commandStart",
		"event=commandFinish",
		"event=sessionClose",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("logs missing %q in:\n%s", want, logText)
		}
	}
	if strings.Contains(logText, session.Invite) {
		t.Fatalf("logs include invite %q in:\n%s", session.Invite, logText)
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

func TestHostIdleTimeoutRestartsAfterCommandWhenClientKeepsSocketOpen(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	hostCtx, cancelHost := context.WithCancel(context.Background())
	defer cancelHost()

	session, err := StartHost(hostCtx, HostConfig{
		RelayURL:    relayURL(server.URL),
		IdleTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	conn, channel := connectTestClient(t, session.Invite)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("close client connection: %v", err)
		}
	}()

	request, err := encryptJSON(channel, message{Type: commandRequest, Command: "printf ok"})
	if err != nil {
		t.Fatalf("encrypt command request: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, request); err != nil {
		t.Fatalf("write command request: %v", err)
	}

	sawOutput := false
	for {
		_, encrypted, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read tunnel message: %v", err)
		}
		msg, err := decryptJSON(channel, encrypted)
		if err != nil {
			t.Fatalf("decrypt tunnel message: %v", err)
		}

		if msg.Type == output {
			sawOutput = string(msg.Data) == "ok"
			continue
		}
		if msg.Type != exit {
			t.Fatalf("message type = %q, want exit", msg.Type)
		}
		if msg.ExitCode != 0 {
			t.Fatalf("exit code = %d, want 0", msg.ExitCode)
		}
		break
	}
	if !sawOutput {
		t.Fatal("did not receive expected command output")
	}

	select {
	case err := <-session.Done:
		if err == nil {
			t.Fatal("host stopped without error, want idle timeout error")
		}
		if !strings.Contains(err.Error(), string(ErrorTypeIdleSessionTimeout)) {
			t.Fatalf("host error = %v, want %s", err, ErrorTypeIdleSessionTimeout)
		}
	case <-time.After(time.Second):
		t.Fatal("host did not idle timeout after command exit")
	}
}

func TestHostSendsCommandStartFailedErrorForRunFailure(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{RelayURL: relayURL(server.URL)})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	conn, channel := connectTestClient(t, session.Invite)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("close client connection: %v", err)
		}
	}()

	request, err := encryptJSON(channel, message{Type: commandRequest, Command: " "})
	if err != nil {
		t.Fatalf("encrypt command request: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, request); err != nil {
		t.Fatalf("write command request: %v", err)
	}

	msg := readEncryptedTestMessage(t, conn, channel)
	if msg.Type != errorMessage {
		t.Fatalf("message type = %q, want error", msg.Type)
	}
	if msg.ErrorType != ErrorTypeCommandStartFailed {
		t.Fatalf("error type = %q, want %q", msg.ErrorType, ErrorTypeCommandStartFailed)
	}
}

func TestHostSendsProtocolErrorForMalformedEncryptedMessage(t *testing.T) {
	server := httptest.NewServer(relay.NewServer().Handler())
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session, err := StartHost(ctx, HostConfig{RelayURL: relayURL(server.URL)})
	if err != nil {
		t.Fatalf("start host: %v", err)
	}

	conn, channel := connectTestClient(t, session.Invite)
	defer func() {
		if err := conn.Close(); err != nil {
			t.Fatalf("close client connection: %v", err)
		}
	}()

	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("not encrypted tunnel json")); err != nil {
		t.Fatalf("write malformed message: %v", err)
	}

	msg := readEncryptedTestMessage(t, conn, channel)
	if msg.Type != errorMessage {
		t.Fatalf("message type = %q, want error", msg.Type)
	}
	if msg.ErrorType != ErrorTypeProtocol {
		t.Fatalf("error type = %q, want %q", msg.ErrorType, ErrorTypeProtocol)
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

func TestSemanticErrorTypes(t *testing.T) {
	values := map[ErrorType]string{
		ErrorTypeHostUnavailable:        "HostUnavailableError",
		ErrorTypeClientAlreadyConnected: "ClientAlreadyConnectedError",
		ErrorTypeHandshakeFailed:        "HandshakeFailedError",
		ErrorTypeCommandAlreadyRunning:  "CommandAlreadyRunningError",
		ErrorTypeCommandTimeout:         "CommandTimeoutError",
		ErrorTypeMaxOutputExceeded:      "MaxOutputExceededError",
		ErrorTypeCommandStartFailed:     "CommandStartFailedError",
		ErrorTypeIdleSessionTimeout:     "IdleSessionTimeoutError",
		ErrorTypeProtocol:               "ProtocolError",
	}

	for got, want := range values {
		if string(got) != want {
			t.Fatalf("error type = %q, want %q", got, want)
		}
	}
}

func TestEncryptedErrorMessageRoundTrip(t *testing.T) {
	client, host := testChannels(t)
	want := message{
		Type:      "error",
		ErrorType: ErrorTypeCommandAlreadyRunning,
		Message:   "Another command is already running for this tunnel.",
	}

	ciphertext, err := encryptJSON(client, want)
	if err != nil {
		t.Fatalf("encrypt error message: %v", err)
	}

	got, err := decryptJSON(host, ciphertext)
	if err != nil {
		t.Fatalf("decrypt error message: %v", err)
	}

	if got.Type != want.Type {
		t.Fatalf("type = %q, want %q", got.Type, want.Type)
	}
	if got.ErrorType != want.ErrorType {
		t.Fatalf("error type = %q, want %q", got.ErrorType, want.ErrorType)
	}
	if got.Message != want.Message {
		t.Fatalf("message = %q, want %q", got.Message, want.Message)
	}
}

func relayURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func connectTestClient(t *testing.T, inviteCode string) (*websocket.Conn, *securechannel.Channel) {
	t.Helper()

	payload, err := invite.Decode(inviteCode)
	if err != nil {
		t.Fatalf("decode invite: %v", err)
	}
	relayURL, err := parseRelayURL(payload.Relay)
	if err != nil {
		t.Fatalf("parse relay url: %v", err)
	}
	conn, _, err := websocket.DefaultDialer.Dial(tunnelEndpoint(relayURL), tunnelHeader("client", payload.SessionID))
	if err != nil {
		t.Fatalf("connect client relay websocket: %v", err)
	}

	handshake, err := securechannel.NewClientHandshake(handshakeConfig(payload.SessionID, payload.Relay, payload.ClientSecret), payload.HostPublicKey)
	if err != nil {
		_ = conn.Close()
		t.Fatalf("new client handshake: %v", err)
	}
	msg1, err := handshake.WriteMessage()
	if err != nil {
		_ = conn.Close()
		t.Fatalf("write handshake message: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, msg1); err != nil {
		_ = conn.Close()
		t.Fatalf("write client handshake: %v", err)
	}
	_, msg2, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		t.Fatalf("read host handshake: %v", err)
	}
	channel, err := handshake.ReadMessage(msg2)
	if err != nil {
		_ = conn.Close()
		t.Fatalf("read handshake message: %v", err)
	}
	return conn, channel
}

func readEncryptedTestMessage(t *testing.T, conn *websocket.Conn, channel *securechannel.Channel) message {
	t.Helper()

	_, encrypted, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read tunnel message: %v", err)
	}
	msg, err := decryptJSON(channel, encrypted)
	if err != nil {
		t.Fatalf("decrypt tunnel message: %v", err)
	}
	return msg
}

func testHostChannel(t *testing.T) *securechannel.Channel {
	t.Helper()

	_, host := testChannels(t)
	return host
}

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
		t.Fatalf("client write message: %v", err)
	}
	msg2, host, err := hostHS.ReadMessage(msg1)
	if err != nil {
		t.Fatalf("host read message: %v", err)
	}
	client, err := clientHS.ReadMessage(msg2)
	if err != nil {
		t.Fatalf("client read message: %v", err)
	}
	return client, host
}
