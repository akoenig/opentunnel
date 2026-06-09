package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseArgsRelay(t *testing.T) {
	cmd, err := parseArgs([]string{"relay", "--listen", ":8080", "--public-url", "http://localhost:8080"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	relay, ok := cmd.(relayCommand)
	if !ok {
		t.Fatalf("parseArgs() command = %T, want relayCommand", cmd)
	}
	if relay.listen != ":8080" {
		t.Fatalf("relay.listen = %q, want %q", relay.listen, ":8080")
	}
	if relay.publicURL != "http://localhost:8080" {
		t.Fatalf("relay.publicURL = %q, want %q", relay.publicURL, "http://localhost:8080")
	}
}

func TestParseArgsCreate(t *testing.T) {
	cmd, err := parseArgs([]string{"create", "--relay", "http://localhost:8080"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	create, ok := cmd.(createCommand)
	if !ok {
		t.Fatalf("parseArgs() command = %T, want createCommand", cmd)
	}
	if create.relayURL != "http://localhost:8080" {
		t.Fatalf("create.relayURL = %q, want %q", create.relayURL, "http://localhost:8080")
	}
}

func TestParseArgsCreateUsesRelayOriginFromEnvironment(t *testing.T) {
	t.Setenv("OPENTUNNEL_RELAY_ORIGIN", "http://localhost:8080")

	cmd, err := parseArgs([]string{"create"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	create, ok := cmd.(createCommand)
	if !ok {
		t.Fatalf("parseArgs() command = %T, want createCommand", cmd)
	}
	if create.relayURL != "http://localhost:8080" {
		t.Fatalf("create.relayURL = %q, want %q", create.relayURL, "http://localhost:8080")
	}
}

func TestBuildRelayCLIArtifactsUsesRunningExecutableAndPlatform(t *testing.T) {
	artifacts, err := buildRelayCLIArtifacts("http://localhost:8080", func() (string, error) {
		return "/tmp/opentunnel", nil
	}, func() (string, error) {
		return "testos-testarch", nil
	})
	if err != nil {
		t.Fatalf("buildRelayCLIArtifacts() error = %v", err)
	}

	if artifacts.RelayOrigin != "http://localhost:8080" {
		t.Fatalf("RelayOrigin = %q, want %q", artifacts.RelayOrigin, "http://localhost:8080")
	}
	if artifacts.Version != "dev" {
		t.Fatalf("Version = %q, want %q", artifacts.Version, "dev")
	}
	if artifacts.PlatformKey != "testos-testarch" {
		t.Fatalf("PlatformKey = %q, want %q", artifacts.PlatformKey, "testos-testarch")
	}
	if artifacts.BinaryPath != "/tmp/opentunnel" {
		t.Fatalf("BinaryPath = %q, want %q", artifacts.BinaryPath, "/tmp/opentunnel")
	}
}

func TestWriteCreateReadyPrintsBootstrapPromptWithoutStandaloneSecrets(t *testing.T) {
	var stdout bytes.Buffer
	invite := "ot1_example_secret"

	writeCreateReady(&stdout, invite, "http://localhost:8080")

	output := stdout.String()
	want := "curl -fsSL http://localhost:8080/cli | sh -s -- exec --invite '" + invite + "' -- <command>"
	if !strings.Contains(output, want) {
		t.Fatalf("create output missing bootstrap prompt %q in:\n%s", want, output)
	}
	if strings.Count(output, invite) != 1 {
		t.Fatalf("create output prints invite %d times, want once in:\n%s", strings.Count(output, invite), output)
	}
	if strings.Contains(output, "invite:") || strings.Contains(output, "secret:") {
		t.Fatalf("create output contains standalone secret label in:\n%s", output)
	}
}

func TestParseArgsExec(t *testing.T) {
	cmd, err := parseArgs([]string{"exec", "--invite", "ot1_example", "--", "hostname"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	exec, ok := cmd.(execCommand)
	if !ok {
		t.Fatalf("parseArgs() command = %T, want execCommand", cmd)
	}
	if exec.invite != "ot1_example" {
		t.Fatalf("exec.invite = %q, want %q", exec.invite, "ot1_example")
	}
	if exec.command != "hostname" {
		t.Fatalf("exec.command = %q, want %q", exec.command, "hostname")
	}
}

func TestParseArgsRejectsUnknownSubcommand(t *testing.T) {
	_, err := parseArgs([]string{"unknown"})
	if err == nil {
		t.Fatal("parseArgs() error = nil, want error")
	}
}

func TestParseArgsRejectsExecMissingSeparatorCommand(t *testing.T) {
	_, err := parseArgs([]string{"exec", "--invite", "ot1_example", "hostname"})
	if err == nil {
		t.Fatal("parseArgs() error = nil, want error")
	}

	_, err = parseArgs([]string{"exec", "--invite", "ot1_example", "--"})
	if err == nil {
		t.Fatal("parseArgs() error = nil, want error")
	}
}
