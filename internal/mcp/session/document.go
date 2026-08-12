package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/mcp/apiclient"

	"github.com/google/uuid"

	"github.com/coder/websocket"
)

const (
	// idleTimeout menutup socket yang tidak dipakai.
	//
	// Socket dibiarkan TERBUKA antar pemanggilan tool, bukan dibuka-tutup tiap
	// kali, dan itu keputusan pokok di berkas ini. Satu elemen lewat socket
	// sekali pakai menuntut tiket, sambungan, document.get, sunting, lalu
	// tutup — empat perjalanan bolak-balik untuk satu suntingan. Lebih buruk
	// lagi, presence akan BERKEDIP: agent muncul dan hilang terus-menerus,
	// sehingga menontonnya bekerja — alasan seluruh penyuntingan dilewatkan
	// socket — menjadi mustahil.
	//
	// Ongkosnya socket yang menganggur, dan lima menit adalah tebusan yang
	// wajar: cukup panjang untuk menahan jeda antar perintah dalam satu
	// percakapan, cukup pendek untuk tidak menumpuk koneksi seharian.
	idleTimeout = 5 * time.Minute

	// sweepInterval adalah denyut pemeriksa yang menganggur.
	sweepInterval = 30 * time.Second

	// readLimit menyamai batas pesan di sisi server, 1 MB. Snapshot
	// dokumen besar mendekatinya, dan batas yang lebih kecil di sini akan
	// memutus koneksi tepat pada dokumen yang paling perlu dibaca.
	readLimit = 1 << 20
)

// Document adalah satu koneksi hidup ke satu dokumen.
//
// Ia memegang isi terkini beserta nomor versinya, disegarkan oleh loop baca yang
// berjalan selama koneksinya hidup. Pembaca dari luar tidak pernah menyentuh
// socket — mereka membaca salinan di bawah mutex, sehingga pemanggilan tool
// tidak pernah menunggu jaringan.
type Document struct {
	documentToken string
	logger        *slog.Logger

	conn   *websocket.Conn
	cancel context.CancelFunc
	done   chan struct{}

	// origin adalah token buram yang menandai suntingan dari sesi ini.
	//
	// Kontrak menuntut kedelapan pesan sunting membawanya, dan satuannya PER
	// EDITOR. Satu sesi dokumen di sini setara satu editor terbuka, jadi
	// tokennya lahir bersama sesi dan mati bersamanya.
	//
	// MCP sendiri tidak memakainya untuk apa pun — ia tidak menerapkan gema.
	// Ia dikirim karena penerima LAIN membutuhkannya: tanpa penanda, frontend
	// tidak dapat membedakan suntingan agent dari gemanya sendiri.
	origin string

	mu      sync.Mutex
	version int64
	content json.RawMessage
	page    PageSize
	// snapshots menghitung snapshot yang sudah tiba.
	//
	// Yang ditunggu Refresh adalah snapshot BARU, dan isi yang lama tidak dapat
	// membedakan "sudah datang yang baru" dari "yang lama masih di sana" —
	// dokumen yang tidak berubah menghasilkan isi yang persis sama. Pencacah ini
	// yang membedakannya.
	snapshots int64
	lastUsed  time.Time
	// recentErrors menampung galat tingkat pesan yang datang dari server.
	//
	// Suntingan dikirim tanpa menunggu balasan — kontraknya memang begitu, hanya
	// element.create yang membalas saat ditolak. Tanpa penampung ini, penolakan
	// hanya muncul di log dan pemanggil tool tidak pernah tahu suntingannya
	// tidak mendarat.
	recentErrors []string
	// fatal menyimpan alasan koneksi berakhir. Dibaca pemanggil berikutnya,
	// supaya sesi yang mati menjawab "kenapa" alih-alih diam.
	fatal error
}

// PageSize adalah ukuran kertas yang ikut datang bersama snapshot.
type PageSize struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// State adalah potret yang dikembalikan ke pemanggil tool.
//
// Content bertipe any, BUKAN json.RawMessage. Keduanya membawa isi yang sama,
// tetapi RawMessage adalah []byte — dan SDK menyimpulkan skema keluaran dari
// tipe Go-nya, sehingga ia mengumumkan "array" untuk sesuatu yang sebenarnya
// objek, lalu menolak jawabannya sendiri saat validasi.
//
// any juga lebih jujur di sini: bentuk isi dokumen dimiliki domain, dan MCP
// tidak semestinya menyatakan ulang bentuk itu — pernyataan ulang yang melenceng
// justru persoalan yang berkas kontrak bersama diadakan untuk menghapus.
type State struct {
	DocumentToken string   `json:"document_token"`
	Version       int64    `json:"version"`
	Page          PageSize `json:"page"`
	Content       any      `json:"content"`
}

