package command

import (
	"context"
	"errors"
	"testing"
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

func TestRunReturnsExitCodeForNonZeroExit(t *testing.T) {
	result, err := Run(context.Background(), "exit 7", nil)

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("ExitCode = %d, want 7", result.ExitCode)
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
