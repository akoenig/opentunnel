package artifact

import (
	"strings"
	"testing"
)

func TestRenderBootstrapRendersPOSIXBootstrapScript(t *testing.T) {
	script, err := RenderBootstrap(BootstrapConfig{
		RelayOrigin: "http://relay.example",
		Version:     "dev",
		PlatformKey: "linux-amd64",
		Checksum:    "abc123",
	})
	if err != nil {
		t.Fatalf("RenderBootstrap returned error: %v", err)
	}

	wants := []string{
		"relay_origin='http://relay.example'",
		"/cli/bin/opentunnel-dev-linux-amd64",
		"/cli/bin/opentunnel-dev-linux-amd64.sha256",
		"umask 077",
		"${TMPDIR:-/tmp}",
		"sha256sum",
		"shasum -a 256",
		"chmod 700",
		"OPENTUNNEL_RELAY_ORIGIN",
		"exec \"$bin\" \"$@\"",
	}
	for _, want := range wants {
		if !strings.Contains(script, want) {
			t.Fatalf("RenderBootstrap() missing %q in script:\n%s", want, script)
		}
	}
}

func TestRenderBootstrapRejectsMissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  BootstrapConfig
	}{
		{
			name: "relay origin",
			cfg: BootstrapConfig{
				Version:     "dev",
				PlatformKey: "linux-amd64",
				Checksum:    "abc123",
			},
		},
		{
			name: "version",
			cfg: BootstrapConfig{
				RelayOrigin: "http://relay.example",
				PlatformKey: "linux-amd64",
				Checksum:    "abc123",
			},
		},
		{
			name: "platform key",
			cfg: BootstrapConfig{
				RelayOrigin: "http://relay.example",
				Version:     "dev",
				Checksum:    "abc123",
			},
		},
		{
			name: "checksum",
			cfg: BootstrapConfig{
				RelayOrigin: "http://relay.example",
				Version:     "dev",
				PlatformKey: "linux-amd64",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := RenderBootstrap(tt.cfg); err == nil {
				t.Fatal("RenderBootstrap returned nil error")
			}
		})
	}
}