// openDocument menerbitkan tiket lalu menyambung, dalam satu tarikan.
//
// Keduanya TIDAK boleh dipisah oleh apa pun yang lama. Tiket hidup tiga puluh
// detik dan hangus sekali pakai; menerbitkannya di awal tugas lalu menunggu
// model menyusun perintah akan gagal dengan close code 1008 yang tidak menyebut
// bahwa penyebabnya tenggat.
func open(ctx context.Context, api *apiclient.Client, documentToken string, logger *slog.Logger) (*Document, error) {
	ticket, err := api.IssueDesignTicket(ctx, documentToken)
	if err != nil {
		return nil, fmt.Errorf("terbitkan tiket: %w", err)
	}

	// Context koneksi TIDAK diwarisi dari permintaan tool. Socket ini hidup
	// lebih lama daripada pemanggilan yang membukanya; mewarisi context
	// permintaan akan menutupnya begitu tool itu selesai menjawab.
	sessionCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))

	conn, _, err := websocket.Dial(ctx, api.SocketURL(documentToken, ticket.Ticket), nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("buka socket: %w", err)
	}
	conn.SetReadLimit(readLimit)

	s := &Document{
		documentToken: documentToken,
		origin:        "mcp-" + uuid.NewString(),
		logger:        logger,
		conn:          conn,
		cancel:        cancel,
		done:          make(chan struct{}),
		lastUsed:      time.Now(),
	}

	go s.readLoop(sessionCtx)

	// document.get memicu snapshot. Klien menjadi ANGGOTA room hanya pada
	// langkah ini — sebelum itu socketnya terbuka tetapi ia belum menerima
	// siaran apa pun, dan suntingan darinya akan ditolak.
	if err := s.send(sessionCtx, map[string]string{"type": "document.get"}); err != nil {
		s.Close()
		return nil, fmt.Errorf("minta snapshot: %w", err)
	}

	if err := s.waitForSnapshot(sessionCtx, 0); err != nil {
		s.Close()
		return nil, err
	}

	return s, nil
}

// Refresh meminta snapshot baru lalu menunggunya tiba.
//
// Dibutuhkan karena siaran perubahan sengaja TIDAK diterapkan ke isi yang
// tersimpan — lihat catatan di apply. Tanpa penyegaran, pembacaan sesudah
// penyuntingan akan mengembalikan dokumen seperti saat sesi dibuka, dan
// suntingan yang baru saja berhasil justru tidak terlihat oleh yang membuatnya.
func (s *Document) Refresh(ctx context.Context) error {
	s.mu.Lock()
	s.lastUsed = time.Now()
	awal, gagal := s.snapshots, s.fatal
	s.mu.Unlock()

	if gagal != nil {
		return gagal
	}

	if err := s.send(ctx, map[string]string{"type": "document.get"}); err != nil {
		return fmt.Errorf("minta snapshot: %w", err)
	}

	return s.waitForSnapshot(ctx, awal)
}

// waitForSnapshot menunggu snapshot yang lebih baru daripada setelah.
//
// Dipakai dua kali dengan maksud yang sama: saat sesi dibuka, isi pertama harus
// sudah ada sebelum tool mana pun membacanya — kalau tidak, dokumen yang belum
// sempat datang tidak dapat dibedakan dari dokumen yang memang kosong.
func (s *Document) waitForSnapshot(ctx context.Context, setelah int64) error {
	tenggat := time.NewTimer(10 * time.Second)
	defer tenggat.Stop()

	for {
		s.mu.Lock()
		ada, gagal := s.snapshots > setelah, s.fatal
		s.mu.Unlock()

		if ada {
			return nil
		}
		if gagal != nil {
			return gagal
		}

		select {
		case <-time.After(25 * time.Millisecond):
		case <-tenggat.C:
			return fmt.Errorf("snapshot tidak datang dalam 10 detik")
		case <-ctx.Done():
			return ctx.Err()
		case <-s.done:
			s.mu.Lock()
			defer s.mu.Unlock()
			if s.fatal != nil {
				return s.fatal
			}

			return fmt.Errorf("koneksi ditutup sebelum snapshot tiba")
		}
	}
}

// readLoop menguras socket sampai koneksinya berakhir.
//
// WAJIB berjalan terus, bahkan ketika tidak ada yang membaca hasilnya: pustaka
// WebSocket membalas ping dari server hanya selama ada yang membaca, dan server
// memutus koneksi yang tidak membalas dalam sepuluh detik. Loop yang berhenti
// karena "tidak ada yang butuh" akan membunuh sesinya sendiri.
func (s *Document) readLoop(ctx context.Context) {
	defer close(s.done)

	for {
		_, payload, err := s.conn.Read(ctx)
		if err != nil {
			s.mu.Lock()
			if s.fatal == nil {
				s.fatal = fmt.Errorf("koneksi dokumen berakhir: %w", err)
			}
			s.mu.Unlock()

			return
		}

		s.apply(payload)
	}
}

