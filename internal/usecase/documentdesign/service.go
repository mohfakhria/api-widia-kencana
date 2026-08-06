package documentdesign

import (
	"context"
	"log/slog"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
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
// berhenti.
//
// Nilainya harus menutupi kasus terburuk satu room saat drain: menunggu penulisan
// yang sedang berjalan (drainSaveWait, 4 detik) lalu menyimpan sekali lagi
// (contentSaveTimeout, 3 detik). Bila lebih kecil, stopAll menyerah di tengah
// drain dan justru membuang suntingan terakhir yang drain diadakan untuk
// menyelamatkan.
const roomStopTimeout = 8 * time.Second

// Service adalah satu-satunya pintu masuk lapisan delivery ke sesi penyuntingan
// realtime. Ia tidak mengenal WebSocket, HTTP, maupun gin.
type Service struct {
	documents   output.DocumentRepository
	users       output.UserRepository
	tickets     *ticketStore
	rooms       *manager
	connections *connectionCounter
}

// NewService menerima context aplikasi karena orchestrator room hidup lebih lama
// daripada koneksi mana pun dan harus berhenti bersama aplikasi.
func NewService(
	appCtx context.Context,
	documents output.DocumentRepository,
	users output.UserRepository,
	encoder MessageEncoder,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		documents:   documents,
		users:       users,
		tickets:     newTicketStore(),
		rooms:       newManager(appCtx, documents, encoder, logger),
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

	// Nama diambil di sini, bukan saat koneksi terbuka. Penerbitan tiket terjadi
	// sekali per sesi dan sudah menyentuh database, sedangkan jalur WebSocket
	// harus tetap bebas dari query.
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return "", 0, domain.NewError(domain.ErrUnauthorized, "User not found or deleted")
	}

	key, err := s.tickets.issue(Ticket{
		UserID:        userID,
		UserName:      user.Name,
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

// Attach mencatat satu koneksi baru pada dokumen, mengambil kuota koneksi user
// lebih dulu.
//
// Belum menjadikan koneksinya anggota room; itu terjadi saat klien meminta
// dokumen lewat Sync. Setiap Attach yang berhasil wajib diimbangi tepat satu
// Detach dengan userID yang sama, kalau tidak kuotanya bocor dan user itu
// perlahan terkunci sendiri.
// Mengembalikan jumlah koneksi yang kini dipegang user tersebut, untuk dicatat
// pemanggil.
func (s *Service) Attach(documentToken, userID string) (int, error) {
	open, ok := s.connections.acquire(userID)
	if !ok {
		return open, domain.NewError(domain.ErrTooManyRequests, "too many concurrent design connections")
	}

	s.rooms.attach(documentToken)

	return open, nil
}

// Sync meminta room mengirimkan snapshot ke koneksi ini dan menjadikannya
// anggota, sehingga mulai saat itu ia menerima siaran.
//
// Boleh dipanggil berulang pada koneksi yang sama: itulah jalur pemulihan ketika
// klien menyadari keadaannya tertinggal.
//
// Identitas dibawa di sini, bukan saat Attach, karena kehadiran dihitung dari
// keanggotaan room — dan seseorang baru dianggap hadir setelah ia benar-benar
// menerima isi dokumennya.
func (s *Service) Sync(ctx context.Context, documentToken string, member Member) error {
	return s.rooms.sync(ctx, documentToken, member)
}

// Snapshot mengembalikan isi dokumen yang paling mutakhir, untuk ekspor.
//
// Sumbernya room bila dokumen sedang disunting, dan database bila tidak. Urutan
// ini penting: penyimpanan bersifat tunda sampai dua detik, jadi membaca database
// pada dokumen yang sedang dibuka dapat menghasilkan PDF yang tertinggal beberapa
// suntingan dari yang dilihat pengguna di layar.
func (s *Service) Snapshot(ctx context.Context, documentToken string) (*entity.DocumentContent, error) {
	result, found, err := s.rooms.snapshot(ctx, documentToken)
	if err != nil {
		return nil, err
	}
	if !found {
		return s.documents.GetContent(ctx, documentToken)
	}

	return &entity.DocumentContent{
		Token:   documentToken,
		Content: result.content,
		Version: result.version,
		Paper:   result.paper,
	}, nil
}

func (s *Service) Detach(documentToken, userID string, sub Subscriber) {
	s.rooms.detach(documentToken, sub)
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
