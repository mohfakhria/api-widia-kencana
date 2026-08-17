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
	tickets     *ticketStore
	rooms       *manager
	connections *connectionCounter
}

// NewService menerima context aplikasi karena orchestrator room hidup lebih lama
// daripada koneksi mana pun dan harus berhenti bersama aplikasi.
func NewService(
	appCtx context.Context,
	documents output.DocumentRepository,
	encoder MessageEncoder,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		documents:   documents,
		tickets:     newTicketStore(),
		rooms:       newManager(appCtx, documents, encoder, logger),
		connections: newConnectionCounter(),
	}
}

// IssueTicket memastikan dokumennya ada sebelum menerbitkan tiket handshake.
//
// Belum ada pemeriksaan kepemilikan karena dokumen memang belum punya pemilik,
// sama seperti endpoint document lain di aplikasi ini.
// IssueTicket menerima nama dari pemanggil, tidak mencarinya sendiri.
//
// Nama sudah ada di sesi — disalin ke sana saat login dan disegarkan tiap
// refresh — sehingga mencarinya lagi di sini berarti menembak database dua kali
// untuk baris yang sama dalam satu permintaan.
//
// Satu pemeriksaan memang hilang bersamanya, dan itu perlu disebut: dulu
// FindByID di sini menolak user yang sudah dihapus. Penolakan itu sebenarnya
// SATU-SATUNYA di seluruh jalur terautentikasi — AuthRequired tidak pernah
// menyentuh database, jadi user yang dihapus tetap dapat memakai setiap rute
// lain sampai sesinya berakhir. Yang berlaku sekarang seragam: sesi menjadi
// wewenang sampai 24 jam, dan database menegaskan dirinya kembali saat refresh,
// tempat user yang lenyap ditolak memperpanjang.
func (s *Service) IssueTicket(ctx context.Context, documentToken string, userID int64, userName string) (string, time.Duration, error) {
	if userID == 0 {
		return "", 0, domain.NewError(domain.ErrUnauthorized, "Invalid or expired token")
	}
	if err := s.ensureDocument(ctx, documentToken); err != nil {
		return "", 0, err
	}

	return s.issueTicket(documentToken, userID, userName)
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

func (s *Service) issueTicket(documentToken string, userID int64, userName string) (string, time.Duration, error) {
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
func (s *Service) Attach(documentToken string, userID int64) (int, error) {
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
func (s *Service) MoveCursor(documentToken string, userID int64, page string, x, y float64) {
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
// SelectElements mencatat elemen yang sedang dipilih satu orang, lalu
// menyiarkannya ke penghuni lain sebagai sorotan.
//
// Kehadiran, bukan isi dokumen — dan itu keputusan pokoknya. Ia tidak menaikkan
// version, tidak masuk riwayat undo, dan tidak pernah tersimpan. Bila dibangun
// sebagai penyuntingan, tiap klik akan membuat klien lain melihat nomor version
// melompat lalu meminta dokumen ulang, dan Ctrl+Z akan membatalkan "orang lain
// mengklik sesuatu".
//
// Tidak mengembalikan apa pun. Daftar yang kelewat panjang dipotong, bukan
// ditolak: penolakan tiba di klien sebagai malformed_message yang memicu
// permintaan dokumen ulang, dan itu tidak sebanding dengan satu seret marquee.
func (s *Service) SelectElements(documentToken string, userID int64, elementIDs []string) {
	s.rooms.selectElements(documentToken, Selection{
		UserID:     userID,
		ElementIDs: elementIDs,
	})
}

func (s *Service) CreateElement(ctx context.Context, documentToken string, sub Subscriber, origin Origin, page string, element design.Element) error {
	return s.rooms.createElement(ctx, documentToken, sub, origin, page, element)
}

// UpdateElement mengganti satu elemen seluruhnya.
//
// Tidak mengembalikan apa pun. Elemen yang sudah lenyap didiamkan: pada
// menang-terakhir itu lomba yang wajar, dan pengirimnya toh sedang menerima
// siaran penghapusannya.
func (s *Service) UpdateElement(documentToken string, sub Subscriber, origin Origin, element design.Element) {
	s.rooms.updateElement(documentToken, sub, origin, element)
}

func (s *Service) DeleteElement(documentToken string, sub Subscriber, origin Origin, id string) {
	s.rooms.deleteElement(documentToken, sub, origin, id)
}

// ReorderElement memindahkan elemen di dalam halamannya, karena urutan elemen
// adalah urutan gambar. Index di luar batas dijepit oleh orchestrator, dan letak
// sesungguhnya itulah yang disiarkan.
func (s *Service) ReorderElement(documentToken string, sub Subscriber, origin Origin, id string, index int) {
	s.rooms.reorderElement(documentToken, sub, origin, id, index)
}

// CreatePage menyisipkan halaman kosong. index kosong berarti di akhir.
//
// Mengembalikan error karena dapat ditolak: id sudah dipakai, atau dokumen sudah
// menyentuh batas jumlah halaman.
func (s *Service) CreatePage(ctx context.Context, documentToken string, sub Subscriber, origin Origin, id string, index *int) error {
	return s.rooms.createPage(ctx, documentToken, sub, origin, id, index)
}

// DeletePage membuang halaman beserta seluruh elemennya.
//
// Mengembalikan error, berbeda dari DeleteElement, karena halaman TERAKHIR tidak
// boleh dibuang: dokumen tanpa halaman akan ditimpa panduan bawaan saat dimuat
// berikutnya, dan itu terjadi bukan pada saat penghapusannya melainkan ketika
// orang berikutnya membukanya.
func (s *Service) DeletePage(ctx context.Context, documentToken string, sub Subscriber, origin Origin, id string) error {
	return s.rooms.deletePage(ctx, documentToken, sub, origin, id)
}

// UpdatePage menyetel properti satu halaman.
//
// Dari keempatnya hanya hidden dan background yang mengubah hasil cetak: ekspor
// melewati halaman tersembunyi seluruhnya termasuk tidak mengunduh asetnya, dan
// background diisi sebelum elemen mana pun digambar. title dan locked keterangan
// bagi editor — tidak pernah digambar, dan locked tidak ditegakkan backend di
// mana pun.
//
// Tidak mengembalikan apa pun. Halaman yang sudah lenyap, dan nilai yang memang
// sudah sama, sama-sama didiamkan tanpa menaikkan version.
func (s *Service) UpdatePage(documentToken string, sub Subscriber, origin Origin, id string, props design.PageProps) {
	s.rooms.updatePage(documentToken, sub, origin, id, props)
}

// ReorderPage memindahkan halaman. Index di luar batas dijepit orchestrator, dan
// letak sesungguhnya itulah yang disiarkan.
func (s *Service) ReorderPage(documentToken string, sub Subscriber, origin Origin, id string, index int) {
	s.rooms.reorderPage(documentToken, sub, origin, id, index)
}

// Undo memundurkan SELURUH DOKUMEN satu langkah, bukan langkah pemanggilnya.
//
// Riwayatnya hidup di memori room dan mati bersamanya — room dibuang sepuluh
// detik setelah penghuni terakhir pergi, dan riwayatnya ikut. Ini bukan riwayat
// versi dokumen; ia tidak pernah menyentuh database.
//
// Tidak mengembalikan apa pun. Tumpukan yang kosong bukan kegagalan melainkan
// keadaan biasa: tidak ada lagi yang bisa dibatalkan.
func (s *Service) Undo(documentToken string, sub Subscriber) {
	s.rooms.undo(documentToken, sub)
}

func (s *Service) Redo(documentToken string, sub Subscriber) {
	s.rooms.redo(documentToken, sub)
}

func (s *Service) Detach(documentToken string, userID int64, sub Subscriber) {
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

// CreateGuide memasang garis bantu baru dan melaporkan penolakannya.
//
// Satu-satunya jalur guide yang mengembalikan error, sejalan dengan
// CreateElement: id yang sudah dipakai dan batas jumlah adalah kesalahan
// pemanggil yang wajib ia ketahui.
func (s *Service) CreateGuide(ctx context.Context, documentToken string, sub Subscriber, origin Origin, guide design.Guide) error {
	return s.rooms.createGuide(ctx, documentToken, sub, origin, guide)
}

// UpdateGuide mengganti guide seluruhnya. Guide yang sudah lenyap didiamkan.
func (s *Service) UpdateGuide(documentToken string, sub Subscriber, origin Origin, guide design.Guide) {
	s.rooms.updateGuide(documentToken, sub, origin, guide)
}

func (s *Service) DeleteGuide(documentToken string, sub Subscriber, origin Origin, id string) {
	s.rooms.deleteGuide(documentToken, sub, origin, id)
}
