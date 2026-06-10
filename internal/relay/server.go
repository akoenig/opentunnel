package relay

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"opentunnel/internal/artifact"

	"github.com/gorilla/websocket"
)

// Server routes opaque websocket tunnel frames between one host and one client per session.
type Server struct {
	mu           sync.Mutex
	sessions     map[string]*session
	upgrader     websocket.Upgrader
	cliArtifacts *CLIArtifacts
}

// CLIArtifact configures one optional CLI artifact response served by the relay.
type CLIArtifact struct {
	PlatformKey string
	BinaryPath  string
	Checksum    string
}

// CLIArtifacts configures optional CLI artifact responses served by the relay.
type CLIArtifacts struct {
	RelayOrigin string
	Version     string
	Artifacts   map[string]CLIArtifact
}

// ServerOptions configures a relay server with plan-defined options.
type ServerOptions struct {
	PublicURL   string
	Version     string
	ArtifactDir string
}

type session struct {
	host           *websocket.Conn
	client         *websocket.Conn
	hostReserved   bool
	clientReserved bool
}

// NewServer creates an in-memory relay server.
func NewServer() *Server {
	return newServer()
}

// NewServerWithOptions creates an in-memory relay server from explicit options.
func NewServerWithOptions(options ServerOptions) (*Server, error) {
	server := newServer()
	if options.PublicURL == "" && options.Version == "" && options.ArtifactDir == "" {
		return server, nil
	}

	artifacts, err := loadCLIArtifacts(options.PublicURL, options.Version, options.ArtifactDir)
	if err != nil {
		return nil, err
	}
	server.cliArtifacts = artifacts
	return server, nil
}

func loadCLIArtifacts(relayOrigin, version, artifactDir string) (*CLIArtifacts, error) {
	artifacts := CLIArtifacts{
		RelayOrigin: relayOrigin,
		Version:     version,
		Artifacts:   make(map[string]CLIArtifact, len(artifact.SupportedPlatforms())),
	}
	bootstrapArtifacts := make([]artifact.BootstrapArtifact, 0, len(artifact.SupportedPlatforms()))

	for _, platform := range artifact.SupportedPlatforms() {
		binaryPath, err := artifact.ArtifactPath(artifactDir, version, platform)
		if err != nil {
			return nil, fmt.Errorf("validate cli artifacts: %w", err)
		}
		checksum, err := artifact.SHA256File(binaryPath)
		if err != nil {
			return nil, fmt.Errorf("validate cli artifacts: %w", err)
		}
		artifacts.Artifacts[platform] = CLIArtifact{
			PlatformKey: platform,
			BinaryPath:  binaryPath,
			Checksum:    checksum,
		}
		bootstrapArtifacts = append(bootstrapArtifacts, artifact.BootstrapArtifact{
			PlatformKey: platform,
			Checksum:    checksum,
		})
	}

	if _, err := artifact.RenderBootstrap(artifact.BootstrapConfig{
		RelayOrigin: artifacts.RelayOrigin,
		Version:     artifacts.Version,
		Artifacts:   bootstrapArtifacts,
	}); err != nil {
		return nil, fmt.Errorf("validate cli artifacts: %w", err)
	}

	return &artifacts, nil
}

func newServer() *Server {
	return &Server{
		sessions: make(map[string]*session),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// Handler returns the HTTP handler for the relay tunnel endpoint.
func (s *Server) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.handleCLIArtifact(w, r) {
			return
		}

		if r.URL.Path != "/tunnel" {
			http.NotFound(w, r)
			return
		}

		role := r.URL.Query().Get("role")
		sessionID := r.URL.Query().Get("session")
		if sessionID == "" || (role != "host" && role != "client") {
			http.Error(w, "invalid tunnel request", http.StatusBadRequest)
			return
		}
		if !s.reserve(role, sessionID) {
			http.Error(w, "session endpoint unavailable", http.StatusConflict)
			return
		}

		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			s.release(role, sessionID)
			return
		}
		s.attach(role, sessionID, conn)

		s.forward(role, sessionID, conn)
	})
}

func (s *Server) handleCLIArtifact(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodGet || (r.URL.Path != "/cli" && !strings.HasPrefix(r.URL.Path, "/cli/bin/")) {
		return false
	}
	if s.cliArtifacts == nil {
		http.NotFound(w, r)
		return true
	}

	if r.URL.Path == "/cli" {
		s.serveBootstrap(w)
		return true
	}

	for _, platform := range artifact.SupportedPlatforms() {
		cliArtifact := s.cliArtifacts.Artifacts[platform]
		binaryPath := cliArtifactURLPath(s.cliArtifacts.Version, platform)
		if r.URL.Path == binaryPath {
			s.serveBinary(w, cliArtifact)
			return true
		}
		if r.URL.Path == binaryPath+".sha256" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprintln(w, cliArtifact.Checksum)
			return true
		}
	}

	http.NotFound(w, r)
	return true
}

