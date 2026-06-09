package securechannel

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// BuildPrologue returns canonical length-prefixed bytes for the secure-channel handshake prologue.
func BuildPrologue(cfg PrologueConfig) ([]byte, error) {
	if cfg.App == "" {
		return nil, fmt.Errorf("build prologue: app is required")
	}
	if cfg.InviteVersion == 0 {
		return nil, fmt.Errorf("build prologue: invite version is required")
	}
	if cfg.NoiseProtocol == "" {
		return nil, fmt.Errorf("build prologue: noise protocol is required")
	}
	if cfg.SessionID == "" {
		return nil, fmt.Errorf("build prologue: session id is required")
	}
	if cfg.RelayOrigin == "" {
		return nil, fmt.Errorf("build prologue: relay origin is required")
	}
	if cfg.PermissionMode == "" {
		return nil, fmt.Errorf("build prologue: permission mode is required")
	}
	if len(cfg.Features) == 0 {
		return nil, fmt.Errorf("build prologue: at least one feature is required")
	}

	var buf bytes.Buffer
	writeField(&buf, []byte(cfg.App))
	writeField(&buf, []byte{cfg.InviteVersion})
	writeField(&buf, []byte(cfg.NoiseProtocol))
	writeField(&buf, []byte(cfg.SessionID))
	writeField(&buf, []byte(cfg.RelayOrigin))
	writeField(&buf, []byte(cfg.PermissionMode))
	writeUint32(&buf, uint32(cfg.CommandDefaults.TimeoutSeconds))
	writeUint32(&buf, uint32(cfg.CommandDefaults.MaxOutputBytes))
	if cfg.CommandDefaults.PTY {
		writeField(&buf, []byte{1})
	} else {
		writeField(&buf, []byte{0})
	}
	writeUint32(&buf, uint32(cfg.CommandDefaults.IdleSessionTimeoutSeconds))
	writeUint32(&buf, uint32(len(cfg.Features)))
	for _, feature := range cfg.Features {
		if feature == "" {
			return nil, fmt.Errorf("build prologue: feature cannot be empty")
		}
		writeField(&buf, []byte(feature))
	}

	return buf.Bytes(), nil
}

func writeField(buf *bytes.Buffer, value []byte) {
	writeUint32(buf, uint32(len(value)))
	buf.Write(value)
}

func writeUint32(buf *bytes.Buffer, value uint32) {
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], value)
	buf.Write(encoded[:])
}
