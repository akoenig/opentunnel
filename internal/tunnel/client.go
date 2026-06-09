package tunnel

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/gorilla/websocket"

	"opentunnel/internal/invite"
	"opentunnel/internal/securechannel"
)

type ExecConfig struct {
	Invite  string
	Command string
	Stdout  io.Writer
	Stderr  io.Writer
}

type ExecResult struct {
	ExitCode int
}

func Exec(ctx context.Context, cfg ExecConfig) (ExecResult, error) {
	if strings.TrimSpace(cfg.Command) == "" {
		return ExecResult{}, fmt.Errorf("command must not be empty")
	}
	payload, err := invite.Decode(cfg.Invite)
	if err != nil {
		return ExecResult{}, fmt.Errorf("decode invite: %w", err)
	}

	relayURL, err := parseRelayURL(payload.Relay)
	if err != nil {
		return ExecResult{}, err
	}
	conn, _, err := websocket.DefaultDialer.Dial(tunnelEndpoint(relayURL, "client", payload.SessionID), nil)
	if err != nil {
		return ExecResult{}, fmt.Errorf("connect client relay websocket: %w", err)
	}
	defer conn.Close()

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()

	handshake, err := securechannel.NewClientHandshake(handshakeConfig(payload.SessionID, payload.Relay, payload.ClientSecret), payload.HostPublicKey)
	if err != nil {
		return ExecResult{}, err
	}
	msg1, err := handshake.WriteMessage()
	if err != nil {
		return ExecResult{}, err
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, msg1); err != nil {
		return ExecResult{}, fmt.Errorf("write client handshake: %w", err)
	}
	_, msg2, err := conn.ReadMessage()
	if err != nil {
		return ExecResult{}, fmt.Errorf("read host handshake: %w", err)
	}
	channel, err := handshake.ReadMessage(msg2)
	if err != nil {
		return ExecResult{}, err
	}

	request, err := encryptJSON(channel, message{Type: commandRequest, Command: cfg.Command})
	if err != nil {
		return ExecResult{}, err
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, request); err != nil {
		return ExecResult{}, fmt.Errorf("write command request: %w", err)
	}

	stdout := cfg.Stdout
	if stdout == nil {
		stdout = io.Discard
	}
	stderr := cfg.Stderr
	if stderr == nil {
		stderr = io.Discard
	}

	for {
		_, encrypted, err := conn.ReadMessage()
		if err != nil {
			return ExecResult{}, fmt.Errorf("read tunnel message: %w", err)
		}
		msg, err := decryptJSON(channel, encrypted)
		if err != nil {
			return ExecResult{}, err
		}

		switch msg.Type {
		case output:
			writer := stdout
			if msg.Stream == "stderr" {
				writer = stderr
			}
			if _, err := writer.Write(msg.Data); err != nil {
				return ExecResult{}, fmt.Errorf("write %s: %w", msg.Stream, err)
			}
		case exit:
			return ExecResult{ExitCode: msg.ExitCode}, nil
		case errorMessage:
			return ExecResult{ExitCode: 1}, fmt.Errorf("%s: %s", msg.ErrorType, msg.Message)
		default:
			return ExecResult{}, fmt.Errorf("unexpected tunnel message type %q", msg.Type)
		}
	}
}
