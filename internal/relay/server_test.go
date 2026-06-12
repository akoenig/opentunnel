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

	"opentunnel/internal/artifact"

	"github.com/gorilla/websocket"
)

const readTimeout = time.Second

func TestNewServerWithOptionsServesCLIArtifacts(t *testing.T) {
	version := "4.5.6"
	artifactDir := writeTestArtifactDir(t, version)
	server, err := NewServerWithOptions(ServerOptions{
		PublicURL:   "https://relay.example.com",
		Version:     version,
		ArtifactDir: artifactDir,
	})
	if err != nil {
		t.Fatalf("NewServerWithOptions() error = %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	bootstrapResponse, bootstrapBody := getRelayPath(t, httpServer.URL, "/cli")
	defer closeResponseBody(t, bootstrapResponse)
	if bootstrapResponse.StatusCode != http.StatusOK {
		t.Fatalf("bootstrap status mismatch: got %d want %d", bootstrapResponse.StatusCode, http.StatusOK)
	}
	contentType := bootstrapResponse.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/x-shellscript") && !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("content type mismatch: got %q", contentType)
	}
	for _, want := range []string{
		"relay_origin='https://relay.example.com'",
		"version='" + version + "'",
	} {
		if !strings.Contains(string(bootstrapBody), want) {
			t.Fatalf("bootstrap missing %q in:\n%s", want, string(bootstrapBody))
		}
	}

	for _, platform := range artifact.SupportedPlatforms() {
		binary := []byte("binary " + platform)
		checksum := testSHA256(binary)
		binaryPath := "/cli/bin/opentunnel-" + version + "-" + platform
		checksumPath := binaryPath + ".sha256"
		for _, want := range []string{
			platform + ") expected_checksum='" + checksum + "'",
		} {
			if !strings.Contains(string(bootstrapBody), want) {
				t.Fatalf("bootstrap missing %q in:\n%s", want, string(bootstrapBody))
			}
		}

		binaryResponse, binaryBody := getRelayPath(t, httpServer.URL, binaryPath)
		defer closeResponseBody(t, binaryResponse)
		if binaryResponse.StatusCode != http.StatusOK {
			t.Fatalf("%s binary status mismatch: got %d want %d", platform, binaryResponse.StatusCode, http.StatusOK)
		}
		if contentType := binaryResponse.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/octet-stream") {
			t.Fatalf("%s binary content type mismatch: got %q", platform, contentType)
		}
		if !bytes.Equal(binaryBody, binary) {
			t.Fatalf("%s binary body mismatch: got %v want %v", platform, binaryBody, binary)
		}

		checksumResponse, checksumBody := getRelayPath(t, httpServer.URL, checksumPath)
		defer closeResponseBody(t, checksumResponse)
		if checksumResponse.StatusCode != http.StatusOK {
			t.Fatalf("%s checksum status mismatch: got %d want %d", platform, checksumResponse.StatusCode, http.StatusOK)
		}
		if contentType := checksumResponse.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
			t.Fatalf("%s checksum content type mismatch: got %q", platform, contentType)
		}
		if got := strings.TrimSpace(string(checksumBody)); got != checksum {
			t.Fatalf("%s checksum body mismatch: got %q want %q", platform, got, checksum)
		}
	}
}

func TestNewServerWithOptionsRejectsInvalidCLIArtifactOptions(t *testing.T) {
	artifactDir := writeTestArtifactDir(t, "1.2.3")
	incompleteArtifactDir := writeTestArtifactDir(t, "1.2.3")
	missingPath, err := artifact.ArtifactPath(incompleteArtifactDir, "1.2.3", "linux-amd64")
	if err != nil {
		t.Fatalf("artifact path: %v", err)
	}
	if err := os.Remove(missingPath); err != nil {
		t.Fatalf("remove test artifact: %v", err)
	}
	tests := []struct {
		name    string
		options ServerOptions
	}{
		{
			name: "public url without artifact dir",
			options: ServerOptions{
				PublicURL: "https://relay.example.com",
				Version:   "1.2.3",
			},
		},
		{
			name: "public url without version",
			options: ServerOptions{
				PublicURL:   "https://relay.example.com",
				ArtifactDir: artifactDir,
			},
		},
		{
			name: "invalid public url",
			options: ServerOptions{
				PublicURL:   "https://relay.example.com/path",
				Version:     "1.2.3",
				ArtifactDir: artifactDir,
			},
		},
		{
			name: "missing artifact dir",
			options: ServerOptions{
				PublicURL:   "https://relay.example.com",
				Version:     "1.2.3",
				ArtifactDir: filepath.Join(t.TempDir(), "missing-artifacts"),
			},
		},
		{
			name: "incomplete artifact dir",
			options: ServerOptions{
				PublicURL:   "https://relay.example.com",
				Version:     "1.2.3",
				ArtifactDir: incompleteArtifactDir,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewServerWithOptions(tt.options)
			if err == nil {
				t.Fatal("NewServerWithOptions() error = nil, want error")
			}
		})
	}
}

