//go:build unix

package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunTerminatesProcessGroupGracefullyBeforeKilling(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "term-marker")
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		_, err := Run(ctx, fmt.Sprintf("trap 'printf term > %s; exit 0' TERM; while true; do sleep 1; done", marker), nil)
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after context cancellation")
	}

	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read termination marker: %v", err)
	}
	if string(data) != "term" {
		t.Fatalf("termination marker = %q, want term", string(data))
	}
}
