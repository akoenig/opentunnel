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

	"opentunnel/internal/relay"
	"opentunnel/internal/tunnel"
)

type command interface {
	run(context.Context, io.Writer, io.Writer) int
}

type relayCommand struct {
	listen    string
	publicURL string
}

type createCommand struct {
	relayURL string
}

type execCommand struct {
	invite  string
	command string
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	cmd, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(stderr, "opentunnel: %v\n", err)
		return 2
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
	cmd := relayCommand{}
	flags.StringVar(&cmd.listen, "listen", ":8080", "HTTP listen address")
	flags.StringVar(&cmd.publicURL, "public-url", "", "public relay URL")
	if err := flags.Parse(args); err != nil {
		return relayCommand{}, err
	}
	if flags.NArg() != 0 {
		return relayCommand{}, fmt.Errorf("relay got unexpected argument %q", flags.Arg(0))
	}
	if cmd.publicURL != "" {
		if err := validatePublicURL(cmd.publicURL); err != nil {
			return relayCommand{}, err
		}
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
		return createCommand{}, errors.New("create requires --relay")
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
	if err := flags.Parse(args[:separator]); err != nil {
		return execCommand{}, err
	}
	if flags.NArg() != 0 {
		return execCommand{}, fmt.Errorf("exec got unexpected argument %q before --", flags.Arg(0))
	}
	if cmd.invite == "" {
		return execCommand{}, errors.New("exec requires --invite")
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

func validatePublicURL(raw string) error {
	publicURL, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse public url: %w", err)
	}
	if publicURL.Scheme != "http" && publicURL.Scheme != "https" {
		return errors.New("public url must use http or https")
	}
	if publicURL.Host == "" {
		return errors.New("public url host is required")
	}
	return nil
}

func (cmd relayCommand) run(ctx context.Context, stdout io.Writer, stderr io.Writer) int {
	server := &http.Server{Addr: cmd.listen, Handler: relay.NewServer().Handler()}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		if err := server.Shutdown(context.Background()); err != nil {
			fmt.Fprintf(stderr, "shutdown relay: %v\n", err)
			return 1
		}
		return 0
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return 0
		}
		fmt.Fprintf(stderr, "start relay: %v\n", err)
		return 1
	}
}

func (cmd createCommand) run(ctx context.Context, stdout io.Writer, stderr io.Writer) int {
	relayURL, err := websocketRelayURL(cmd.relayURL)
	if err != nil {
		fmt.Fprintf(stderr, "create: %v\n", err)
		return 1
	}

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	session, err := tunnel.StartHost(signalCtx, tunnel.HostConfig{RelayURL: relayURL, LogWriter: stderr})
	if err != nil {
		fmt.Fprintf(stderr, "create: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "agent-ready\ninvite: %s\nexec: opentunnel exec --invite %s -- hostname\n", session.Invite, session.Invite)

	select {
	case <-signalCtx.Done():
		return 0
	case err, ok := <-session.Done:
		if ok && err != nil {
			fmt.Fprintf(stderr, "host: %v\n", err)
			return 1
		}
		return 0
	}
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
		fmt.Fprintf(stderr, "exec: %v\n", err)
		return 1
	}
	return result.ExitCode
}
