package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"meowgram/server/internal/model"
)

const wsTicketLifetime = 30 * time.Second

// WSTicketStore issues short-lived, one-use credentials for browser WebSocket
// handshakes. Only a hash is retained in memory.
type WSTicketStore struct {
	mu      sync.Mutex
	tickets map[[32]byte]wsTicket
}

type wsTicket struct {
	user      *model.User
	expiresAt time.Time
}

func NewWSTicketStore() *WSTicketStore {
	return &WSTicketStore{tickets: make(map[[32]byte]wsTicket)}
}

func (s *WSTicketStore) Issue(user *model.User) (string, error) {
	if user == nil {
		return "", errors.New("cannot issue a WebSocket ticket without a user")
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	digest := sha256.Sum256([]byte(token))
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()
	for key, ticket := range s.tickets {
		if !ticket.expiresAt.After(now) {
			delete(s.tickets, key)
		}
	}
	if len(s.tickets) >= 10000 {
		return "", errors.New("WebSocket ticket capacity reached")
	}
	userCopy := *user
	s.tickets[digest] = wsTicket{user: &userCopy, expiresAt: now.Add(wsTicketLifetime)}
	return token, nil
}

// Consume atomically invalidates a ticket whether it is valid or expired.
func (s *WSTicketStore) Consume(token string) (*model.User, bool) {
	if token == "" {
		return nil, false
	}
	digest := sha256.Sum256([]byte(token))
	s.mu.Lock()
	defer s.mu.Unlock()
	ticket, ok := s.tickets[digest]
	delete(s.tickets, digest)
	if !ok || !ticket.expiresAt.After(time.Now()) {
		return nil, false
	}
	userCopy := *ticket.user
	return &userCopy, true
}