func TestUnknownCLIArtifactPathReturnsNotFound(t *testing.T) {
	version := "1.2.3"
	artifactDir := writeTestArtifactDir(t, version)
	if err := os.WriteFile(filepath.Join(artifactDir, "extra-file"), []byte("secret"), 0o600); err != nil {
		t.Fatalf("write extra file: %v", err)
	}
	server, err := NewServerWithOptions(ServerOptions{
		PublicURL:   "http://relay.example.com",
		Version:     version,
		ArtifactDir: artifactDir,
	})
	if err != nil {
		t.Fatalf("NewServerWithOptions() error = %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	for _, path := range []string{
		"/cli/bin/opentunnel-" + version + "-windows-amd64",
		"/cli/bin/opentunnel-" + version + "-linux-amd64.exe",
		"/cli/bin/extra-file",
	} {
		response, _ := getRelayPath(t, httpServer.URL, path)
		defer closeResponseBody(t, response)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s status mismatch: got %d want %d", path, response.StatusCode, http.StatusNotFound)
		}
	}
}

func TestClientBinaryMessageReachesHostUnchanged(t *testing.T) {
	server := NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	host := dialTunnel(t, httpServer.URL, "host", "s1")
	defer closeWebSocket(t, host)
	client := dialTunnel(t, httpServer.URL, "client", "s1")
	defer closeWebSocket(t, client)

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

func TestWebSocketWithOriginIsRejected(t *testing.T) {
	server := NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	header := tunnelHeader("host", "s1")
	header.Set("Origin", "https://browser.example")
	conn, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL), header)
	if err == nil {
		_ = conn.Close()
		t.Fatalf("expected dial with Origin to fail")
	}
	if response == nil {
		t.Fatalf("expected rejection response")
	}
	defer closeResponseBody(t, response)
	if response.StatusCode == http.StatusSwitchingProtocols {
		t.Fatalf("expected non-101 rejection response")
	}
}

func TestWebSocketWithoutOriginUpgrades(t *testing.T) {
	server := NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	host := dialTunnel(t, httpServer.URL, "host", "s1")
	defer closeWebSocket(t, host)
}

func TestSessionCapRejectsNewHostSessions(t *testing.T) {
	server, err := NewServerWithOptions(ServerOptions{MaxSessions: 1})
	if err != nil {
		t.Fatalf("NewServerWithOptions() error = %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	host := dialTunnel(t, httpServer.URL, "host", "s1")
	defer closeWebSocket(t, host)

	secondHost, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL), tunnelHeader("host", "s2"))
	if err == nil {
		_ = secondHost.Close()
		t.Fatalf("expected second host session to fail")
	}
	if response == nil {
		t.Fatalf("expected rejection response")
	}
	defer closeResponseBody(t, response)
	if response.StatusCode == http.StatusSwitchingProtocols {
		t.Fatalf("expected non-101 rejection response")
	}
}

func TestReservationTTLReapsUnattachedHostReservation(t *testing.T) {
	server, err := NewServerWithOptions(ServerOptions{MaxSessions: 1, ReservationTTL: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("NewServerWithOptions() error = %v", err)
	}
	firstHostInUpgrade := make(chan struct{})
	releaseFirstHost := make(chan struct{})
	var checkOriginMu sync.Mutex
	seenFirstHost := false
	server.upgrader.CheckOrigin = func(r *http.Request) bool {
		isFirstHost := false
		checkOriginMu.Lock()
		if !seenFirstHost && r.Header.Get(tunnelRoleHeader) == "host" {
			seenFirstHost = true
			isFirstHost = true
		}
		checkOriginMu.Unlock()

		if isFirstHost {
			close(firstHostInUpgrade)
			<-releaseFirstHost
		}
		return true
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	firstHostResult := make(chan dialResult, 1)
	go func() {
		conn, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL), tunnelHeader("host", "s1"))
		firstHostResult <- dialResult{conn: conn, response: response, err: err}
	}()

	select {
	case <-firstHostInUpgrade:
	case <-time.After(readTimeout):
		t.Fatalf("timed out waiting for first host upgrade")
	}
	time.Sleep(20 * time.Millisecond)

	secondHost := dialTunnel(t, httpServer.URL, "host", "s2")
	defer closeWebSocket(t, secondHost)

	close(releaseFirstHost)
	firstHost := receiveDialResult(t, firstHostResult)
	defer closeWebSocket(t, firstHost.conn)
}

func TestReapedReservationCannotAttachToReplacementSession(t *testing.T) {
	server, err := NewServerWithOptions(ServerOptions{ReservationTTL: time.Nanosecond})
	if err != nil {
		t.Fatalf("NewServerWithOptions() error = %v", err)
	}

	staleReservation, ok := server.reserve("host", "s1")
	if !ok {
		t.Fatalf("reserve stale host session failed")
	}
	staleReservation.reservedAt = time.Now().Add(-time.Second)
	server.mu.Lock()
	server.reapExpiredReservationsLocked(time.Now())
	server.mu.Unlock()

	replacementReservation, ok := server.reserve("host", "s1")
	if !ok {
		t.Fatalf("reserve replacement host session failed")
	}
	if replacementReservation == staleReservation {
		t.Fatalf("expected replacement reservation to use a new session")
	}

	if server.attach("host", "s1", staleReservation, &websocket.Conn{}) {
		t.Fatalf("expected stale reservation attach to fail")
	}
	if replacementReservation.host != nil {
		t.Fatalf("stale reservation attached to replacement session")
	}
}

