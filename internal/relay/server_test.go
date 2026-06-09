package relay

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const readTimeout = time.Second

func TestCLIBootstrapUsesConfiguredArtifactCoordinates(t *testing.T) {
	binaryPath := writeTestBinary(t, []byte("binary bytes"))
	checksum := testSHA256([]byte("binary bytes"))
	server := NewServer(WithCLIArtifacts(CLIArtifacts{
		RelayOrigin: "https://relay.example.com",
		Version:     "1.2.3",
		PlatformKey: "linux-amd64",
		BinaryPath:  binaryPath,
	}))
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	response, body := getRelayPath(t, httpServer.URL, "/cli")
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status mismatch: got %d want %d", response.StatusCode, http.StatusOK)
	}
	contentType := response.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/x-shellscript") && !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("content type mismatch: got %q", contentType)
	}
	for _, want := range []string{
		"relay_origin='https://relay.example.com'",
		"version='1.2.3'",
		"platform='linux-amd64'",
		"expected_checksum='" + checksum + "'",
		"/cli/bin/opentunnel-1.2.3-linux-amd64",
		"/cli/bin/opentunnel-1.2.3-linux-amd64.sha256",
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("bootstrap missing %q in:\n%s", want, string(body))
		}
	}
}

func TestCLIBinaryAndChecksumAreServedFromConfiguredArtifact(t *testing.T) {
	binary := []byte{0, 1, 2, 3, 255}
	binaryPath := writeTestBinary(t, binary)
	checksum := testSHA256(binary)
	server := NewServer(WithCLIArtifacts(CLIArtifacts{
		RelayOrigin: "http://relay.example.com",
		Version:     "9.8.7",
		PlatformKey: "darwin-arm64",
		BinaryPath:  binaryPath,
	}))
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	binaryResponse, binaryBody := getRelayPath(t, httpServer.URL, "/cli/bin/opentunnel-9.8.7-darwin-arm64")
	defer binaryResponse.Body.Close()
	if binaryResponse.StatusCode != http.StatusOK {
		t.Fatalf("binary status mismatch: got %d want %d", binaryResponse.StatusCode, http.StatusOK)
	}
	if contentType := binaryResponse.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/octet-stream") {
		t.Fatalf("binary content type mismatch: got %q", contentType)
	}
	if !bytes.Equal(binaryBody, binary) {
		t.Fatalf("binary body mismatch: got %v want %v", binaryBody, binary)
	}

	checksumResponse, checksumBody := getRelayPath(t, httpServer.URL, "/cli/bin/opentunnel-9.8.7-darwin-arm64.sha256")
	defer checksumResponse.Body.Close()
	if checksumResponse.StatusCode != http.StatusOK {
		t.Fatalf("checksum status mismatch: got %d want %d", checksumResponse.StatusCode, http.StatusOK)
	}
	if contentType := checksumResponse.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("checksum content type mismatch: got %q", contentType)
	}
	if got := strings.TrimSpace(string(checksumBody)); got != checksum {
		t.Fatalf("checksum body mismatch: got %q want %q", got, checksum)
	}
}

func TestUnknownCLIArtifactPathReturnsNotFound(t *testing.T) {
	binaryPath := writeTestBinary(t, []byte("binary bytes"))
	server := NewServer(WithCLIArtifacts(CLIArtifacts{
		RelayOrigin: "http://relay.example.com",
		Version:     "1.2.3",
		PlatformKey: "linux-amd64",
		BinaryPath:  binaryPath,
	}))
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	response, _ := getRelayPath(t, httpServer.URL, "/cli/bin/opentunnel-1.2.3-linux-arm64")
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status mismatch: got %d want %d", response.StatusCode, http.StatusNotFound)
	}
}

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

func TestClientDisconnectClosesHostAndRemovesSession(t *testing.T) {
	server := NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	host := dialTunnel(t, httpServer.URL, "host", "s1")
	client := dialTunnel(t, httpServer.URL, "client", "s1")

	if err := client.Close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	_, _, err := readMessage(t, host)
	if err == nil {
		t.Fatalf("expected host read to fail after paired client disconnect")
	}

	_, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL, "client", "s1"), nil)
	if err == nil {
		t.Fatalf("expected new client without a new host to fail")
	}
	if response == nil {
		t.Fatalf("expected rejection response")
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusSwitchingProtocols {
		t.Fatalf("expected non-101 rejection response")
	}
}

func TestCleanupRemovesWholeSessionAtomically(t *testing.T) {
	server := NewServer()
	host := &websocket.Conn{}
	client := &websocket.Conn{}
	server.sessions["s1"] = &session{
		host:           host,
		client:         client,
		hostReserved:   true,
		clientReserved: true,
	}

	peer := server.releaseConnection("client", "s1", client)
	if peer != host {
		t.Fatalf("peer mismatch: got %p want %p", peer, host)
	}
	if _, ok := server.sessions["s1"]; ok {
		t.Fatalf("expected cleanup to remove whole session")
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

func writeTestBinary(t *testing.T, contents []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "opentunnel")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write test binary: %v", err)
	}
	return path
}

func testSHA256(contents []byte) string {
	hash := sha256.Sum256(contents)
	return hex.EncodeToString(hash[:])
}

func getRelayPath(t *testing.T, serverURL, path string) (*http.Response, []byte) {
	t.Helper()

	response, err := http.Get(serverURL + path)
	if err != nil {
		t.Fatalf("get %s: %v", path, err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		response.Body.Close()
		t.Fatalf("read %s body: %v", path, err)
	}
	return response, body
}

func tunnelURL(serverURL, role, session string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + "/tunnel?role=" + role + "&session=" + session
}
