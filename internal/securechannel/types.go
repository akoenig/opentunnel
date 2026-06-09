package securechannel

import "errors"

const (
	PatternNKpsk0 = "Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s"
	PatternXXpsk3 = "Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s"

	SelectedPattern = PatternNKpsk0

	ClientSecretSize          = 32
	CommandTimeoutSeconds     = 120
	MaxOutputBytes            = 10 * 1024 * 1024
	IdleSessionTimeoutSeconds = 1800
)

var (
	ErrInvalidClientSecret = errors.New("invalid client secret")
	ErrHostKeyMismatch     = errors.New("host public key mismatch")
	ErrHandshakeFailed     = errors.New("handshake failed")
	ErrProtocol            = errors.New("protocol error")
)

type CommandDefaults struct {
	TimeoutSeconds            int
	MaxOutputBytes            int
	PTY                       bool
	IdleSessionTimeoutSeconds int
}

type PrologueConfig struct {
	App             string
	InviteVersion   byte
	NoiseProtocol   string
	SessionID       string
	RelayOrigin     string
	PermissionMode  string
	CommandDefaults CommandDefaults
	Features        []string
}

type HandshakeConfig struct {
	SessionID      string
	RelayOrigin    string
	ClientSecret   [ClientSecretSize]byte
	PermissionMode string
	Features       []string
}

func DefaultCommandDefaults() CommandDefaults {
	return CommandDefaults{
		TimeoutSeconds:            CommandTimeoutSeconds,
		MaxOutputBytes:            MaxOutputBytes,
		PTY:                       false,
		IdleSessionTimeoutSeconds: IdleSessionTimeoutSeconds,
	}
}

func NewPrologueConfig(cfg HandshakeConfig) PrologueConfig {
	return PrologueConfig{
		App:             "OpenTunnel",
		InviteVersion:   1,
		NoiseProtocol:   SelectedPattern,
		SessionID:       cfg.SessionID,
		RelayOrigin:     cfg.RelayOrigin,
		PermissionMode:  cfg.PermissionMode,
		CommandDefaults: DefaultCommandDefaults(),
		Features:        append([]string(nil), cfg.Features...),
	}
}
