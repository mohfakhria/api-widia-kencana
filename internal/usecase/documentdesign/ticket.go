package documentdesign

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
)

// TicketTTL sengaja pendek. Tiket hanya perlu bertahan dari saat frontend
// menerimanya sampai handshake WebSocket dibuka, yang berlangsung sekejap.
const TicketTTL = 30 * time.Second

// Ticket adalah hasil penukaran, berisi siapa yang terhubung dan ke dokumen mana.
type Ticket struct {
	UserID        string
	DocumentToken string
}

type ticketEntry struct {
	ticket    Ticket
	expiresAt time.Time
}

// ticketStore menyimpan tiket handshake di memori proses.
//
// Tipe konkret tanpa interface: hanya ada satu implementasi yang masuk akal,
// dan tiket berumur 30 detik memang tidak pantas dipersistenkan.
type ticketStore struct {
	mu      sync.Mutex
	tickets map[string]ticketEntry
}

func newTicketStore() *ticketStore {
	return &ticketStore{tickets: make(map[string]ticketEntry)}
}

func (s *ticketStore) issue(ticket Ticket, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate design ticket: %w", err)
	}
	key := base64.RawURLEncoding.EncodeToString(raw)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.tickets[key] = ticketEntry{ticket: ticket, expiresAt: now.Add(TicketTTL)}
	return key, nil
}

// redeem menghanguskan tiket apa pun hasilnya. Tiket yang sudah kedaluwarsa
// tetap dihapus supaya tidak menumpuk di antara sapuan pembersih.
func (s *ticketStore) redeem(key string, now time.Time) (Ticket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.tickets[key]
	if !ok {
		return Ticket{}, domain.NewError(domain.ErrUnauthorized, "invalid design ticket")
	}
	delete(s.tickets, key)

	if now.After(entry.expiresAt) {
		return Ticket{}, domain.NewError(domain.ErrUnauthorized, "design ticket expired")
	}

	return entry.ticket, nil
}

func (s *ticketStore) evictExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, entry := range s.tickets {
		if now.After(entry.expiresAt) {
			delete(s.tickets, key)
		}
	}
}
