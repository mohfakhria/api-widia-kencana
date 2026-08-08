package documentdesign

import (
	"context"
	"log/slog"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
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

// Identitas Widia Agent di dalam ruang penyuntingan.
//
// Berbentuk angka seperti id pengguna sungguhan, supaya frontend tidak perlu
// memperlakukan agent sebagai kekecualian di mana pun ia mengurai atau
// membandingkan id.
//
// Nilainya dipilih jauh di atas jangkauan pemakaian nyata agar dikenali sekilas
// di log maupun di presence, dan barisnya dicadangkan di migration/users.sql
// supaya urutan identitas tidak pernah memberikannya kepada orang.
//
// Baris itu penanda, BUKAN sumber kebenaran. Agent masuk dengan kunci dari
// environment dan tidak pernah membacanya — nama di bawah ini satu-satunya
// sumber sebutannya di presence. Sengaja begitu: menjadikannya query berarti
// agent mati total di setiap database yang barisnya belum tersisip.
//
// Kuota koneksi berlaku pada id ini seperti pada user lain: seluruh agent yang
// berjalan bersamaan berbagi jatah maxConnectionsPerUser.
const (
	AgentUserID   = "99999"
	AgentUserName = "Widia-Agent"
)

// IssueTicket memastikan dokumennya ada sebelum menerbitkan tiket handshake.
//
// Belum ada pemeriksaan kepemilikan karena dokumen memang belum punya pemilik,
// sama seperti endpoint document lain di aplikasi ini.
func (s *Service) IssueTicket(ctx context.Context, documentToken, userID string) (string, time.Duration, error) {
	if userID == "" {
		return "", 0, domain.NewError(domain.ErrUnauthorized, "Invalid or expired token")
	}
	if err := s.ensureDocument(ctx, documentToken); err != nil {
		return "", 0, err
	}

	// Nama diambil di sini, bukan saat koneksi terbuka. Penerbitan tiket terjadi
	// sekali per sesi dan sudah menyentuh database, sedangkan jalur WebSocket
	// harus tetap bebas dari query.
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return "", 0, domain.NewError(domain.ErrUnauthorized, "User not found or deleted")
	}

	return s.issueTicket(documentToken, userID, user.Name)
}

// IssueAgentTicket menerbitkan tiket untuk Widia Agent.
//
// Jalur tersendiri, bukan cabang di dalam IssueTicket, karena yang membedakannya
// bukan cara memeriksa melainkan SIAPA yang memeriksa: pemanggilnya sudah lolos
// AgentRequired, bukan AuthRequired. Menyatukannya berarti satu jalur yang harus
// mengingat dua model autentikasi, dan setiap penyunting berikutnya harus
// memutuskan cabang mana yang sedang ia sentuh.
//
// Tidak ada pencarian ke tabel user: agent tidak punya baris di sana.
func (s *Service) IssueAgentTicket(ctx context.Context, documentToken string) (string, time.Duration, error) {
	if err := s.ensureDocument(ctx, documentToken); err != nil {
		return "", 0, err
	}

	return s.issueTicket(documentToken, AgentUserID, AgentUserName)
}

// ensureDocument menolak token yang cacat dan dokumen yang tidak ada, sebelum
// tiket apa pun dibuat.
func (s *Service) ensureDocument(ctx context.Context, documentToken string) error {
	if _, err := uuid.Parse(documentToken); err != nil {
		return domain.NewError(domain.ErrInvalidInput, "invalid document token")
	}
	if _, err := s.documents.GetByToken(ctx, documentToken); err != nil {
		return err
	}

	return nil
}

