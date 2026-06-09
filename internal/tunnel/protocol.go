package tunnel

import (
	"encoding/json"
	"fmt"

	"opentunnel/internal/securechannel"
)

const (
	commandRequest = "commandRequest"
	output         = "output"
	exit           = "exit"
)

type message struct {
	Type     string `json:"type"`
	Command  string `json:"command,omitempty"`
	Stream   string `json:"stream,omitempty"`
	Data     []byte `json:"data,omitempty"`
	ExitCode int    `json:"exitCode,omitempty"`
}

func encryptJSON(channel *securechannel.Channel, msg message) ([]byte, error) {
	plaintext, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encode tunnel message: %w", err)
	}

	ciphertext, err := channel.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	return ciphertext, nil
}

func decryptJSON(channel *securechannel.Channel, ciphertext []byte) (message, error) {
	plaintext, err := channel.Decrypt(ciphertext)
	if err != nil {
		return message{}, err
	}

	var msg message
	if err := json.Unmarshal(plaintext, &msg); err != nil {
		return message{}, fmt.Errorf("decode tunnel message: %w", err)
	}
	return msg, nil
}
