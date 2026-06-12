package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"opentunnel/internal/command"
	"opentunnel/internal/invite"
	"opentunnel/internal/securechannel"
)

const (
	defaultCommandTimeout = 120 * time.Second
	defaultIdleTimeout    = 30 * time.Minute
	defaultMaxOutputBytes = 10 * 1024 * 1024
)

const (
	tunnelRoleHeader    = "OpenTunnel-Role"
	tunnelSessionHeader = "OpenTunnel-Session"
)

var errIdleSessionTimeout = errors.New(string(ErrorTypeIdleSessionTimeout))

var errPreCommandClientFailure = errors.New("pre-command client failure")

type HostConfig struct {
	RelayURL       string
	CommandTimeout time.Duration
	IdleTimeout    time.Duration
	MaxOutputBytes int
	LogWriter      io.Writer
}

type HostSession struct {
	SessionID string
	Invite    string
	Done      <-chan error
}

type hostRuntime struct {
	conn           *websocket.Conn
	hostKey        securechannel.HostKeypair
	clientSecret   [securechannel.ClientSecretSize]byte
	relay          string
	sessionID      string
	commandTimeout time.Duration
	idleTimeout    time.Duration
	maxOutputBytes int
	logger         *hostLogger
	endpoint       string
	header         http.Header
}

func StartHost(ctx context.Context, cfg HostConfig) (HostSession, error) {
	relayURL, err := parseRelayURL(cfg.RelayURL)
	if err != nil {
		return HostSession{}, err
	}

	sessionID, err := generateSessionID()
	if err != nil {
		return HostSession{}, err
	}
	var clientSecret [securechannel.ClientSecretSize]byte
	if _, err := rand.Read(clientSecret[:]); err != nil {
		return HostSession{}, fmt.Errorf("generate client secret: %w", err)
	}

	hostKey, err := securechannel.GenerateHostKeypair(rand.Reader)
	if err != nil {
		return HostSession{}, err
	}

	inviteCode, err := invite.Encode(invite.Payload{
		Relay:         relayURL.String(),
		SessionID:     sessionID,
		HostPublicKey: hostKey.Public,
		ClientSecret:  clientSecret,
	})
	if err != nil {
		return HostSession{}, fmt.Errorf("encode invite: %w", err)
	}

	endpoint := tunnelEndpoint(relayURL)
	header := tunnelHeader("host", sessionID)
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, header)
	if err != nil {
		return HostSession{}, fmt.Errorf("connect host relay websocket: %w", err)
	}

	done := make(chan error, 1)
	logger := hostLogger{writer: cfg.LogWriter}
	runtime := hostRuntime{
		conn:           conn,
		hostKey:        hostKey,
		clientSecret:   clientSecret,
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

	return HostSession{SessionID: sessionID, Invite: inviteCode, Done: done}, nil
}

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

	conn := h.conn
	for {
		h.logger.log("waiting")
		resetIdle := true
		if err := handleOneHostConnection(hostCtx, conn, h.hostKey, h.clientSecret, h.relay, h.sessionID, h.commandTimeout, h.maxOutputBytes, h.logger, func() {
			stopTimer(idleTimer)
		}); err != nil && ctx.Err() == nil {
			if errors.Is(context.Cause(hostCtx), errIdleSessionTimeout) {
				done <- fmt.Errorf("%w: session idle timeout", errIdleSessionTimeout)
				return
			}
			if errors.Is(err, errPreCommandClientFailure) {
				h.logger.log("clientRejected")
				resetIdle = false
			} else {
				done <- err
				return
			}
		}
		if ctx.Err() != nil {
			return
		}
		if resetIdle {
			resetTimer(idleTimer, h.idleTimeout)
		}

		var err error
		conn, err = dialHostRelay(hostCtx, h.endpoint, h.header)
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
	}
}