func (s *Service) issueTicket(documentToken, userID, userName string) (string, time.Duration, error) {
	key, err := s.tickets.issue(Ticket{
		UserID:        userID,
		UserName:      userName,
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

// MoveCursor menyiarkan letak kursor seseorang ke seluruh penghuni dokumen.
//
// Tidak mengembalikan apa pun dan tidak menunggu: kursor adalah keadaan sesaat,
// dan memberinya nilai balik hanya mengundang pemanggil menanganinya seolah ia
// berarti.
func (s *Service) MoveCursor(documentToken, userID, page string, x, y float64) {
	s.rooms.moveCursor(documentToken, Cursor{
		UserID: userID,
		Page:   page,
		X:      x,
		Y:      y,
	})
}

// CreateElement memasang elemen baru pada satu halaman.
//
// Satu-satunya penyuntingan yang mengembalikan error, karena satu-satunya yang
// dapat ditolak: halaman tidak ada, atau id elemennya sudah dipakai. Pemanggil
// wajib meneruskannya ke klien — elemen yang sudah tergambar optimistis di
// layarnya tidak akan pernah ada di dokumen bila ia tidak diberi tahu.
func (s *Service) CreateElement(ctx context.Context, documentToken string, sub Subscriber, page string, element design.Element) error {
	return s.rooms.createElement(ctx, documentToken, sub, page, element)
}

// UpdateElement mengganti satu elemen seluruhnya.
//
// Tidak mengembalikan apa pun. Elemen yang sudah lenyap didiamkan: pada
// menang-terakhir itu lomba yang wajar, dan pengirimnya toh sedang menerima
// siaran penghapusannya.
func (s *Service) UpdateElement(documentToken string, sub Subscriber, element design.Element) {
	s.rooms.updateElement(documentToken, sub, element)
}

func (s *Service) DeleteElement(documentToken string, sub Subscriber, id string) {
	s.rooms.deleteElement(documentToken, sub, id)
}

// ReorderElement memindahkan elemen di dalam halamannya, karena urutan elemen
// adalah urutan gambar. Index di luar batas dijepit oleh orchestrator, dan letak
// sesungguhnya itulah yang disiarkan.
func (s *Service) ReorderElement(documentToken string, sub Subscriber, id string, index int) {
	s.rooms.reorderElement(documentToken, sub, id, index)
}

// CreatePage menyisipkan halaman kosong. index kosong berarti di akhir.
//
// Mengembalikan error karena dapat ditolak: id sudah dipakai, atau dokumen sudah
// menyentuh batas jumlah halaman.
func (s *Service) CreatePage(ctx context.Context, documentToken string, sub Subscriber, id string, index *int) error {
	return s.rooms.createPage(ctx, documentToken, sub, id, index)
}

// DeletePage membuang halaman beserta seluruh elemennya.
//
// Mengembalikan error, berbeda dari DeleteElement, karena halaman TERAKHIR tidak
// boleh dibuang: dokumen tanpa halaman akan ditimpa panduan bawaan saat dimuat
// berikutnya, dan itu terjadi bukan pada saat penghapusannya melainkan ketika
// orang berikutnya membukanya.
func (s *Service) DeletePage(ctx context.Context, documentToken string, sub Subscriber, id string) error {
	return s.rooms.deletePage(ctx, documentToken, sub, id)
}

// UpdatePage menyetel title, hidden, dan locked pada satu halaman.
//
// Dari ketiganya hanya hidden yang mengubah hasil cetak: ekspor melewati halaman
// tersembunyi seluruhnya, termasuk tidak mengunduh asetnya. title dan locked
// keterangan bagi editor — tidak pernah digambar, dan locked tidak ditegakkan
// backend di mana pun.
//
// Tidak mengembalikan apa pun. Halaman yang sudah lenyap, dan nilai yang memang
// sudah sama, sama-sama didiamkan tanpa menaikkan version.
func (s *Service) UpdatePage(documentToken string, sub Subscriber, id, title string, hidden, locked bool) {
	s.rooms.updatePage(documentToken, sub, id, title, hidden, locked)
}

// ReorderPage memindahkan halaman. Index di luar batas dijepit orchestrator, dan
// letak sesungguhnya itulah yang disiarkan.
func (s *Service) ReorderPage(documentToken string, sub Subscriber, id string, index int) {
	s.rooms.reorderPage(documentToken, sub, id, index)
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
