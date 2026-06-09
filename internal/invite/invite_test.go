package invite

import (
	"bytes"
	"strings"
	"testing"
)

func TestEncodeReturnsOT1Prefix(t *testing.T) {
	code, err := Encode(testPayload())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	if !strings.HasPrefix(code, "ot1_") {
		t.Fatalf("encoded invite missing ot1_ prefix: %q", code)
	}
}

func TestDecodeRoundTripsEncodedPayload(t *testing.T) {
	payload := testPayload()

	code, err := Encode(payload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	decoded, err := Decode(code)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if decoded.Relay != payload.Relay {
		t.Fatalf("Relay mismatch: got %q want %q", decoded.Relay, payload.Relay)
	}
	if decoded.SessionID != payload.SessionID {
		t.Fatalf("SessionID mismatch: got %q want %q", decoded.SessionID, payload.SessionID)
	}
	if !bytes.Equal(decoded.HostPublicKey, payload.HostPublicKey) {
		t.Fatalf("HostPublicKey mismatch: got %x want %x", decoded.HostPublicKey, payload.HostPublicKey)
	}
	if decoded.ClientSecret != payload.ClientSecret {
		t.Fatalf("ClientSecret mismatch: got %x want %x", decoded.ClientSecret, payload.ClientSecret)
	}
}

func TestDecodeRejectsMissingPrefix(t *testing.T) {
	if _, err := Decode("not-an-ot1-invite"); err == nil {
		t.Fatalf("expected missing prefix to fail")
	}
}

func TestDecodeRejectsMalformedBase64Payload(t *testing.T) {
	if _, err := Decode("ot1_!!!"); err == nil {
		t.Fatalf("expected malformed base64 payload to fail")
	}
}

func TestEncodeRejectsEmptyRelay(t *testing.T) {
	payload := testPayload()
	payload.Relay = ""

	if _, err := Encode(payload); err == nil {
		t.Fatalf("expected empty Relay to fail")
	}
}

func TestEncodeRejectsEmptySessionID(t *testing.T) {
	payload := testPayload()
	payload.SessionID = ""

	if _, err := Encode(payload); err == nil {
		t.Fatalf("expected empty SessionID to fail")
	}
}

func TestEncodeRejectsWrongHostPublicKeyLength(t *testing.T) {
	payload := testPayload()
	payload.HostPublicKey = payload.HostPublicKey[:31]

	if _, err := Encode(payload); err == nil {
		t.Fatalf("expected wrong HostPublicKey length to fail")
	}
}

func TestEncodeRejectsAllZeroClientSecret(t *testing.T) {
	payload := testPayload()
	payload.ClientSecret = [32]byte{}

	if _, err := Encode(payload); err == nil {
		t.Fatalf("expected all-zero ClientSecret to fail")
	}
}

func testPayload() Payload {
	hostPublicKey := make([]byte, 32)
	for i := range hostPublicKey {
		hostPublicKey[i] = byte(i + 1)
	}

	var clientSecret [32]byte
	for i := range clientSecret {
		clientSecret[i] = byte(32 - i)
	}

	return Payload{
		Relay:         "https://relay.example",
		SessionID:     "session-123",
		HostPublicKey: hostPublicKey,
		ClientSecret:  clientSecret,
	}
}
