package relay

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ActiveTunnelLogInterval is how often the relay reports the number of active
// tunnels.
const ActiveTunnelLogInterval = 30 * time.Second

// ActiveTunnels reports the number of sessions with a connected host. A
// tunnel counts as active from the moment the host websocket is attached
// until it disconnects, independent of whether a client is connected.
func (s *Server) ActiveTunnels() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, tunnelSession := range s.sessions {
		if tunnelSession.host != nil {
			count++
		}
	}
	return count
}

// LogActiveTunnels writes the active tunnel count to writer at the given
// interval until ctx is canceled.
func (s *Server) LogActiveTunnels(ctx context.Context, interval time.Duration, writer io.Writer) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.writeActiveTunnels(writer)
		}
	}
}

func (s *Server) writeActiveTunnels(writer io.Writer) {
	_, _ = fmt.Fprintf(writer, "relay: active tunnels: %d\n", s.ActiveTunnels())
}

// HealthHandler returns a handler serving GET /healthz with the aggregate
// active tunnel count. The count is operational telemetry, not public
// information: the handler is deliberately not part of Handler() and is
// served only when the relay is started with --health-listen, on a separate
// address that operators keep private.
func (s *Server) HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = fmt.Fprintf(w, "active tunnels: %d\n", s.ActiveTunnels())
	})
}
