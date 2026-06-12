package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"opentunnel/internal/buildinfo"
	"opentunnel/internal/originurl"
	"opentunnel/internal/relay"
	"opentunnel/internal/tunnel"
)

type command interface {
	run(context.Context, io.Writer, io.Writer) int
}

type relayCommand struct {
	listen      string
	publicURL   string
	artifactDir string
	version     string
}

type createCommand struct {
	relayURL string
}

type execCommand struct {
	invite      string
	inviteStdin bool
	command     string
}

func main() {
	os.Exit(runWithStdin(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runWithStdin(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	cmd, err := parseArgs(args)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "opentunnel: %v\n", err)
		return 2
	}
	if exec, ok := cmd.(execCommand); ok {
		return exec.runWithStdin(ctx, stdin, stdout, stderr)
	}
	return cmd.run(ctx, stdout, stderr)
}

func parseArgs(args []string) (command, error) {
	if len(args) == 0 {
		return nil, errors.New("subcommand is required")
	}

	switch args[0] {
	case "relay":
		return parseRelayArgs(args[1:])
	case "create":
		return parseCreateArgs(args[1:])
	case "exec":
		return parseExecArgs(args[1:])
	default:
		return nil, fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func parseRelayArgs(args []string) (relayCommand, error) {
	flags := flag.NewFlagSet("relay", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	cmd := relayCommand{listen: ":8080", artifactDir: "/opentunnel-artifacts", version: buildinfo.Version}
	flags.StringVar(&cmd.listen, "listen", ":8080", "HTTP listen address")
	flags.StringVar(&cmd.publicURL, "public-url", "", "public relay URL")
	flags.StringVar(&cmd.artifactDir, "artifact-dir", "/opentunnel-artifacts", "CLI artifact directory")
	flags.StringVar(&cmd.version, "version", buildinfo.Version, "CLI artifact version")
	if err := flags.Parse(args); err != nil {
		return relayCommand{}, err
	}
	if flags.NArg() != 0 {
		return relayCommand{}, fmt.Errorf("relay got unexpected argument %q", flags.Arg(0))
	}
	if cmd.publicURL == "" {
		return relayCommand{}, errors.New("relay requires public url")
	}
	if err := validateRelayOrigin(cmd.publicURL, "public url"); err != nil {
		return relayCommand{}, err
	}
	return cmd, nil
}

func parseCreateArgs(args []string) (createCommand, error) {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	cmd := createCommand{}
	flags.StringVar(&cmd.relayURL, "relay", "", "relay URL")
	if err := flags.Parse(args); err != nil {
		return createCommand{}, err
	}
	if flags.NArg() != 0 {
		return createCommand{}, fmt.Errorf("create got unexpected argument %q", flags.Arg(0))
	}
	if cmd.relayURL == "" {
		cmd.relayURL = os.Getenv("OPENTUNNEL_RELAY_ORIGIN")
	}
	if cmd.relayURL == "" {
		return createCommand{}, errors.New("create requires --relay")
	}
	if err := validateRelayOrigin(cmd.relayURL, "relay url"); err != nil {
		return createCommand{}, err
	}
	return cmd, nil
}

func parseExecArgs(args []string) (execCommand, error) {
	separator := separatorIndex(args)
	if separator == -1 {
		return execCommand{}, errors.New("exec requires -- before command")
	}
	if separator == len(args)-1 {
		return execCommand{}, errors.New("exec requires command after --")
	}

	flags := flag.NewFlagSet("exec", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	cmd := execCommand{}
	flags.StringVar(&cmd.invite, "invite", "", "invite code")
	flags.BoolVar(&cmd.inviteStdin, "invite-stdin", false, "read invite code from stdin")
	if err := flags.Parse(args[:separator]); err != nil {
		return execCommand{}, err
	}
	if flags.NArg() != 0 {
		return execCommand{}, fmt.Errorf("exec got unexpected argument %q before --", flags.Arg(0))
	}
	if cmd.invite == "" && !cmd.inviteStdin {
		cmd.invite = os.Getenv("OPENTUNNEL_INVITE")
	}
	if cmd.invite == "" && !cmd.inviteStdin {
		return execCommand{}, errors.New("exec requires --invite, OPENTUNNEL_INVITE, or --invite-stdin")
	}
	cmd.command = strings.Join(args[separator+1:], " ")
	return cmd, nil
}

func separatorIndex(args []string) int {
	for index, arg := range args {
		if arg == "--" {
			return index
		}
	}
	return -1
}

func validateRelayOrigin(raw string, name string) error {
	return originurl.Validate(raw, name)
}

func (cmd relayCommand) run(ctx context.Context, stdout io.Writer, stderr io.Writer) int {
	relayServer, err := relay.NewServerWithOptions(relay.ServerOptions{
		PublicURL:   cmd.publicURL,
		ArtifactDir: cmd.artifactDir,
		Version:     cmd.version,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "start relay: %v\n", err)
		return 1
	}

	loggingCtx, stopLogging := context.WithCancel(ctx)
	defer stopLogging()
	go relayServer.LogActiveTunnels(loggingCtx, relay.ActiveTunnelLogInterval, stderr)

	server := &http.Server{
		Addr:              cmd.listen,
		Handler:           relayServer.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		if err := server.Shutdown(context.Background()); err != nil {
			_, _ = fmt.Fprintf(stderr, "shutdown relay: %v\n", err)
			return 1
		}
		return 0
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return 0
		}
		_, _ = fmt.Fprintf(stderr, "start relay: %v\n", err)
		return 1
	}
}

func (cmd createCommand) run(ctx context.Context, stdout io.Writer, stderr io.Writer) int {
	relayURL, err := websocketRelayURL(cmd.relayURL)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "create: %v\n", err)
		return 1
	}

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	session, err := tunnel.StartHost(signalCtx, tunnel.HostConfig{RelayURL: relayURL, LogWriter: stderr})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "create: %v\n", err)
		return 1
	}
	writeCreateReady(stdout, session.Invite, cmd.relayURL)

	select {
	case <-signalCtx.Done():
		return 0
	case err, ok := <-session.Done:
		if ok && err != nil {
			_, _ = fmt.Fprintf(stderr, "host: %v\n", err)
			return 1
		}
		return 0
	}
}

