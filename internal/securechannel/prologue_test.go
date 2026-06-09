package securechannel

import (
	"bytes"
	"testing"
)

func TestBuildPrologueDeterministic(t *testing.T) {
	cfg := PrologueConfig{
		App:             "OpenTunnel",
		InviteVersion:   1,
		NoiseProtocol:   PatternNKpsk0,
		SessionID:       "stn_test",
		RelayOrigin:     "https://relay.example",
		PermissionMode:  "yolo",
		CommandDefaults: DefaultCommandDefaults(),
		Features:        []string{"exec.v1", "stdoutStderr.v1"},
	}

	first, err := BuildPrologue(cfg)
	if err != nil {
		t.Fatalf("BuildPrologue first call: %v", err)
	}

	second, err := BuildPrologue(cfg)
	if err != nil {
		t.Fatalf("BuildPrologue second call: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Fatalf("prologue is not deterministic")
	}
}

func TestBuildPrologueChangesWhenSecurityContextChanges(t *testing.T) {
	base := PrologueConfig{
		App:             "OpenTunnel",
		InviteVersion:   1,
		NoiseProtocol:   PatternNKpsk0,
		SessionID:       "stn_test",
		RelayOrigin:     "https://relay.example",
		PermissionMode:  "yolo",
		CommandDefaults: DefaultCommandDefaults(),
		Features:        []string{"exec.v1", "stdoutStderr.v1"},
	}

	baseBytes, err := BuildPrologue(base)
	if err != nil {
		t.Fatalf("BuildPrologue base: %v", err)
	}

	changed := base
	changed.SessionID = "stn_other"

	changedBytes, err := BuildPrologue(changed)
	if err != nil {
		t.Fatalf("BuildPrologue changed: %v", err)
	}

	if bytes.Equal(baseBytes, changedBytes) {
		t.Fatalf("prologue did not change when session id changed")
	}
}

func TestBuildPrologueRejectsMissingFields(t *testing.T) {
	_, err := BuildPrologue(PrologueConfig{
		App:             "OpenTunnel",
		InviteVersion:   1,
		NoiseProtocol:   PatternNKpsk0,
		SessionID:       "",
		RelayOrigin:     "https://relay.example",
		PermissionMode:  "yolo",
		CommandDefaults: DefaultCommandDefaults(),
		Features:        []string{"exec.v1"},
	})

	if err == nil {
		t.Fatalf("expected missing session id error")
	}
}
