package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseArgsRelay(t *testing.T) {
	cmd, err := parseArgs([]string{"relay", "--listen", ":8081", "--public-url", "http://localhost:8080", "--artifact-dir", "/tmp/opentunnel-artifacts", "--version", "1.2.3"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	relay, ok := cmd.(relayCommand)
	if !ok {
		t.Fatalf("parseArgs() command = %T, want relayCommand", cmd)
	}
	if relay.listen != ":8081" {
		t.Fatalf("relay.listen = %q, want %q", relay.listen, ":8081")
	}
	if relay.publicURL != "http://localhost:8080" {
		t.Fatalf("relay.publicURL = %q, want %q", relay.publicURL, "http://localhost:8080")
	}
	if relay.artifactDir != "/tmp/opentunnel-artifacts" {
		t.Fatalf("relay.artifactDir = %q, want %q", relay.artifactDir, "/tmp/opentunnel-artifacts")
	}
	if relay.version != "1.2.3" {
		t.Fatalf("relay.version = %q, want %q", relay.version, "1.2.3")
	}
}

func TestParseArgsRelayRequiresPublicURL(t *testing.T) {
	_, err := parseArgs([]string{"relay"})
	if err == nil {
		t.Fatal("parseArgs() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "public url") {
		t.Fatalf("parseArgs() error = %q, want public url", err.Error())
	}
}

func TestParseArgsRelayDefaultsArtifactDirAndVersion(t *testing.T) {
	cmd, err := parseArgs([]string{"relay", "--public-url", "https://relay.example.com"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	relay, ok := cmd.(relayCommand)
	if !ok {
		t.Fatalf("parseArgs() command = %T, want relayCommand", cmd)
	}
	if relay.artifactDir != "/opentunnel-artifacts" {
		t.Fatalf("relay.artifactDir = %q, want %q", relay.artifactDir, "/opentunnel-artifacts")
	}
	if relay.version != "dev" {
		t.Fatalf("relay.version = %q, want %q", relay.version, "dev")
	}
}

func TestParseArgsRelayRejectsArtifactPath(t *testing.T) {
	_, err := parseArgs([]string{"relay", "--public-url", "https://relay.example.com", "--artifact-path", "/tmp/opentunnel"})
	if err == nil {
		t.Fatal("parseArgs() error = nil, want error")
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

func TestParseArgsCreateRejectsUnsafeRelayOrigin(t *testing.T) {
	tests := []string{
		"http://example.test/$(id)",
		"http://example.test/path",
		"http://example.test?download=true",
		"http://example.test#fragment",
		"http://user@example.test",
		"ftp://example.test",
		"http:///missing-host",
		"-http://example.test",
	}

	for _, relayOrigin := range tests {
		t.Run(relayOrigin, func(t *testing.T) {
			_, err := parseArgs([]string{"create", "--relay", relayOrigin})
			if err == nil {
				t.Fatal("parseArgs() error = nil, want error")
			}
		})
	}
}

func TestParseArgsCreateRejectsUnsafeRelayOriginFromEnvironment(t *testing.T) {
	t.Setenv("OPENTUNNEL_RELAY_ORIGIN", "http://example.test/$(id)")

	_, err := parseArgs([]string{"create"})
	if err == nil {
		t.Fatal("parseArgs() error = nil, want error")
	}
}

func TestParseArgsRelayRejectsUnsafePublicURLOrigin(t *testing.T) {
	tests := []string{
		"http://example.test/$(id)",
		"http://example.test/path",
		"http://example.test?download=true",
		"http://example.test#fragment",
		"http://user@example.test",
		"ftp://example.test",
		"http:///missing-host",
		"-http://example.test",
	}

	for _, publicURL := range tests {
		t.Run(publicURL, func(t *testing.T) {
			_, err := parseArgs([]string{"relay", "--public-url", publicURL})
			if err == nil {
				t.Fatal("parseArgs() error = nil, want error")
			}
		})
	}
}

func TestWriteCreateReadyPrintsPublicAgentPrompt(t *testing.T) {
	var stdout bytes.Buffer
	invite := "ot1_example_secret"

	writeCreateReady(&stdout, invite, "http://localhost:8080")

	output := stdout.String()
	wants := []string{
		"I opened an OpenTunnel session for you.",
		"Run commands on my host with:",
		"curl -fsSL http://localhost:8080/cli | sh -s -- exec \\",
		"  --invite '" + invite + "' \\",
		"  -- '<COMMAND>'",
		"Start with:",
		"  -- 'hostname && uname -a && pwd'",
		"Commands execute without per-command approval while this foreground session is running.",
		"Treat the invite as bearer-secret material.",
		"The host owner can revoke access with Ctrl+C.",
		"Use non-interactive commands.",
		"No PTY or interactive stdin is available in the first major version.",
		"Avoid sudo unless it is passwordless and non-interactive.",
		"Only one client can connect to this tunnel at a time.",
		"Only one command runs at a time.",
		"The temporary OpenTunnel CLI is cached in the system temp directory during the session.",
	}
	for _, want := range wants {
		if !strings.Contains(output, want) {
			t.Fatalf("create output missing %q in:\n%s", want, output)
		}
	}
	if strings.Count(output, invite) != 2 {
		t.Fatalf("create output prints invite %d times, want twice in command examples:\n%s", strings.Count(output, invite), output)
	}
	if strings.Contains(output, " --relay ") || strings.Contains(output, "\nrelay:") || strings.Contains(output, "\nsecret:") {
		t.Fatalf("create output contains user-facing relay flag or standalone secret label in:\n%s", output)
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