func (s *Server) serveBootstrap(w http.ResponseWriter) {
	bootstrapArtifacts := make([]artifact.BootstrapArtifact, 0, len(artifact.SupportedPlatforms()))
	for _, platform := range artifact.SupportedPlatforms() {
		cliArtifact := s.cliArtifacts.Artifacts[platform]
		bootstrapArtifacts = append(bootstrapArtifacts, artifact.BootstrapArtifact{
			PlatformKey: platform,
			Checksum:    cliArtifact.Checksum,
		})
	}

	script, err := artifact.RenderBootstrap(artifact.BootstrapConfig{
		RelayOrigin: s.cliArtifacts.RelayOrigin,
		Version:     s.cliArtifacts.Version,
		Artifacts:   bootstrapArtifacts,
	})
	if err != nil {
		http.Error(w, "artifact unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	fmt.Fprint(w, script)
}

func (s *Server) serveBinary(w http.ResponseWriter, cliArtifact CLIArtifact) {
	contents, err := os.ReadFile(cliArtifact.BinaryPath)
	if err != nil {
		http.Error(w, "artifact unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(contents)
}

func cliArtifactURLPath(version, platform string) string {
	return "/cli/bin/opentunnel-" + version + "-" + platform
}

func (s *Server) reserve(role, sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if role == "host" {
		tunnelSession := s.sessions[sessionID]
		if tunnelSession != nil && tunnelSession.hostReserved {
			return false
		}
		if tunnelSession == nil {
			tunnelSession = &session{}
			s.sessions[sessionID] = tunnelSession
		}
		tunnelSession.hostReserved = true
		return true
	}

	tunnelSession := s.sessions[sessionID]
	if tunnelSession == nil || !tunnelSession.hostReserved || tunnelSession.clientReserved {
		return false
	}
	tunnelSession.clientReserved = true
	return true
}

func (s *Server) attach(role, sessionID string, conn *websocket.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tunnelSession := s.sessions[sessionID]
	if tunnelSession == nil {
		return
	}
	if role == "host" {
		tunnelSession.host = conn
		return
	}
	tunnelSession.client = conn
}

func (s *Server) forward(role, sessionID string, conn *websocket.Conn) {
	defer s.disconnect(role, sessionID, conn)

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.BinaryMessage {
			return
		}

		peer := s.peer(role, sessionID, conn)
		if peer == nil {
			continue
		}
		if err := peer.WriteMessage(websocket.BinaryMessage, payload); err != nil {
			return
		}
	}
}

func (s *Server) peer(role, sessionID string, conn *websocket.Conn) *websocket.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil {
		return nil
	}
	if role == "host" && session.host == conn {
		return session.client
	}
	if role == "client" && session.client == conn {
		return session.host
	}
	return nil
}

func (s *Server) disconnect(role, sessionID string, conn *websocket.Conn) {
	peer := s.releaseConnection(role, sessionID, conn)
	conn.Close()
	if peer != nil {
		peer.Close()
	}
}

func (s *Server) release(role, sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tunnelSession := s.sessions[sessionID]
	if tunnelSession == nil {
		return
	}
	if role == "host" {
		tunnelSession.hostReserved = false
	} else {
		tunnelSession.clientReserved = false
	}
	s.deleteEmptySession(sessionID, tunnelSession)
}

func (s *Server) releaseConnection(role, sessionID string, conn *websocket.Conn) *websocket.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()

	tunnelSession := s.sessions[sessionID]
	if tunnelSession == nil {
		return nil
	}

	var peer *websocket.Conn
	if role == "host" && tunnelSession.host == conn {
		peer = tunnelSession.client
	} else if role == "client" && tunnelSession.client == conn {
		peer = tunnelSession.host
	} else {
		return nil
	}

	delete(s.sessions, sessionID)
	return peer
}

func (s *Server) deleteEmptySession(sessionID string, tunnelSession *session) {
	if tunnelSession.host == nil && tunnelSession.client == nil && !tunnelSession.hostReserved && !tunnelSession.clientReserved {
		delete(s.sessions, sessionID)
	}
}
