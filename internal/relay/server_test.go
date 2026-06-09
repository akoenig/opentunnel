package relay

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestClientBinaryMessageReachesHostUnchanged(t *testing.T) {
	server := NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	host := dialTunnel(t, httpServer.URL, "host", "s1")
	defer host.Close()
	client := dialTunnel(t, httpServer.URL, "client", "s1")
	defer client.Close()

	frame := []byte{0, 1, 2, 3, 255, 4}
	if err := client.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		t.Fatalf("client write message: %v", err)
	}

	messageType, payload, err := host.ReadMessage()
	if err != nil {
		t.Fatalf("host read message: %v", err)
	}
	if messageType != websocket.BinaryMessage {
		t.Fatalf("host message type mismatch: got %d want %d", messageType, websocket.BinaryMessage)
	}
	if !bytes.Equal(payload, frame) {
		t.Fatalf("host payload mismatch: got %v want %v", payload, frame)
	}
}

func TestSecondClientForSameSessionIsRejected(t *testing.T) {
	server := NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	host := dialTunnel(t, httpServer.URL, "host", "s1")
	defer host.Close()
	client := dialTunnel(t, httpServer.URL, "client", "s1")
	defer client.Close()

	_, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL, "client", "s1"), nil)
	if err == nil {
		t.Fatalf("expected second client dial to fail")
	}
	if response == nil {
		t.Fatalf("expected rejection response")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusSwitchingProtocols {
		t.Fatalf("expected non-101 rejection response")
	}
}

func TestClientBeforeHostIsRejected(t *testing.T) {
	server := NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	_, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL, "client", "s1"), nil)
	if err == nil {
		t.Fatalf("expected client-before-host dial to fail")
	}
	if response == nil {
		t.Fatalf("expected rejection response")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusSwitchingProtocols {
		t.Fatalf("expected non-101 rejection response")
	}
}

func dialTunnel(t *testing.T, serverURL, role, session string) *websocket.Conn {
	t.Helper()

	conn, response, err := websocket.DefaultDialer.Dial(tunnelURL(serverURL, role, session), nil)
	if err != nil {
		if response != nil {
			defer response.Body.Close()
			t.Fatalf("dial %s: %v status=%s", role, err, response.Status)
		}
		t.Fatalf("dial %s: %v", role, err)
	}
	return conn
}

func tunnelURL(serverURL, role, session string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + "/tunnel?role=" + role + "&session=" + session
}
