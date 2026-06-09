package invite

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const prefix = "ot1_"

// Payload contains the data carried by an opaque invite code.
type Payload struct {
	Relay         string
	SessionID     string
	HostPublicKey []byte
	ClientSecret  [32]byte
}

type encodedPayload struct {
	Relay         string   `json:"relay"`
	SessionID     string   `json:"sid"`
	HostPublicKey []byte   `json:"hpk"`
	ClientSecret  [32]byte `json:"cs"`
}

// Encode validates payload and returns an opaque invite code.
func Encode(payload Payload) (string, error) {
	encoded := encodedPayload{
		Relay:         payload.Relay,
		SessionID:     payload.SessionID,
		HostPublicKey: copyBytes(payload.HostPublicKey),
		ClientSecret:  payload.ClientSecret,
	}

	if err := validate(encoded); err != nil {
		return "", err
	}

	data, err := json.Marshal(encoded)
	if err != nil {
		return "", fmt.Errorf("encode invite payload: %w", err)
	}

	return prefix + base64.RawURLEncoding.EncodeToString(data), nil
}

// Decode validates and decodes an opaque invite code.
func Decode(code string) (Payload, error) {
	encodedCode, ok := strings.CutPrefix(code, prefix)
	if !ok {
		return Payload{}, errors.New("invite code missing ot1_ prefix")
	}

	data, err := base64.RawURLEncoding.DecodeString(encodedCode)
	if err != nil {
		return Payload{}, fmt.Errorf("decode invite payload base64: %w", err)
	}

	var encoded encodedPayload
	if err := json.Unmarshal(data, &encoded); err != nil {
		return Payload{}, fmt.Errorf("decode invite payload json: %w", err)
	}

	if err := validate(encoded); err != nil {
		return Payload{}, err
	}

	return Payload{
		Relay:         encoded.Relay,
		SessionID:     encoded.SessionID,
		HostPublicKey: copyBytes(encoded.HostPublicKey),
		ClientSecret:  encoded.ClientSecret,
	}, nil
}

func validate(payload encodedPayload) error {
	if payload.Relay == "" {
		return errors.New("invite relay is required")
	}
	if payload.SessionID == "" {
		return errors.New("invite session id is required")
	}
	if len(payload.HostPublicKey) != 32 {
		return errors.New("invite host public key must be 32 bytes")
	}
	if isZeroSecret(payload.ClientSecret) {
		return errors.New("invite client secret must not be all zero")
	}

	return nil
}

func isZeroSecret(secret [32]byte) bool {
	for _, value := range secret {
		if value != 0 {
			return false
		}
	}

	return true
}

func copyBytes(data []byte) []byte {
	if data == nil {
		return nil
	}

	copied := make([]byte, len(data))
	copy(copied, data)
	return copied
}
