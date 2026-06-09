package main

import "testing"

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
