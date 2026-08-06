package documentdesign

import (
	"context"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/google/uuid"
)

// Service adalah satu-satunya pintu masuk lapisan delivery ke sesi penyuntingan
// realtime. Ia tidak mengenal WebSocket, HTTP, maupun gin.
type Service struct {
	documents output.DocumentRepository
	tickets   *ticketStore
	rooms     *manager
}

func NewService(documents output.DocumentRepository) *Service {
	return &Service{
		documents: documents,
		tickets:   newTicketStore(),
		rooms:     newManager(),
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

func (s *Service) Join(documentToken string, sub Subscriber) *Room {
	return s.rooms.join(documentToken, sub)
}

func (s *Service) Leave(documentToken string, sub Subscriber) {
	s.rooms.leave(documentToken, sub)
}

// Name dan Run membuat Service dapat dijalankan sebagai komponen bootstrap,
// sehingga goroutine pembersih tiket punya pemilik dan berhenti bersama aplikasi.
func (s *Service) Name() string {
	return "document-design-janitor"
}

func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(ticketCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case now := <-ticker.C:
			s.tickets.evictExpired(now)
		}
	}
}
