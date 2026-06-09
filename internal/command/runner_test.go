package command

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunCapturesStdoutChunk(t *testing.T) {
	var chunks []OutputChunk

	result, err := Run(context.Background(), "printf hello", func(chunk OutputChunk) {
		chunks = append(chunks, chunk)
	})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].Stream != "stdout" {
		t.Fatalf("Stream = %q, want stdout", chunks[0].Stream)
	}
	if string(chunks[0].Data) != "hello" {
		t.Fatalf("Data = %q, want hello", string(chunks[0].Data))
	}
}

func TestRunCapturesStderrSeparately(t *testing.T) {
	var chunks []OutputChunk

	result, err := Run(context.Background(), "printf err >&2", func(chunk OutputChunk) {
		chunks = append(chunks, chunk)
	})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if chunks[0].Stream != "stderr" {
		t.Fatalf("Stream = %q, want stderr", chunks[0].Stream)
	}
	if string(chunks[0].Data) != "err" {
		t.Fatalf("Data = %q, want err", string(chunks[0].Data))
	}
}

func TestRunSerializesOutputCallbacks(t *testing.T) {
	var inCallback atomic.Bool
	var concurrentCallback atomic.Bool

	result, err := Run(context.Background(), "i=0; while [ \"$i\" -lt 100 ]; do printf o; printf e >&2; i=$((i + 1)); done", func(OutputChunk) {
		if inCallback.Swap(true) {
			concurrentCallback.Store(true)
		}
		time.Sleep(time.Millisecond)
		inCallback.Store(false)
	})

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", result.ExitCode)
	}
	if concurrentCallback.Load() {
		t.Fatal("onChunk was invoked concurrently")
	}
}

func TestRunReturnsExitCodeForNonZeroExit(t *testing.T) {
	result, err := Run(context.Background(), "exit 7", nil)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", result.ExitCode)
	}
}

func TestRunCancelsCommandOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)

	started := time.Now()
	go func() {
		_, err := Run(ctx, "sleep 2", nil)
		done <- err
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()

	var err error
	select {
	case err = <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Run did not return promptly after context cancellation")
	}
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("Run returned nil error, want cancellation error")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run error = %v, want context cancellation error", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("Run returned after %s, want prompt cancellation", elapsed)
	}
}

func TestRunRejectsEmptyCommand(t *testing.T) {
	_, err := Run(context.Background(), "", nil)

	if err == nil {
		t.Fatal("Run returned nil error, want error")
	}
	if errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned context cancellation error for empty command: %v", err)
	}
}
