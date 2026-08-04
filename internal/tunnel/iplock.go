package tunnel

import (
	"fmt"
	"net"
	"sync"
)

// IPLocker enforces single-client IP binding after host approval.
type IPLocker struct {
	mu       sync.Mutex
	lockedIP net.IP
	accepted bool
}

// NewIPLocker returns a new IPLocker.
func NewIPLocker() *IPLocker {
	return &IPLocker{}
}

// Accept locks the session to the given IP if not already locked.
// Returns an error if the session is already locked to a different IP.
func (l *IPLocker) Accept(ip net.IP) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.accepted {
		if !l.lockedIP.Equal(ip) {
			return fmt.Errorf("IP %s does not match locked IP %s", ip, l.lockedIP)
		}
		return nil
	}

	l.lockedIP = ip
	l.accepted = true
	return nil
}

// Allow checks if the given IP is permitted to connect.
// Returns true if the session is not yet locked, or if the IP matches the locked IP.
// Returns false if the session is locked to a different IP.
func (l *IPLocker) Allow(ip net.IP) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.accepted {
		return true
	}
	return l.lockedIP.Equal(ip)
}

// Locked returns the currently locked IP, or nil if no IP has been accepted yet.
func (l *IPLocker) Locked() net.IP {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.accepted {
		return nil
	}
	return l.lockedIP
}
