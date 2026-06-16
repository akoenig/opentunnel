package main

import (
	"bytes"
	"context"
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
		"curl -fsSL http://localhost:8080/cli | OPENTUNNEL_INVITE='" + invite + "' sh -s -- exec \\",
		"  -- '<COMMAND>'",
		"Start with:",
		"curl -fsSL http://localhost:8080/cli | OPENTUNNEL_INVITE='" + invite + "' sh -s -- exec \\",
		"  -- 'hostname && uname -a && pwd'",
		"Commands execute without per-command approval while this foreground session is running.",
		"Always ask me to confirm before running anything destructive or irreversible, for example deleting files or directories, overwriting data, changing system, package, or network configuration, or moving data off the machine.",
		"Treat the invite as bearer-secret material.",
		"The host owner can revoke access with Ctrl+C.",
		"For shared machines, prefer --invite-stdin or shell-history controls because typed environment assignments can still be saved by your shell.",
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
	if strings.Contains(output, "\n  --invite ") {
		t.Fatalf("create output contains --invite in generated command examples:\n%s", output)
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

func TestParseArgsExecUsesInviteFromEnvironment(t *testing.T) {
	t.Setenv("OPENTUNNEL_INVITE", "ot1_from_env")

	cmd, err := parseArgs([]string{"exec", "--", "hostname"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	exec, ok := cmd.(execCommand)
	if !ok {
		t.Fatalf("parseArgs() command = %T, want execCommand", cmd)
	}
	if exec.invite != "ot1_from_env" {
		t.Fatalf("exec.invite = %q, want %q", exec.invite, "ot1_from_env")
	}
}

func TestParseArgsExecInviteFlagWinsOverEnvironment(t *testing.T) {
	t.Setenv("OPENTUNNEL_INVITE", "ot1_from_env")

	cmd, err := parseArgs([]string{"exec", "--invite", "ot1_from_flag", "--", "hostname"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	exec, ok := cmd.(execCommand)
	if !ok {
		t.Fatalf("parseArgs() command = %T, want execCommand", cmd)
	}
	if exec.invite != "ot1_from_flag" {
		t.Fatalf("exec.invite = %q, want %q", exec.invite, "ot1_from_flag")
	}
}

func TestParseArgsExecRequiresInviteSource(t *testing.T) {
	t.Setenv("OPENTUNNEL_INVITE", "")

	_, err := parseArgs([]string{"exec", "--", "hostname"})
	if err == nil {
		t.Fatal("parseArgs() error = nil, want error")
	}
	for _, want := range []string{"--invite", "OPENTUNNEL_INVITE", "--invite-stdin"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("parseArgs() error = %q, want %q", err.Error(), want)
		}
	}
}

func TestParseArgsExecAcceptsInviteStdinWithoutInvite(t *testing.T) {
	t.Setenv("OPENTUNNEL_INVITE", "")

	cmd, err := parseArgs([]string{"exec", "--invite-stdin", "--", "hostname"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}

	exec, ok := cmd.(execCommand)
	if !ok {
		t.Fatalf("parseArgs() command = %T, want execCommand", cmd)
	}
	if !exec.inviteStdin {
		t.Fatal("exec.inviteStdin = false, want true")
	}
}

func TestExecRunWithStdinRejectsEmptyInvite(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := (execCommand{inviteStdin: true, command: "hostname"}).runWithStdin(context.Background(), strings.NewReader(" \n\t"), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("runWithStdin() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "exec: invite from stdin is empty") {
		t.Fatalf("stderr = %q, want empty invite error", stderr.String())
	}
}

func TestExecRunWithStdinTrimsInviteAndRunsExecPath(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := (execCommand{inviteStdin: true, command: "hostname"}).runWithStdin(context.Background(), strings.NewReader(" \not1_!!!\n"), &stdout, &stderr)

	if code != 1 {
		t.Fatalf("runWithStdin() = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "decode invite payload base64") {
		t.Fatalf("stderr = %q, want trimmed invite to reach exec decode path", stderr.String())
	}
	if strings.Contains(stderr.String(), "missing ot1_ prefix") || strings.Contains(stderr.String(), "invite from stdin is empty") {
		t.Fatalf("stderr = %q, want no untrimmed or empty invite error", stderr.String())
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
