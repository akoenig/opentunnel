package relay

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Server routes opaque websocket tunnel frames between one host and one client per session.
type Server struct {
	mu       sync.Mutex
	sessions map[string]*session
	upgrader websocket.Upgrader
}

type session struct {
	host           *websocket.Conn
	client         *websocket.Conn
	hostReserved   bool
	clientReserved bool
}

// NewServer creates an in-memory relay server.
func NewServer() *Server {
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
		tunnelSession.host = nil
		tunnelSession.hostReserved = false
	} else if role == "client" && tunnelSession.client == conn {
		peer = tunnelSession.host
		tunnelSession.client = nil
		tunnelSession.clientReserved = false
	}

	s.deleteEmptySession(sessionID, tunnelSession)
	return peer
}

func (s *Server) deleteEmptySession(sessionID string, tunnelSession *session) {
	if tunnelSession.host == nil && tunnelSession.client == nil && !tunnelSession.hostReserved && !tunnelSession.clientReserved {
		delete(s.sessions, sessionID)
	}
}
