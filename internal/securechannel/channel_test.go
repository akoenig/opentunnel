package securechannel

import (
	"bytes"
	"crypto/rand"
	"errors"
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

func TestSplitNKpsk0HandshakeMatchesInMemoryChannel(t *testing.T) {
	cfg := testHandshakeConfig(t)
	hostKey, err := GenerateHostKeypair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateHostKeypair: %v", err)
	}

	clientHS, err := NewClientHandshake(cfg, hostKey.Public)
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
	msg2, host, err := hostHS.ReadMessage(msg1)
	if err != nil {
		t.Fatalf("host read message: %v", err)
	}
	client, err := clientHS.ReadMessage(msg2)
	if err != nil {
		t.Fatalf("client read message: %v", err)
	}

	frame := []byte("commandRequest:hostname")
	ciphertext, err := client.Encrypt(frame)
	if err != nil {
		t.Fatalf("client encrypt: %v", err)
	}
	plaintext, err := host.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("host decrypt: %v", err)
	}
	if !bytes.Equal(plaintext, frame) {
		t.Fatalf("host plaintext mismatch: got %q want %q", plaintext, frame)
	}

	reply := []byte("stdoutData:api-staging")
	replyCiphertext, err := host.Encrypt(reply)
	if err != nil {
		t.Fatalf("host encrypt: %v", err)
	}
	replyPlaintext, err := client.Decrypt(replyCiphertext)
	if err != nil {
		t.Fatalf("client decrypt: %v", err)
	}
	if !bytes.Equal(replyPlaintext, reply) {
		t.Fatalf("client plaintext mismatch: got %q want %q", replyPlaintext, reply)
	}
}

func TestHandshakeFailsWithWrongClientSecret(t *testing.T) {
	clientCfg := testHandshakeConfig(t)
	hostCfg := clientCfg
	hostKey, err := GenerateHostKeypair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateHostKeypair: %v", err)
	}

	clientCfg.ClientSecret[0] ^= 0xff

	_, _, err = establishNKpsk0WithConfigs(clientCfg, hostCfg, hostKey.private, hostKey.Public)
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

	_, _, err = establishNKpsk0(cfg, hostKey.private, otherHostKey.Public)
	if err == nil {
		t.Fatalf("expected wrong host public key to fail")
	}
	if !errors.Is(err, ErrHostKeyMismatch) {
		t.Fatalf("expected ErrHostKeyMismatch, got %v", err)
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

	_, _, err = establishNKpsk0WithConfigs(clientCfg, hostCfg, hostKey.private, hostKey.Public)
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

func TestDecryptRejectsMalformedCiphertext(t *testing.T) {
	cfg := testHandshakeConfig(t)
	hostKey, err := GenerateHostKeypair(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateHostKeypair: %v", err)
	}

	client, host, err := EstablishChannelWithHostKey(cfg, hostKey, hostKey.Public)
	if err != nil {
		t.Fatalf("EstablishChannelWithHostKey: %v", err)
	}

	if _, err := host.Decrypt([]byte("not a valid noise ciphertext")); err == nil {
		t.Fatalf("expected malformed ciphertext to fail")
	}

	ciphertext, err := client.Encrypt([]byte("stdoutData:first"))
	if err != nil {
		t.Fatalf("client encrypt: %v", err)
	}

	tampered := append([]byte(nil), ciphertext...)
	tampered[len(tampered)-1] ^= 0xff
	if _, err := host.Decrypt(tampered); err == nil {
		t.Fatalf("expected tampered ciphertext to fail")
	}
}

func TestXXpsk3PatternIsAvailableForFallbackEvaluation(t *testing.T) {
	if noise.HandshakeXX.Name != "XX" {
		t.Fatalf("noise.HandshakeXX is not available")
	}

	cfg := noise.Config{
		CipherSuite:           noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s),
		Pattern:               noise.HandshakeXX,
		Initiator:             true,
		Prologue:              []byte("OpenTunnel XXpsk3 availability check"),
		PresharedKey:          bytes.Repeat([]byte{7}, ClientSecretSize),
		PresharedKeyPlacement: 3,
	}

	if _, err := noise.NewHandshakeState(cfg); err != nil {
		t.Fatalf("XXpsk3 handshake state should be constructible: %v", err)
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
