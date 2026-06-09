package command

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

// OutputChunk is a chunk of process output from stdout or stderr.
type OutputChunk struct {
	Stream string
	Data   []byte
}

// Result is the outcome of a completed command.
type Result struct {
	ExitCode int
}

// Run executes command with /bin/sh -c and streams stdout and stderr chunks.
func Run(ctx context.Context, command string, onChunk func(OutputChunk)) (Result, error) {
	if strings.TrimSpace(command) == "" {
		return Result{}, errors.New("command must not be empty")
	}

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("open stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, fmt.Errorf("open stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Result{}, fmt.Errorf("command canceled: %w", ctxErr)
		}
		return Result{}, fmt.Errorf("start command: %w", err)
	}

	var readers sync.WaitGroup
	chunks := make(chan OutputChunk)
	streamErrors := make(chan error, 2)
	copyOutput := func(stream string, reader io.Reader) {
		defer readers.Done()
		buffer := make([]byte, 4096)
		for {
			count, readErr := reader.Read(buffer)
			if count > 0 {
				data := make([]byte, count)
				copy(data, buffer[:count])
				chunks <- OutputChunk{Stream: stream, Data: data}
			}
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) {
					streamErrors <- fmt.Errorf("read %s: %w", stream, readErr)
				}
				return
			}
		}
	}

	readers.Add(2)
	go copyOutput("stdout", stdout)
	go copyOutput("stderr", stderr)
	go func() {
		readers.Wait()
		close(chunks)
		close(streamErrors)
	}()

	for chunk := range chunks {
		if onChunk != nil {
			onChunk(chunk)
		}
	}

	waitErr := cmd.Wait()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Result{}, fmt.Errorf("command canceled: %w", ctxErr)
	}

	for streamErr := range streamErrors {
		if streamErr != nil {
			return Result{}, streamErr
		}
	}

	if waitErr == nil {
		return Result{ExitCode: 0}, nil
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		return Result{ExitCode: exitErr.ExitCode()}, nil
	}
	return Result{}, fmt.Errorf("wait for command: %w", waitErr)
}