func resetTimer(timer *time.Timer, timeout time.Duration) {
	stopTimer(timer)
	timer.Reset(timeout)
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func dialHostRelay(ctx context.Context, endpoint string, header http.Header) (*websocket.Conn, error) {
	for {
		conn, response, err := websocket.DefaultDialer.DialContext(ctx, endpoint, header)
		if err == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if response == nil || response.StatusCode != http.StatusConflict {
			return nil, err
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func handleOneHostConnection(ctx context.Context, conn *websocket.Conn, hostKey securechannel.HostKeypair, clientSecret [securechannel.ClientSecretSize]byte, relay string, sessionID string, commandTimeout time.Duration, maxOutputBytes int, logger *hostLogger, stopIdle func()) error {
	defer func() { _ = conn.Close() }()

	connectionDone := make(chan struct{})
	defer close(connectionDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-connectionDone:
		}
	}()

	return handleOneCommand(ctx, conn, hostKey, clientSecret, relay, sessionID, commandTimeout, maxOutputBytes, logger, stopIdle)
}

func handleOneCommand(ctx context.Context, conn *websocket.Conn, hostKey securechannel.HostKeypair, clientSecret [securechannel.ClientSecretSize]byte, relay string, sessionID string, commandTimeout time.Duration, maxOutputBytes int, logger *hostLogger, stopIdle func()) error {
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
	if err := conn.WriteMessage(websocket.BinaryMessage, msg2); err != nil {
		return fmt.Errorf("write host handshake: %w", err)
	}

	_, encryptedRequest, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read command request: %w", err)
	}
	request, err := decryptJSON(channel, encryptedRequest)
	if err != nil {
		if sendErr := sendError(channel, conn.WriteMessage, ErrorTypeProtocol, "Invalid encrypted tunnel message."); sendErr != nil {
			return sendErr
		}
		return err
	}
	if request.Type != commandRequest || request.Command == "" {
		err := fmt.Errorf("unexpected tunnel message type %q", request.Type)
		if sendErr := sendError(channel, conn.WriteMessage, ErrorTypeProtocol, "Unexpected tunnel message."); sendErr != nil {
			return sendErr
		}
		return err
	}
	stopIdle()

	// M2 supports one command per client connection; callers can start a new host for another command.
	sender := outputSender{maxOutputBytes: maxOutputBytes, logger: logger}
	commandCtx, cancel := context.WithTimeout(ctx, effectiveCommandTimeout(commandTimeout))
	defer cancel()
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()
	logger.log("commandStart")
	result, err := command.Run(commandCtx, request.Command, func(chunk command.OutputChunk) {
		sender.send(channel, conn.WriteMessage, chunk)
		if sender.err() != nil {
			cancel()
		}
	})
	if err != nil {
		if senderErr := sender.err(); senderErr != nil {
			return senderErr
		}
		if errors.Is(err, context.DeadlineExceeded) {
			logger.log("commandTimeout")
			if sendErr := sendError(channel, conn.WriteMessage, ErrorTypeCommandTimeout, "Command exceeded timeout."); sendErr != nil {
				return sendErr
			}
		} else if errors.Is(err, context.Canceled) && ctx.Err() == nil {
			return nil
		} else if ctx.Err() == nil {
			if sendErr := sendError(channel, conn.WriteMessage, ErrorTypeCommandStartFailed, "Command failed to start."); sendErr != nil {
				return sendErr
			}
		}
		return err
	}
	logger.log("commandFinish")
	if err := sender.err(); err != nil {
		return err
	}

	frame, err := encryptJSON(channel, message{Type: exit, ExitCode: result.ExitCode})
	if err != nil {
		return err
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		return fmt.Errorf("write exit: %w", err)
	}
	return nil
}

func effectiveCommandTimeout(commandTimeout time.Duration) time.Duration {
	if commandTimeout == 0 {
		return defaultCommandTimeout
	}
	return commandTimeout
}

func effectiveIdleTimeout(idleTimeout time.Duration) time.Duration {
	if idleTimeout == 0 {
		return defaultIdleTimeout
	}
	return idleTimeout
}

func effectiveMaxOutputBytes(maxOutputBytes int) int {
	if maxOutputBytes == 0 {
		return defaultMaxOutputBytes
	}
	return maxOutputBytes
}

func sendError(channel *securechannel.Channel, writeMessage func(int, []byte) error, errorType ErrorType, text string) error {
	frame, err := encryptJSON(channel, message{
		Type:      errorMessage,
		ErrorType: errorType,
		Message:   text,
	})
	if err != nil {
		return err
	}
	if err := writeMessage(websocket.BinaryMessage, frame); err != nil {
		return fmt.Errorf("write %s: %w", errorType, err)
	}
	return nil
}

func parseRelayURL(raw string) (*url.URL, error) {
	if raw == "" {
		return nil, fmt.Errorf("relay url is required")
	}
	relayURL, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse relay url: %w", err)
	}
	if relayURL.Scheme != "ws" && relayURL.Scheme != "wss" {
		return nil, fmt.Errorf("relay url must use ws or wss")
	}
	if relayURL.Host == "" {
		return nil, fmt.Errorf("relay url host is required")
	}
	return relayURL, nil
}

func generateSessionID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("generate session id: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}

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

func handshakeConfig(sessionID string, relay string, clientSecret [securechannel.ClientSecretSize]byte) securechannel.HandshakeConfig {
	return securechannel.HandshakeConfig{
		SessionID:      sessionID,
		RelayOrigin:    relay,
		ClientSecret:   clientSecret,
		PermissionMode: "execute",
		Features:       []string{"exec.v1", "stdoutStderr.v1"},
	}
}

type outputSender struct {
	mu             sync.Mutex
	sendErr        error
	maxOutputBytes int
	bytesSent      int
	logger         *hostLogger
}

func (s *outputSender) send(channel *securechannel.Channel, writeMessage func(int, []byte) error, chunk command.OutputChunk) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sendErr != nil {
		return
	}

	data := chunk.Data
	exceeded := false
	if s.maxOutputBytes > 0 {
		remaining := s.maxOutputBytes - s.bytesSent
		if remaining <= 0 {
			data = nil
			exceeded = true
		} else if len(data) > remaining {
			data = data[:remaining]
			exceeded = true
		}
	}

	if len(data) > 0 {
		frame, err := encryptJSON(channel, message{Type: output, Stream: chunk.Stream, Data: data})
		if err != nil {
			s.sendErr = fmt.Errorf("encrypt output: %w", err)
			return
		}
		if err := writeMessage(websocket.BinaryMessage, frame); err != nil {
			s.sendErr = fmt.Errorf("write output: %w", err)
			return
		}
		s.bytesSent += len(data)
	}

	if exceeded {
		s.sendMaxOutputExceeded(channel, writeMessage)
	}
}

func (s *outputSender) sendMaxOutputExceeded(channel *securechannel.Channel, writeMessage func(int, []byte) error) {
	s.logger.log("outputTruncated")
	frame, err := encryptJSON(channel, message{
		Type:      errorMessage,
		ErrorType: ErrorTypeMaxOutputExceeded,
		Message:   "Command output exceeded maximum size.",
	})
	if err != nil {
		s.sendErr = fmt.Errorf("encrypt max output exceeded: %w", err)
		return
	}
	if err := writeMessage(websocket.BinaryMessage, frame); err != nil {
		s.sendErr = fmt.Errorf("write max output exceeded: %w", err)
		return
	}
	s.sendErr = fmt.Errorf("%s: command output exceeded maximum size", ErrorTypeMaxOutputExceeded)
}

func (s *outputSender) err() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sendErr
}

type hostLogger struct {
	mu     sync.Mutex
	writer io.Writer
}

func (l *hostLogger) log(event string) {
	if l == nil || l.writer == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.writer, "opentunnel event=%s\n", event)
}
