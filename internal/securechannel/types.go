package securechannel

import "errors"

const (
	// PatternNKpsk0 is the Noise NK pattern with PSK in message position 0.
	PatternNKpsk0 = "Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s"
	// PatternXXpsk3 is the Noise XX pattern with PSK in message position 3.
	PatternXXpsk3 = "Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s"

	// SelectedPattern is the Noise pattern used by the secure-channel spike.
	SelectedPattern = PatternNKpsk0

	// ClientSecretSize is the required byte length for client PSK material.
	ClientSecretSize = 32
	// CommandTimeoutSeconds is the default command execution timeout.
	CommandTimeoutSeconds = 120
	// MaxOutputBytes is the default maximum command output size.
	MaxOutputBytes = 10 * 1024 * 1024
	// IdleSessionTimeoutSeconds is the default idle session timeout.
	IdleSessionTimeoutSeconds = 1800
)

var (
	// ErrInvalidClientSecret reports malformed client secret material.
	ErrInvalidClientSecret = errors.New("invalid client secret")
	// ErrHostKeyMismatch reports a host public key verification failure.
	ErrHostKeyMismatch = errors.New("host public key mismatch")
	// ErrHandshakeFailed reports an unsuccessful secure-channel handshake.
	ErrHandshakeFailed = errors.New("handshake failed")
	// ErrProtocol reports malformed or unexpected secure-channel protocol data.
	ErrProtocol = errors.New("protocol error")
)

// CommandDefaults describes default command-execution limits bound into the prologue.
type CommandDefaults struct {
	TimeoutSeconds            int
	MaxOutputBytes            int
	PTY                       bool
	IdleSessionTimeoutSeconds int
}

// PrologueConfig contains stable values authenticated by the Noise prologue.
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

// HandshakeConfig contains caller-provided inputs for secure-channel setup.
type HandshakeConfig struct {
	SessionID      string
	RelayOrigin    string
	ClientSecret   [ClientSecretSize]byte
	PermissionMode string
	Features       []string
}

// DefaultCommandDefaults returns command limits used when building a prologue.
func DefaultCommandDefaults() CommandDefaults {
	return CommandDefaults{
		TimeoutSeconds:            CommandTimeoutSeconds,
		MaxOutputBytes:            MaxOutputBytes,
		PTY:                       false,
		IdleSessionTimeoutSeconds: IdleSessionTimeoutSeconds,
	}
}

// NewPrologueConfig derives authenticated prologue values from handshake inputs.
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
