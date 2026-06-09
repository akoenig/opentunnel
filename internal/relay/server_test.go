package relay

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const readTimeout = time.Second

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

	messageType, payload, err := readMessage(t, host)
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

func TestClientSlotReservedBeforeWebSocketUpgrade(t *testing.T) {
	server := NewServer()
	firstClientInUpgrade := make(chan struct{})
	releaseFirstClient := make(chan struct{})
	var checkOriginMu sync.Mutex
	seenFirstClient := false
	server.upgrader.CheckOrigin = func(r *http.Request) bool {
		isFirstClient := false
		checkOriginMu.Lock()
		if !seenFirstClient && r.URL.Query().Get("role") == "client" {
			seenFirstClient = true
			isFirstClient = true
		}
		checkOriginMu.Unlock()

		if isFirstClient {
			close(firstClientInUpgrade)
			<-releaseFirstClient
		}
		return true
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	host := dialTunnel(t, httpServer.URL, "host", "s1")
	defer host.Close()

	firstClientResult := make(chan dialResult, 1)
	go func() {
		conn, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL, "client", "s1"), nil)
		firstClientResult <- dialResult{conn: conn, response: response, err: err}
	}()

	select {
	case <-firstClientInUpgrade:
	case <-time.After(readTimeout):
		t.Fatalf("timed out waiting for first client upgrade")
	}

	secondClient, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL, "client", "s1"), nil)
	if err == nil {
		secondClient.Close()
		close(releaseFirstClient)
		t.Fatalf("expected second client dial to fail while first client slot is reserved")
	}
	if response == nil {
		close(releaseFirstClient)
		t.Fatalf("expected rejection response")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusSwitchingProtocols {
		close(releaseFirstClient)
		t.Fatalf("expected non-101 rejection response")
	}

	close(releaseFirstClient)
	firstClient := receiveDialResult(t, firstClientResult)
	defer firstClient.conn.Close()
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

type dialResult struct {
	conn     *websocket.Conn
	response *http.Response
	err      error
}

func receiveDialResult(t *testing.T, results <-chan dialResult) dialResult {
	t.Helper()

	select {
	case result := <-results:
		if result.err != nil {
			if result.response != nil {
				defer result.response.Body.Close()
				t.Fatalf("dial: %v status=%s", result.err, result.response.Status)
			}
			t.Fatalf("dial: %v", result.err)
		}
		return result
	case <-time.After(readTimeout):
		t.Fatalf("timed out waiting for dial result")
	}
	panic("unreachable")
}

func readMessage(t *testing.T, conn *websocket.Conn) (int, []byte, error) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	return conn.ReadMessage()
}

func tunnelURL(serverURL, role, session string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + "/tunnel?role=" + role + "&session=" + session
}