func writeCreateReady(stdout io.Writer, invite string, relayURL string) {
	origin := strings.TrimRight(relayURL, "/")
	_, _ = fmt.Fprintf(stdout, `I opened an OpenTunnel session for you.

Run commands on my host with:

curl -fsSL %[1]s/cli | OPENTUNNEL_INVITE='%[2]s' sh -s -- exec \
  -- '<COMMAND>'

Start with:

curl -fsSL %[1]s/cli | OPENTUNNEL_INVITE='%[2]s' sh -s -- exec \
  -- 'hostname && uname -a && pwd'

Commands execute without per-command approval while this foreground session is running.
Treat the invite as bearer-secret material. Do not copy it into shared logs, tickets, summaries, or long-lived notes. The host owner can revoke access with Ctrl+C.

Notes:
- For shared machines, prefer --invite-stdin or shell-history controls because typed environment assignments can still be saved by your shell.
- Use non-interactive commands.
- No PTY or interactive stdin is available in the first major version.
- Avoid sudo unless it is passwordless and non-interactive.
- Avoid long-running commands unless necessary.
- Only one client can connect to this tunnel at a time.
- Only one command runs at a time.
- The temporary OpenTunnel CLI is cached in the system temp directory during the session.
`, origin, invite)
}

func websocketRelayURL(raw string) (string, error) {
	relayURL, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse relay url: %w", err)
	}
	if relayURL.Host == "" {
		return "", errors.New("relay url host is required")
	}
	switch relayURL.Scheme {
	case "http":
		relayURL.Scheme = "ws"
	case "https":
		relayURL.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", errors.New("relay url must use http, https, ws, or wss")
	}
	return relayURL.String(), nil
}

func (cmd execCommand) run(ctx context.Context, stdout io.Writer, stderr io.Writer) int {
	result, err := tunnel.Exec(ctx, tunnel.ExecConfig{
		Invite:  cmd.invite,
		Command: cmd.command,
		Stdout:  stdout,
		Stderr:  stderr,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "exec: %v\n", err)
		return 1
	}
	return result.ExitCode
}

func (cmd execCommand) runWithStdin(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if cmd.inviteStdin {
		data, err := io.ReadAll(stdin)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "exec: read invite from stdin: %v\n", err)
			return 1
		}
		cmd.invite = strings.TrimSpace(string(data))
		if cmd.invite == "" {
			_, _ = fmt.Fprintln(stderr, "exec: invite from stdin is empty")
			return 1
		}
	}
	return cmd.run(ctx, stdout, stderr)
}
