package relay

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestActiveTunnelsCountsSessionsWithConnectedHosts(t *testing.T) {
	server := NewServer()

	server.mu.Lock()
	server.sessions["host-attached"] = &session{hostReserved: true, host: &websocket.Conn{}}
	server.sessions["host-and-client"] = &session{
		hostReserved:   true,
		clientReserved: true,
		host:           &websocket.Conn{},
		client:         &websocket.Conn{},
	}
	server.sessions["reserved-only"] = &session{hostReserved: true, reservedAt: time.Now()}
	server.sessions["client-only"] = &session{clientReserved: true, client: &websocket.Conn{}}
	server.mu.Unlock()

	if got, want := server.ActiveTunnels(), 2; got != want {
		t.Fatalf("active tunnels: got %d want %d", got, want)
	}
}

func TestActiveTunnelsIsZeroOnFreshServer(t *testing.T) {
	if got := NewServer().ActiveTunnels(); got != 0 {
		t.Fatalf("active tunnels on fresh server: got %d want 0", got)
	}
}

func TestWriteActiveTunnelsFormat(t *testing.T) {
	server := NewServer()
	server.mu.Lock()
	server.sessions["one"] = &session{hostReserved: true, host: &websocket.Conn{}}
	server.mu.Unlock()

	var buffer bytes.Buffer
	server.writeActiveTunnels(&buffer)

	if got, want := buffer.String(), "relay: active tunnels: 1\n"; got != want {
		t.Fatalf("log line: got %q want %q", got, want)
	}
}

func TestLogActiveTunnelsStopsWhenContextCanceled(t *testing.T) {
	server := NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		server.LogActiveTunnels(ctx, time.Hour, &bytes.Buffer{})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(readTimeout):
		t.Fatal("LogActiveTunnels did not stop after context cancellation")
	}
}

func TestLogActiveTunnelsWritesOnTick(t *testing.T) {
	server := NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writes := make(chan string, 1)
	go server.LogActiveTunnels(ctx, time.Millisecond, channelWriter{writes: writes})

	select {
	case line := <-writes:
		if !strings.HasPrefix(line, "relay: active tunnels: ") {
			t.Fatalf("unexpected log line: %q", line)
		}
	case <-time.After(readTimeout):
		t.Fatal("LogActiveTunnels did not write within the deadline")
	}
}

type channelWriter struct {
	writes chan string
}

func (w channelWriter) Write(p []byte) (int, error) {
	select {
	case w.writes <- string(p):
	default:
	}
	return len(p), nil
}

func TestHealthHandlerReportsActiveTunnels(t *testing.T) {
	server := NewServer()
	server.mu.Lock()
	server.sessions["one"] = &session{hostReserved: true, host: &websocket.Conn{}}
	server.mu.Unlock()

	recorder := httptest.NewRecorder()
	server.HealthHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("healthz status: got %d want %d", recorder.Code, http.StatusOK)
	}
	if got, want := recorder.Body.String(), "active tunnels: 1\n"; got != want {
		t.Fatalf("healthz body: got %q want %q", got, want)
	}
}

func TestHealthHandlerRejectsNonGET(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewServer().HealthHandler().ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/healthz", nil))

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("healthz POST status: got %d want %d", recorder.Code, http.StatusMethodNotAllowed)
	}
}

func TestTunnelHandlerDoesNotServeHealth(t *testing.T) {
	recorder := httptest.NewRecorder()
	NewServer().Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("public handler healthz status: got %d want %d", recorder.Code, http.StatusNotFound)
	}
}
