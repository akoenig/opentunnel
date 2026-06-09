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
	host   *websocket.Conn
	client *websocket.Conn
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
		if !s.canConnect(role, sessionID) {
			http.Error(w, "session endpoint unavailable", http.StatusConflict)
			return
		}

		conn, err := s.upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		if !s.register(role, sessionID, conn) {
			conn.Close()
			return
		}

		s.forward(role, sessionID, conn)
	})
}

func (s *Server) canConnect(role, sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if role == "host" {
		return session == nil || session.host == nil
	}
	return session != nil && session.host != nil && session.client == nil
}

func (s *Server) register(role, sessionID string, conn *websocket.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	tunnelSession := s.sessions[sessionID]
	if role == "host" {
		if tunnelSession != nil && tunnelSession.host != nil {
			return false
		}
		if tunnelSession == nil {
			tunnelSession = &session{}
			s.sessions[sessionID] = tunnelSession
		}
		tunnelSession.host = conn
		return true
	}

	if tunnelSession == nil || tunnelSession.host == nil || tunnelSession.client != nil {
		return false
	}
	tunnelSession.client = conn
	return true
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
	peer := s.unregister(role, sessionID, conn)
	conn.Close()
	if peer != nil {
		peer.Close()
	}
}

func (s *Server) unregister(role, sessionID string, conn *websocket.Conn) *websocket.Conn {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := s.sessions[sessionID]
	if session == nil {
		return nil
	}

	var peer *websocket.Conn
	if role == "host" && session.host == conn {
		peer = session.client
		session.host = nil
	} else if role == "client" && session.client == conn {
		peer = session.host
		session.client = nil
	}

	if session.host == nil && session.client == nil {
		delete(s.sessions, sessionID)
	}
	return peer
}
