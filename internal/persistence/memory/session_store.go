package memory

import (
	"context"
	"sync"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

const cleanupInterval = 10 * time.Minute

type sessionEntry struct {
	session   output.Session
	expiresAt time.Time
}

// SessionStore menyimpan refresh session di memory proses.
//
// Session tidak bertahan setelah restart dan tidak dibagi antar instance,
// sehingga API harus dijalankan sebagai satu replica. Kalau nanti perlu
// scale horizontal, ganti implementasi ini dengan store bersama.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]sessionEntry
	byUser   map[int64]map[string]struct{}
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]sessionEntry),
		byUser:   make(map[int64]map[string]struct{}),
	}
}

func (s *SessionStore) Set(_ context.Context, sessionID string, session output.Session, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Session yang berpindah pemilik akan meninggalkan index user lama.
	if existing, ok := s.sessions[sessionID]; ok && existing.session.UserID != session.UserID {
		s.detachLocked(existing.session.UserID, sessionID)
	}

	s.sessions[sessionID] = sessionEntry{
		session:   session,
		expiresAt: time.Now().Add(ttl),
	}
	if s.byUser[session.UserID] == nil {
		s.byUser[session.UserID] = make(map[string]struct{})
	}
	s.byUser[session.UserID][sessionID] = struct{}{}

	return nil
}

func (s *SessionStore) Get(_ context.Context, sessionID string) (*output.Session, error) {
	s.mu.RLock()
	entry, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok {
		return nil, domain.NewError(domain.ErrNotFound, "refresh session not found")
	}
	if time.Now().After(entry.expiresAt) {
		s.evictIfExpired(sessionID)
		return nil, domain.NewError(domain.ErrNotFound, "refresh session expired")
	}

	session := entry.session
	return &session, nil
}

func (s *SessionStore) Delete(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.removeLocked(sessionID)
	return nil
}

func (s *SessionStore) DeleteAll(_ context.Context, userID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID := range s.byUser[userID] {
		delete(s.sessions, sessionID)
	}
	delete(s.byUser, userID)

	return nil
}

// Name dan Run membuat store ini bisa dijalankan sebagai komponen bootstrap,
// sehingga goroutine pembersihnya punya pemilik dan berhenti bersama aplikasi.
func (s *SessionStore) Name() string {
	return "refresh-session-janitor"
}

func (s *SessionStore) Run(ctx context.Context) error {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			s.evictExpired(now)
		}
	}
}

func (s *SessionStore) evictExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, entry := range s.sessions {
		if now.After(entry.expiresAt) {
			delete(s.sessions, sessionID)
			s.detachLocked(entry.session.UserID, sessionID)
		}
	}
}

// evictIfExpired mengambil ulang entry di bawah write lock supaya session baru
// yang ditulis setelah pembacaan di Get tidak ikut terhapus.
func (s *SessionStore) evictIfExpired(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.sessions[sessionID]
	if !ok || !time.Now().After(entry.expiresAt) {
		return
	}

	delete(s.sessions, sessionID)
	s.detachLocked(entry.session.UserID, sessionID)
}

func (s *SessionStore) removeLocked(sessionID string) {
	entry, ok := s.sessions[sessionID]
	if !ok {
		return
	}

	delete(s.sessions, sessionID)
	s.detachLocked(entry.session.UserID, sessionID)
}

func (s *SessionStore) detachLocked(userID int64, sessionID string) {
	sessions, ok := s.byUser[userID]
	if !ok {
		return
	}

	delete(sessions, sessionID)
	if len(sessions) == 0 {
		delete(s.byUser, userID)
	}
}