func TestOversizedFrameIsRejected(t *testing.T) {
	server, err := NewServerWithOptions(ServerOptions{MaxFrameBytes: 8})
	if err != nil {
		t.Fatalf("NewServerWithOptions() error = %v", err)
	}
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	host := dialTunnel(t, httpServer.URL, "host", "s1")
	defer closeWebSocket(t, host)
	client := dialTunnel(t, httpServer.URL, "client", "s1")
	defer closeWebSocket(t, client)

	if err := client.WriteMessage(websocket.BinaryMessage, []byte("0123456789abcdef")); err != nil {
		t.Fatalf("client write message: %v", err)
	}
	_, _, err = readMessage(t, host)
	if err == nil {
		t.Fatalf("expected host read to fail after oversized client frame")
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
		if !seenFirstClient && r.Header.Get(tunnelRoleHeader) == "client" {
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
	defer closeWebSocket(t, host)

	firstClientResult := make(chan dialResult, 1)
	go func() {
		conn, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL), tunnelHeader("client", "s1"))
		firstClientResult <- dialResult{conn: conn, response: response, err: err}
	}()

	select {
	case <-firstClientInUpgrade:
	case <-time.After(readTimeout):
		t.Fatalf("timed out waiting for first client upgrade")
	}

	secondClient, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL), tunnelHeader("client", "s1"))
	if err == nil {
		_ = secondClient.Close()
		close(releaseFirstClient)
		t.Fatalf("expected second client dial to fail while first client slot is reserved")
	}
	if response == nil {
		close(releaseFirstClient)
		t.Fatalf("expected rejection response")
	}
	defer closeResponseBody(t, response)
	if response.StatusCode == http.StatusSwitchingProtocols {
		close(releaseFirstClient)
		t.Fatalf("expected non-101 rejection response")
	}

	close(releaseFirstClient)
	firstClient := receiveDialResult(t, firstClientResult)
	defer closeWebSocket(t, firstClient.conn)
}

func TestSecondClientForSameSessionIsRejected(t *testing.T) {
	server := NewServer()
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	host := dialTunnel(t, httpServer.URL, "host", "s1")
	defer closeWebSocket(t, host)
	client := dialTunnel(t, httpServer.URL, "client", "s1")
	defer closeWebSocket(t, client)

	_, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL), tunnelHeader("client", "s1"))
	if err == nil {
		t.Fatalf("expected second client dial to fail")
	}
	if response == nil {
		t.Fatalf("expected rejection response")
	}
	defer closeResponseBody(t, response)
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

	_, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL), tunnelHeader("client", "s1"))
	if err == nil {
		t.Fatalf("expected new client without a new host to fail")
	}
	if response == nil {
		t.Fatalf("expected rejection response")
	}
	defer closeResponseBody(t, response)
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

	_, response, err := websocket.DefaultDialer.Dial(tunnelURL(httpServer.URL), tunnelHeader("client", "s1"))
	if err == nil {
		t.Fatalf("expected client-before-host dial to fail")
	}
	if response == nil {
		t.Fatalf("expected rejection response")
	}
	defer closeResponseBody(t, response)
	if response.StatusCode == http.StatusSwitchingProtocols {
		t.Fatalf("expected non-101 rejection response")
	}
}

func dialTunnel(t *testing.T, serverURL, role, session string) *websocket.Conn {
	t.Helper()

	conn, response, err := websocket.DefaultDialer.Dial(tunnelURL(serverURL), tunnelHeader(role, session))
	if err != nil {
		if response != nil {
			defer closeResponseBody(t, response)
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
				defer closeResponseBody(t, result.response)
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

func writeTestArtifactDir(t *testing.T, version string) string {
	t.Helper()

	dir := t.TempDir()
	for _, platform := range artifact.SupportedPlatforms() {
		path, err := artifact.ArtifactPath(dir, version, platform)
		if err != nil {
			t.Fatalf("artifact path: %v", err)
		}
		if err := os.WriteFile(path, []byte("binary "+platform), 0o600); err != nil {
			t.Fatalf("write test artifact: %v", err)
		}
	}
	return dir
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
		_ = response.Body.Close()
		t.Fatalf("read %s body: %v", path, err)
	}
	return response, body
}

func closeResponseBody(t *testing.T, response *http.Response) {
	t.Helper()
	if err := response.Body.Close(); err != nil {
		t.Fatalf("close response body: %v", err)
	}
}

func closeWebSocket(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	if err := conn.Close(); err != nil {
		t.Fatalf("close websocket: %v", err)
	}
}

func tunnelURL(serverURL string) string {
	return "ws" + strings.TrimPrefix(serverURL, "http") + "/tunnel"
}

func tunnelHeader(role, session string) http.Header {
	header := http.Header{}
	header.Set(tunnelRoleHeader, role)
	header.Set(tunnelSessionHeader, session)
	return header
}
