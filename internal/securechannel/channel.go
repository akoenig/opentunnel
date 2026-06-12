package securechannel

import (
	"fmt"
	"io"

	"github.com/flynn/noise"
)

// HostKeypair contains the host's per-session X25519 key material.
type HostKeypair struct {
	Public  []byte
	private noise.DHKey
}

// Channel encrypts frames in one direction and decrypts frames from the peer.
type Channel struct {
	send *noise.CipherState
	recv *noise.CipherState
}

// ClientHandshake drives the client side of the NKpsk0 transport handshake.
type ClientHandshake struct {
	hs *noise.HandshakeState
}

// HostHandshake drives the host side of the NKpsk0 transport handshake.
type HostHandshake struct {
	hs *noise.HandshakeState
}

// GenerateHostKeypair creates a fresh per-session host keypair.
func GenerateHostKeypair(r io.Reader) (HostKeypair, error) {
	key, err := noise.DH25519.GenerateKeypair(r)
	if err != nil {
		return HostKeypair{}, fmt.Errorf("generate host keypair: %w", err)
	}

	return HostKeypair{
		Public:  append([]byte(nil), key.Public...),
		private: key,
	}, nil
}

// NewClientHandshake creates a client-side NKpsk0 handshake for a known host public key.
func NewClientHandshake(cfg HandshakeConfig, expectedHostPublic []byte) (*ClientHandshake, error) {
	if len(expectedHostPublic) == 0 {
		return nil, fmt.Errorf("%w: expected host public key is required", ErrHostKeyMismatch)
	}

	noiseCfg, err := nkpsk0Config(cfg)
	if err != nil {
		return nil, err
	}
	noiseCfg.Initiator = true
	noiseCfg.PeerStatic = expectedHostPublic

	hs, err := noise.NewHandshakeState(noiseCfg)
	if err != nil {
		return nil, fmt.Errorf("%w: create client handshake: %w", ErrHandshakeFailed, err)
	}

	return &ClientHandshake{hs: hs}, nil
}

// NewHostHandshake creates a host-side NKpsk0 handshake with the host keypair.
func NewHostHandshake(cfg HandshakeConfig, hostKey HostKeypair) (*HostHandshake, error) {
	noiseCfg, err := nkpsk0Config(cfg)
	if err != nil {
		return nil, err
	}
	noiseCfg.Initiator = false
	noiseCfg.StaticKeypair = hostKey.private

	hs, err := noise.NewHandshakeState(noiseCfg)
	if err != nil {
		return nil, fmt.Errorf("%w: create host handshake: %w", ErrHandshakeFailed, err)
	}

	return &HostHandshake{hs: hs}, nil
}

// WriteMessage writes the client's first NKpsk0 handshake message.
func (h *ClientHandshake) WriteMessage() ([]byte, error) {
	if h == nil || h.hs == nil {
		return nil, fmt.Errorf("%w: client handshake is not initialized", ErrProtocol)
	}
	message, _, _, err := h.hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: client write handshake: %w", ErrHandshakeFailed, err)
	}
	return message, nil
}

// ReadMessage reads the host's second NKpsk0 handshake message and returns a transport channel.
func (h *ClientHandshake) ReadMessage(message []byte) (*Channel, error) {
	if h == nil || h.hs == nil {
		return nil, fmt.Errorf("%w: client handshake is not initialized", ErrProtocol)
	}
	_, clientSend, clientRecv, err := h.hs.ReadMessage(nil, message)
	if err != nil {
		return nil, fmt.Errorf("%w: client read handshake: %w", ErrHandshakeFailed, err)
	}
	return &Channel{send: clientSend, recv: clientRecv}, nil
}

// ReadMessage reads the client's first NKpsk0 handshake message and returns the host response and transport channel.
func (h *HostHandshake) ReadMessage(message []byte) ([]byte, *Channel, error) {
	if h == nil || h.hs == nil {
		return nil, nil, fmt.Errorf("%w: host handshake is not initialized", ErrProtocol)
	}
	if _, _, _, err := h.hs.ReadMessage(nil, message); err != nil {
		return nil, nil, fmt.Errorf("%w: host read handshake: %w", ErrHandshakeFailed, err)
	}
	response, hostRecv, hostSend, err := h.hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: host write handshake: %w", ErrHandshakeFailed, err)
	}
	return response, &Channel{send: hostSend, recv: hostRecv}, nil
}

func nkpsk0Config(cfg HandshakeConfig) (noise.Config, error) {
	prologue, err := BuildPrologue(NewPrologueConfig(cfg))
	if err != nil {
		return noise.Config{}, err
	}

	return noise.Config{
		CipherSuite:           noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s),
		Pattern:               noise.HandshakeNK,
		Prologue:              prologue,
		PresharedKey:          cfg.ClientSecret[:],
		PresharedKeyPlacement: 0,
	}, nil
}

// Encrypt encrypts one transport frame for the peer.
func (c *Channel) Encrypt(plaintext []byte) ([]byte, error) {
	if c == nil || c.send == nil {
		return nil, fmt.Errorf("%w: send cipher is not initialized", ErrProtocol)
	}
	ciphertext, err := c.send.Encrypt(nil, nil, plaintext)
	if err != nil {
		return nil, fmt.Errorf("%w: encrypt frame: %w", ErrProtocol, err)
	}
	return ciphertext, nil
}

// Decrypt decrypts one transport frame from the peer.
func (c *Channel) Decrypt(ciphertext []byte) ([]byte, error) {
	if c == nil || c.recv == nil {
		return nil, fmt.Errorf("%w: receive cipher is not initialized", ErrProtocol)
	}
	plaintext, err := c.recv.Decrypt(nil, nil, ciphertext)
	if err != nil {
		return nil, fmt.Errorf("%w: decrypt frame: %w", ErrProtocol, err)
	}
	return plaintext, nil
}
