package securechannel

import (
	"crypto/subtle"
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

// EstablishChannelWithHostKey performs the v1 NKpsk0 handshake for a known host key.
func EstablishChannelWithHostKey(cfg HandshakeConfig, hostKey HostKeypair, expectedHostPublic []byte) (*Channel, *Channel, error) {
	return establishNKpsk0(cfg, hostKey.private, expectedHostPublic)
}

func establishNKpsk0(cfg HandshakeConfig, hostKey noise.DHKey, expectedHostPublic []byte) (*Channel, *Channel, error) {
	if len(expectedHostPublic) == 0 {
		return nil, nil, fmt.Errorf("%w: expected host public key is required", ErrHostKeyMismatch)
	}

	prologue, err := BuildPrologue(NewPrologueConfig(cfg))
	if err != nil {
		return nil, nil, err
	}

	noiseCfg := noise.Config{
		CipherSuite:           noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s),
		Pattern:               noise.HandshakeNK,
		Initiator:             true,
		Prologue:              prologue,
		PresharedKey:          cfg.ClientSecret[:],
		PresharedKeyPlacement: 0,
		PeerStatic:            expectedHostPublic,
	}

	clientHS, err := noise.NewHandshakeState(noiseCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: create client handshake: %w", ErrHandshakeFailed, err)
	}

	hostCfg := noiseCfg
	hostCfg.Initiator = false
	hostCfg.StaticKeypair = hostKey
	hostCfg.PeerStatic = nil

	hostHS, err := noise.NewHandshakeState(hostCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: create host handshake: %w", ErrHandshakeFailed, err)
	}

	msg1, _, _, err := clientHS.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: client write handshake: %w", ErrHandshakeFailed, err)
	}

	if _, _, _, err := hostHS.ReadMessage(nil, msg1); err != nil {
		return nil, nil, fmt.Errorf("%w: host read handshake: %w", ErrHandshakeFailed, err)
	}

	msg2, hostRecv, hostSend, err := hostHS.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: host write handshake: %w", ErrHandshakeFailed, err)
	}

	_, clientSend, clientRecv, err := clientHS.ReadMessage(nil, msg2)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: client read handshake: %w", ErrHandshakeFailed, err)
	}

	if subtle.ConstantTimeCompare(hostKey.Public, expectedHostPublic) != 1 {
		return nil, nil, ErrHostKeyMismatch
	}

	return &Channel{send: clientSend, recv: clientRecv}, &Channel{send: hostSend, recv: hostRecv}, nil
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
