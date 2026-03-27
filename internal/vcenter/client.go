package vcenter

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vim25/soap"
)

// Session holds a single govmomi client connection to vCenter.
// It is safe for concurrent use.
type Session struct {
	mu     sync.RWMutex
	client *govmomi.Client
	host   string
	params ConnectParams // stored for keep-alive reconnects
}

// ConnectParams holds the parameters needed to establish a vCenter connection.
type ConnectParams struct {
	URL      string
	Username string
	Password string
	Insecure bool // skip TLS verification — useful for self-signed certs
}

// Connect establishes a new vCenter session, replacing any existing one.
func (s *Session) Connect(ctx context.Context, p ConnectParams) error {
	u, err := soap.ParseURL(p.URL)
	if err != nil {
		return fmt.Errorf("invalid vCenter URL: %w", err)
	}
	u.User = url.UserPassword(p.Username, p.Password)

	c, err := govmomi.NewClient(ctx, u, p.Insecure)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		_ = s.client.Logout(ctx)
	}

	s.client = c
	s.host = u.Host
	s.params = p
	return nil
}

// Disconnect logs out of the current vCenter session.
func (s *Session) Disconnect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil {
		return nil
	}
	err := s.client.Logout(ctx)
	s.client = nil
	s.host = ""
	return err
}

// Client returns the underlying govmomi client.
// Returns an error if not connected.
func (s *Session) Client() (*govmomi.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.client == nil {
		return nil, fmt.Errorf("not connected to vCenter")
	}
	return s.client, nil
}

// Host returns the vCenter host of the current session, or empty string if not connected.
func (s *Session) Host() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.host
}

// StartKeepAlive starts a background goroutine that pings vCenter every 10 minutes
// and reconnects if the session has expired. ctx should be the Wails app context
// so the goroutine exits cleanly on shutdown.
func (s *Session) StartKeepAlive(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.mu.RLock()
				client := s.client
				params := s.params
				s.mu.RUnlock()

				if client == nil {
					return
				}

				smgr := session.NewManager(client.Client)
				if _, err := smgr.UserSession(ctx); err != nil {
					// Session expired — attempt silent reconnect.
					_ = s.Connect(ctx, params)
				}
			}
		}
	}()
}
