package securechannel

import (
	"bytes"
	"crypto/rand"
	"testing"
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

		reply := append([]byte("ack:"), frame...)
		replyCiphertext, err := host.Encrypt(reply)
		if err != nil {
			t.Fatalf("host encrypt: %v", err)
		}
		if bytes.Contains(replyCiphertext, reply) {
			t.Fatalf("reply ciphertext contains plaintext frame %q", reply)
		}

		replyPlaintext, err := client.Decrypt(replyCiphertext)
		if err != nil {
			t.Fatalf("client decrypt: %v", err)
		}
		if !bytes.Equal(replyPlaintext, reply) {
			t.Fatalf("reply plaintext mismatch: got %q want %q", replyPlaintext, reply)
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
		SessionID:      "stn_test",
		RelayOrigin:    "https://relay.example",
		ClientSecret:   secret,
		PermissionMode: "yolo",
		Features:       []string{"exec.v1", "stdoutStderr.v1"},
	}
}
