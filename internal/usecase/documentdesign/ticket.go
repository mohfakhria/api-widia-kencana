package documentdesign

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
)

const (
	// TicketTTL sengaja pendek. Tiket hanya perlu bertahan dari saat frontend
	// menerimanya sampai handshake WebSocket dibuka, yang berlangsung sekejap.
	TicketTTL = 30 * time.Second

	// maxTicketsPerUser membatasi jumlah tiket hidup milik satu user.
	//
	// Secara sah hanya perlu satu tiket per percobaan koneksi; lima sudah
	// menutup beberapa tab sekaligus percobaan ulang. Tanpa batas ini, satu user
	// yang memanggil endpoint tiket berulang-ulang dapat menumbuhkan peta tiket
	// tanpa henti.
	maxTicketsPerUser = 5
)

// Ticket adalah hasil penukaran, berisi siapa yang terhubung dan ke dokumen mana.
// Ticket adalah izin sekali pakai untuk satu handshake.
//
// UserName ikut dibawa supaya jalur WebSocket tidak perlu membaca tabel user
// sama sekali. Nama hanya dipakai untuk menampilkan siapa yang sedang membuka
// dokumen, dan tiket berumur tiga puluh detik — cukup pendek untuk memastikan
// namanya tidak basi, cukup jauh dari orchestrator untuk memastikan query-nya
// tidak pernah menahan penyuntingan siapa pun.
type Ticket struct {
	UserID        string
	UserName      string
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
//
// byUser adalah index balik dari user ke tiket miliknya, dipakai untuk menegakkan
// maxTicketsPerUser tanpa menyapu seluruh peta tiket setiap kali menerbitkan.
type ticketStore struct {
	mu      sync.Mutex
	tickets map[string]ticketEntry
	byUser  map[string]map[string]struct{}
}

func newTicketStore() *ticketStore {
	return &ticketStore{
		tickets: make(map[string]ticketEntry),
		byUser:  make(map[string]map[string]struct{}),
	}
}

func (s *ticketStore) issue(ticket Ticket, now time.Time) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate design ticket: %w", err)
	}
	key := base64.RawURLEncoding.EncodeToString(raw)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.enforceUserQuotaLocked(ticket.UserID, now)

	s.tickets[key] = ticketEntry{ticket: ticket, expiresAt: now.Add(TicketTTL)}
	if s.byUser[ticket.UserID] == nil {
		s.byUser[ticket.UserID] = make(map[string]struct{})
	}
	s.byUser[ticket.UserID][key] = struct{}{}

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
	s.removeLocked(key)

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
			s.removeLocked(key)
		}
	}
}

// enforceUserQuotaLocked menyisihkan tiket kedaluwarsa milik satu user, lalu
// membuang yang tertua sampai kuotanya menyisakan tempat.
//
// Yang tertua dibuang, bukan permintaan barunya yang ditolak, supaya user yang
// sekadar membuka banyak tab tidak terkunci dari dokumennya sendiri. Tiket lama
// yang belum sempat dipakai memang paling kecil kemungkinannya masih dinanti.
func (s *ticketStore) enforceUserQuotaLocked(userID string, now time.Time) {
	for key := range s.byUser[userID] {
		entry, ok := s.tickets[key]
		if !ok {
			// Index dan penyimpanan tidak sinkron; rapikan indexnya saja.
			delete(s.byUser[userID], key)
			continue
		}
		if now.After(entry.expiresAt) {
			s.removeLocked(key)
		}
	}

	for len(s.byUser[userID]) >= maxTicketsPerUser {
		oldest, ok := s.oldestLocked(userID)
		if !ok {
			break
		}
		s.removeLocked(oldest)
	}
}

// oldestLocked mencari tiket paling tua milik satu user. Karena seluruh tiket
// memakai TTL yang sama, kedaluwarsa paling awal berarti diterbitkan paling awal.
// Penyapuan linear tidak jadi soal: panjangnya dibatasi maxTicketsPerUser.
func (s *ticketStore) oldestLocked(userID string) (string, bool) {
	var (
		oldestKey string
		oldestAt  time.Time
		found     bool
	)

	for key := range s.byUser[userID] {
		entry, ok := s.tickets[key]
		if !ok {
			continue
		}
		if !found || entry.expiresAt.Before(oldestAt) {
			oldestKey, oldestAt, found = key, entry.expiresAt, true
		}
	}

	return oldestKey, found
}

func (s *ticketStore) removeLocked(key string) {
	entry, ok := s.tickets[key]
	if !ok {
		return
	}
	delete(s.tickets, key)

	tickets := s.byUser[entry.ticket.UserID]
	delete(tickets, key)
	if len(tickets) == 0 {
		delete(s.byUser, entry.ticket.UserID)
	}
}
