package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"

	"github.com/gorilla/websocket"

	"opentunnel/internal/command"
	"opentunnel/internal/invite"
	"opentunnel/internal/securechannel"
)

type HostConfig struct {
	RelayURL string
}

type HostSession struct {
	SessionID string
	Invite    string
	Done      <-chan error
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

	conn, _, err := websocket.DefaultDialer.Dial(tunnelEndpoint(relayURL, "host", sessionID), nil)
	if err != nil {
		return HostSession{}, fmt.Errorf("connect host relay websocket: %w", err)
	}

	done := make(chan error, 1)
	go runHost(ctx, conn, hostKey, clientSecret, relayURL.String(), sessionID, done)

	return HostSession{SessionID: sessionID, Invite: inviteCode, Done: done}, nil
}

func runHost(ctx context.Context, conn *websocket.Conn, hostKey securechannel.HostKeypair, clientSecret [securechannel.ClientSecretSize]byte, relay string, sessionID string, done chan<- error) {
	defer close(done)
	defer conn.Close()

	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	if err := handleOneCommand(ctx, conn, hostKey, clientSecret, relay, sessionID); err != nil && !errors.Is(ctx.Err(), context.Canceled) {
		done <- err
	}
}

func handleOneCommand(ctx context.Context, conn *websocket.Conn, hostKey securechannel.HostKeypair, clientSecret [securechannel.ClientSecretSize]byte, relay string, sessionID string) error {
	_, msg1, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read client handshake: %w", err)
	}

	handshake, err := securechannel.NewHostHandshake(handshakeConfig(sessionID, relay, clientSecret), hostKey)
	if err != nil {
		return err
	}
	msg2, channel, err := handshake.ReadMessage(msg1)
	if err != nil {
		return err
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
		return err
	}
	if request.Type != commandRequest || request.Command == "" {
		return fmt.Errorf("unexpected tunnel message type %q", request.Type)
	}

	// M2 supports one command per client connection; callers can start a new host for another command.
	result, err := command.Run(ctx, request.Command, func(chunk command.OutputChunk) {
		frame, frameErr := encryptJSON(channel, message{Type: output, Stream: chunk.Stream, Data: chunk.Data})
		if frameErr != nil {
			return
		}
		conn.WriteMessage(websocket.BinaryMessage, frame)
	})
	if err != nil {
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

func tunnelEndpoint(relayURL *url.URL, role string, sessionID string) string {
	endpoint := *relayURL
	endpoint.Path = "/tunnel"
	query := endpoint.Query()
	query.Set("role", role)
	query.Set("session", sessionID)
	endpoint.RawQuery = query.Encode()
	return endpoint.String()
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
