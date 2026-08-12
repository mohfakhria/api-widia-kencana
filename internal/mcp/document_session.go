package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	// sessionIdleTimeout menutup socket yang tidak dipakai.
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
	sessionIdleTimeout = 5 * time.Minute

	// sessionSweepInterval adalah denyut pemeriksa yang menganggur.
	sessionSweepInterval = 30 * time.Second

	// socketReadLimit menyamai batas pesan di sisi server, 1 MB. Snapshot
	// dokumen besar mendekatinya, dan batas yang lebih kecil di sini akan
	// memutus koneksi tepat pada dokumen yang paling perlu dibaca.
	socketReadLimit = 1 << 20
)

// DocumentSession adalah satu koneksi hidup ke satu dokumen.
//
// Ia memegang isi terkini beserta nomor versinya, disegarkan oleh loop baca yang
// berjalan selama koneksinya hidup. Pembaca dari luar tidak pernah menyentuh
// socket — mereka membaca salinan di bawah mutex, sehingga pemanggilan tool
// tidak pernah menunggu jaringan.
type DocumentSession struct {
	documentToken string
	logger        *slog.Logger

	conn   *websocket.Conn
	cancel context.CancelFunc
	done   chan struct{}

	mu       sync.Mutex
	version  int64
	content  json.RawMessage
	page     DocumentPageSize
	lastUsed time.Time
	// fatal menyimpan alasan koneksi berakhir. Dibaca pemanggil berikutnya,
	// supaya sesi yang mati menjawab "kenapa" alih-alih diam.
	fatal error
}

// DocumentPageSize adalah ukuran kertas yang ikut datang bersama snapshot.
type DocumentPageSize struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// DocumentState adalah potret yang dikembalikan ke pemanggil tool.
//
// Content bertipe any, BUKAN json.RawMessage. Keduanya membawa isi yang sama,
// tetapi RawMessage adalah []byte — dan SDK menyimpulkan skema keluaran dari
// tipe Go-nya, sehingga ia mengumumkan "array" untuk sesuatu yang sebenarnya
// objek, lalu menolak jawabannya sendiri saat validasi.
//
// any juga lebih jujur di sini: bentuk isi dokumen dimiliki domain, dan MCP
// tidak semestinya menyatakan ulang bentuk itu — pernyataan ulang yang melenceng
// justru persoalan yang berkas kontrak bersama diadakan untuk menghapus.
type DocumentState struct {
	DocumentToken string           `json:"document_token"`
	Version       int64            `json:"version"`
	Page          DocumentPageSize `json:"page"`
	Content       any              `json:"content"`
}

// openDocumentSession menerbitkan tiket lalu menyambung, dalam satu tarikan.
//
// Keduanya TIDAK boleh dipisah oleh apa pun yang lama. Tiket hidup tiga puluh
// detik dan hangus sekali pakai; menerbitkannya di awal tugas lalu menunggu
// model menyusun perintah akan gagal dengan close code 1008 yang tidak menyebut
// bahwa penyebabnya tenggat.
func openDocumentSession(ctx context.Context, api *APIClient, documentToken string, logger *slog.Logger) (*DocumentSession, error) {
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
	conn.SetReadLimit(socketReadLimit)

	s := &DocumentSession{
		documentToken: documentToken,
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

	if err := s.waitForSnapshot(sessionCtx); err != nil {
		s.Close()
		return nil, err
	}

	return s, nil
}

// waitForSnapshot menunggu isi pertama tiba.
//
// Tanpa ini, tool pertama yang memakai sesi ini akan mendapat dokumen kosong
// dan tidak dapat membedakannya dari dokumen yang memang kosong.
func (s *DocumentSession) waitForSnapshot(ctx context.Context) error {
	tenggat := time.NewTimer(10 * time.Second)
	defer tenggat.Stop()

	for {
		s.mu.Lock()
		ada, gagal := s.content != nil, s.fatal
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
func (s *DocumentSession) readLoop(ctx context.Context) {
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
func (s *DocumentSession) apply(payload []byte) {
	var amplop struct {
		Type    string           `json:"type"`
		Version int64            `json:"version"`
		Page    DocumentPageSize `json:"page"`
		Content json.RawMessage  `json:"content"`
		Code    string           `json:"code"`
		Message string           `json:"message"`
	}
	if err := json.Unmarshal(payload, &amplop); err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	switch amplop.Type {
	case "snapshot":
		s.content = amplop.Content
		s.version = amplop.Version
		s.page = amplop.Page
	case "error":
		// Dicatat, tidak mematikan sesi. Galat tingkat pesan berarti satu
		// suntingan ditolak, bukan koneksinya tidak berlaku lagi.
		s.logger.Warn("galat dari server dokumen",
			"document", s.documentToken, "code", amplop.Code, "message", amplop.Message)
	default:
		if amplop.Version > s.version {
			s.version = amplop.Version
		}
	}
}

func (s *DocumentSession) send(ctx context.Context, pesan any) error {
	payload, err := json.Marshal(pesan)
	if err != nil {
		return err
	}

	return s.conn.Write(ctx, websocket.MessageText, payload)
}

// State mengembalikan potret terkini dan menandai sesi ini baru dipakai.
func (s *DocumentSession) State() (DocumentState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastUsed = time.Now()
	if s.fatal != nil {
		return DocumentState{}, s.fatal
	}

	// Diurai di sini, bukan saat snapshot tiba: yang tiba disimpan mentah, dan
	// penguraiannya hanya dibayar ketika ada yang benar-benar meminta isinya.
	var isi any
	if s.content != nil {
		if err := json.Unmarshal(s.content, &isi); err != nil {
			return DocumentState{}, fmt.Errorf("urai isi dokumen: %w", err)
		}
	}

	return DocumentState{
		DocumentToken: s.documentToken,
		Version:       s.version,
		Page:          s.page,
		Content:       isi,
	}, nil
}

func (s *DocumentSession) idleSince(now time.Time) time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	return now.Sub(s.lastUsed)
}

func (s *DocumentSession) alive() bool {
	select {
	case <-s.done:
		return false
	default:
		return true
	}
}

func (s *DocumentSession) Close() {
	s.cancel()
	_ = s.conn.Close(websocket.StatusNormalClosure, "selesai")
}