// apply menyerap satu pesan dari server.
//
// Hanya snapshot yang menggantikan isi. Siaran perubahan cukup menaikkan
// version yang tercatat: menerapkan delta di sisi klien berarti menyalin ulang
// seluruh aturan penyuntingan yang sudah dimiliki server, dan salinan itu pasti
// melenceng. Ketika isi terkini benar-benar dibutuhkan, document.get jauh lebih
// murah daripada kesalahan yang tidak terlihat.
func (s *Document) apply(payload []byte) {
	// Page bertipe RawMessage, dan itu BUKAN sekadar penundaan penguraian.
	// Nama field ini membawa dua bentuk yang berbeda tergantung jenis pesannya:
	// objek {width,height} pada snapshot, dan id halaman berupa STRING pada
	// element.created. Satu amplop dengan Page bertipe PageSize karenanya gagal
	// mengurai seluruh pesan element.created — bukan sekadar field itu — dan
	// akibatnya paling menyesatkan: version tidak pernah naik untuk pembuatan
	// elemen, sehingga suntingan yang berhasil terlihat seperti tidak mendarat.
	var amplop struct {
		Type    string          `json:"type"`
		Version int64           `json:"version"`
		Page    json.RawMessage `json:"page"`
		Content json.RawMessage `json:"content"`
		Code    string          `json:"code"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(payload, &amplop); err != nil {
		s.logger.Warn("pesan dokumen tidak dapat diurai",
			"document", s.documentToken, "error", err)

		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch amplop.Type {
	case "snapshot":
		s.content = amplop.Content
		s.version = amplop.Version
		s.snapshots++

		// Ketiadaan field dibedakan dari field yang cacat. Yang pertama bukan
		// kejadian yang perlu dicatat; mencatatnya hanya menumpuk peringatan yang
		// tidak menunjuk apa pun, dan peringatan semacam itu membuat yang
		// sungguhan ikut diabaikan.
		if len(amplop.Page) > 0 {
			var ukuran PageSize
			if err := json.Unmarshal(amplop.Page, &ukuran); err != nil {
				s.logger.Warn("ukuran kertas pada snapshot tidak dapat diurai",
					"document", s.documentToken, "error", err)
			} else {
				s.page = ukuran
			}
		}
	case "error":
		// Ditampung DAN dicatat. Tidak mematikan sesi: galat tingkat pesan berarti
		// satu suntingan ditolak, bukan koneksinya tidak berlaku lagi.
		//
		// Penampungnya dibatasi supaya sesi yang lama hidup dengan banyak
		// penolakan tidak menumbuhkan memori tanpa henti.
		if len(s.recentErrors) < 32 {
			s.recentErrors = append(s.recentErrors,
				fmt.Sprintf("%s: %s", amplop.Code, amplop.Message))
		}
		s.logger.Warn("galat dari server dokumen",
			"document", s.documentToken, "code", amplop.Code, "message", amplop.Message)
	default:
		if amplop.Version > s.version {
			s.version = amplop.Version
		}
	}
}

// Origin adalah penanda penyunting milik sesi ini.
func (s *Document) Origin() string { return s.origin }

// Send mengirim satu pesan sunting ke server.
//
// TIDAK menunggu balasan, karena kontraknya memang tidak membalas — hanya
// element.create yang menjawab, dan itu pun hanya saat ditolak. Konfirmasi
// datang belakangan lewat siaran, dan penolakan lewat TakeErrors.
func (s *Document) Send(ctx context.Context, pesan any) error {
	s.mu.Lock()
	s.lastUsed = time.Now()
	gagal := s.fatal
	s.mu.Unlock()

	if gagal != nil {
		return gagal
	}

	return s.send(ctx, pesan)
}

// TakeErrors mengambil galat yang terkumpul lalu mengosongkan penampungnya.
//
// Dikosongkan supaya penolakan dari satu pemanggilan tool tidak dilaporkan
// ulang pada pemanggilan berikutnya — dan pemanggil berikutnya tidak menyangka
// suntingannya gagal padahal yang gagal suntingan sebelumnya.
func (s *Document) TakeErrors() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	galat := s.recentErrors
	s.recentErrors = nil

	return galat
}

// Version mengembalikan nomor revisi terkini yang tercatat sesi ini.
func (s *Document) Version() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.version
}

func (s *Document) send(ctx context.Context, pesan any) error {
	payload, err := json.Marshal(pesan)
	if err != nil {
		return err
	}

	return s.conn.Write(ctx, websocket.MessageText, payload)
}

// State mengembalikan potret terkini dan menandai sesi ini baru dipakai.
func (s *Document) State() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastUsed = time.Now()
	if s.fatal != nil {
		return State{}, s.fatal
	}

	// Diurai di sini, bukan saat snapshot tiba: yang tiba disimpan mentah, dan
	// penguraiannya hanya dibayar ketika ada yang benar-benar meminta isinya.
	var isi any
	if s.content != nil {
		if err := json.Unmarshal(s.content, &isi); err != nil {
			return State{}, fmt.Errorf("urai isi dokumen: %w", err)
		}
	}

	return State{
		DocumentToken: s.documentToken,
		Version:       s.version,
		Page:          s.page,
		Content:       isi,
	}, nil
}

func (s *Document) idleSince(now time.Time) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	return now.Sub(s.lastUsed)
}

func (s *Document) alive() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

func (s *Document) Close() {
	s.cancel()
	_ = s.conn.Close(websocket.StatusNormalClosure, "selesai")
}
