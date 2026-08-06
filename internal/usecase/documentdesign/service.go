package documentdesign

import (
	"context"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/google/uuid"
)

// janitorInterval adalah denyut pembersih. Satu goroutine melayani dua sapuan
// sekaligus — tiket kedaluwarsa dan room yang sudah kosong — karena keduanya
// hanya menyusuri peta di memori dan tidak sepadan dengan goroutine sendiri.
//
// Konsekuensinya keduanya berbagi irama, sehingga denyutnya harus mengikuti umur
// terpendek yang dijaga, yaitu roomIdleGrace. Bila denyut dan umurnya sama,
// sampah bisa bertahan hampir dua kali lipat sebelum tersapu.
const janitorInterval = 5 * time.Second

// roomStopTimeout membatasi berapa lama shutdown menunggu seluruh orchestrator
// berhenti. Disamakan dengan tenggat shutdown HTTP server supaya keduanya tidak
// saling menunggu lebih lama dari yang diperlukan.
const roomStopTimeout = 5 * time.Second

// Service adalah satu-satunya pintu masuk lapisan delivery ke sesi penyuntingan
// realtime. Ia tidak mengenal WebSocket, HTTP, maupun gin.
type Service struct {
	documents   output.DocumentRepository
	tickets     *ticketStore
	rooms       *manager
	connections *connectionCounter
}

func NewService(documents output.DocumentRepository) *Service {
	return &Service{
		documents:   documents,
		tickets:     newTicketStore(),
		rooms:       newManager(),
		connections: newConnectionCounter(),
	}
}

// IssueTicket memastikan dokumennya ada sebelum menerbitkan tiket handshake.
//
// Belum ada pemeriksaan kepemilikan karena dokumen memang belum punya pemilik,
// sama seperti endpoint document lain di aplikasi ini.
func (s *Service) IssueTicket(ctx context.Context, documentToken, userID string) (string, time.Duration, error) {
	if userID == "" {
		return "", 0, domain.NewError(domain.ErrUnauthorized, "Invalid or expired token")
	}
	if _, err := uuid.Parse(documentToken); err != nil {
		return "", 0, domain.NewError(domain.ErrInvalidInput, "invalid document token")
	}
	if _, err := s.documents.GetByToken(ctx, documentToken); err != nil {
		return "", 0, err
	}

	key, err := s.tickets.issue(Ticket{
		UserID:        userID,
		DocumentToken: documentToken,
	}, time.Now())
	if err != nil {
		return "", 0, err
	}

	return key, TicketTTL, nil
}

// Redeem menghanguskan tiket dan memastikan ia memang diterbitkan untuk dokumen
// yang sedang diminta, bukan dokumen lain.
func (s *Service) Redeem(key, documentToken string) (Ticket, error) {
	ticket, err := s.tickets.redeem(key, time.Now())
	if err != nil {
		return Ticket{}, err
	}
	if ticket.DocumentToken != documentToken {
		return Ticket{}, domain.NewError(domain.ErrForbidden, "design ticket does not match document")
	}

	return ticket, nil
}

// Join mendaftarkan subscriber ke room dokumen dan mengembalikan isinya saat itu.
//
// Kuota koneksi per user diambil lebih dulu, dan dikembalikan bila pendaftaran
// gagal. Room tidak ikut dikembalikan: pemanggil tidak membutuhkannya, dan
// menyembunyikan tipe itu menjaga lapisan delivery tetap buta terhadap cara room
// bekerja.
//
// Setiap Join yang berhasil wajib diimbangi tepat satu Leave dengan userID yang
// sama, kalau tidak kuotanya bocor dan user itu perlahan terkunci sendiri.
func (s *Service) Join(ctx context.Context, documentToken, userID string, sub Subscriber) (Snapshot, error) {
	if !s.connections.acquire(userID) {
		return Snapshot{}, domain.NewError(domain.ErrTooManyRequests, "too many concurrent design connections")
	}

	snapshot, err := s.rooms.join(ctx, documentToken, sub)
	if err != nil {
		s.connections.release(userID)
		return Snapshot{}, err
	}

	return snapshot, nil
}

func (s *Service) Leave(documentToken, userID string, sub Subscriber) {
	s.rooms.leave(documentToken, sub)
	s.connections.release(userID)
}

// Name dan Run membuat Service dapat dijalankan sebagai komponen bootstrap,
// sehingga pembersih berkala dan seluruh orchestrator room punya pemilik yang
// jelas dan berhenti bersama aplikasi.
func (s *Service) Name() string {
	return "document-design-janitor"
}

func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(janitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Hentikan seluruh orchestrator yang masih hidup lalu tunggu sampai
			// benar-benar berhenti. Koneksi biasanya sudah membubarkan diri lebih
			// dulu karena context-nya turunan dari context aplikasi, tetapi ini
			// menutup kemungkinan ada yang tersisa — termasuk room yang sedang
			// menunggu masa tenggangnya habis.
			s.rooms.stopAll(roomStopTimeout)
			return nil
		case now := <-ticker.C:
			s.tickets.evictExpired(now)
			s.rooms.sweepIdle(now)
		}
	}
}
