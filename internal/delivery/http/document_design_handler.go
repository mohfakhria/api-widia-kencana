package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/documentdesign"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

const (
	// Batas ukuran satu pesan masuk. Tanpa ini satu klien dapat mengirim frame
	// sebesar apa pun dan backend akan menampungnya di memori.
	designMaxMessageBytes = 1 << 20 // 1 MB

	// Batas panjang antrean per arah, per klien. Melewatinya berarti klien
	// tertinggal terlalu jauh atau membanjiri lebih cepat daripada kemampuan
	// proses, dan koneksinya diputus.
	designQueueLimit = 64

	// Jarak antar ping. Tiga puluh detik juga menahan proxy dan load balancer
	// yang biasanya memutus koneksi menganggur di sekitar satu menit.
	designPingInterval = 30 * time.Second

	// Tenggang menunggu pong. Koneksi yang mati terdeteksi paling lambat
	// designPingInterval + designPongTimeout setelah benar-benar putus.
	designPongTimeout = 10 * time.Second

	// Tenggat satu kali penulisan ke socket. Tanpa ini, klien yang berhenti
	// membaca dapat menahan penulis sampai ping gagal, yaitu sekitar 40 detik.
	designWriteTimeout = 5 * time.Second
)

type DocumentDesignHandler struct {
	service *documentdesign.Service
	logger  *slog.Logger
	origins []string

	// appCtx adalah context aplikasi, bukan context request. Ia disimpan karena
	// setiap koneksi harus ikut mati saat aplikasi berhenti, sedangkan context
	// request tidak dapat dipakai untuk itu.
	appCtx context.Context
}

func NewDocumentDesignHandler(appCtx context.Context, service *documentdesign.Service, cfg config.Config, logger *slog.Logger) *DocumentDesignHandler {
	if logger == nil {
		logger = slog.Default()
	}

	origins := designOriginPatterns(cfg)
	if cfg.IsLocal() {
		// Pelonggaran keamanan sebaiknya terlihat di log, supaya tidak pernah
		// diam-diam aktif di lingkungan yang seharusnya ketat.
		logger.Warn("document design origin check relaxed for local environment",
			"patterns", origins)
	}

	return &DocumentDesignHandler{
		service: service,
		logger:  logger,
		origins: origins,
		appCtx:  appCtx,
	}
}

func (h *DocumentDesignHandler) IssueTicket(c *gin.Context) {
	ticket, ttl, err := h.service.IssueTicket(c.Request.Context(), c.Param("token"), c.GetString("userID"))
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "Design ticket issued successfully", dto.NewDocumentDesignTicketResponse(ticket, int64(ttl.Seconds())))
}

// Connect adalah handler net/http biasa, bukan gin.
//
// coder/websocket menulis status 101 lewat ResponseWriter sebelum mengambil alih
// koneksi, dan untuk gin ia memanggil WriteHeaderNow() agar status itu benar-benar
// terkirim. Sejak gin v1.11, menyentuh response membuat Hijack ditolak, sehingga
// jalur itu buntu. Melayani handshake di luar gin memakai ResponseWriter asli
// menghindari bentrokan itu sekaligus melewati middleware yang tidak berlaku
// untuk WebSocket.
//
// Satu koneksi memakai empat goroutine: pembaca, penulis, dan pendetak ping yang
// dijalankan di sini, ditambah goroutine handler ini sendiri yang bertugas
// sebagai dispatcher.
func (h *DocumentDesignHandler) Connect(w http.ResponseWriter, r *http.Request) {
	documentToken := r.PathValue("token")

	// Accept lebih dulu. Origin diperiksa di dalamnya dan ditolak dengan HTTP 403
	// sebelum upgrade, sehingga permintaan lintas situs tetap tertahan tanpa
	// pernah menyentuh tiket — dan tiket yang sah tidak hangus percuma.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: h.origins,
	})
	if err != nil {
		h.logger.Warn("document design handshake failed", "document", documentToken, "error", err)
		return
	}
	defer conn.CloseNow()

	// Tiket ditukar setelah upgrade supaya kegagalannya dapat disampaikan sebagai
	// close frame beralasan. Browser tidak pernah bisa membaca status HTTP dari
	// handshake WebSocket yang gagal, sedangkan CloseEvent.reason terbaca — jadi
	// inilah satu-satunya cara frontend tahu ia perlu menerbitkan tiket baru.
	ticket, err := h.service.Redeem(r.URL.Query().Get("ticket"), documentToken)
	if err != nil {
		h.logger.Warn("redeem design ticket", "document", documentToken, "error", err)
		conn.Close(websocket.StatusPolicyViolation, "invalid or expired design ticket")
		return
	}

	conn.SetReadLimit(designMaxMessageBytes)

	connCtx, cancel := context.WithCancel(h.appCtx)
	defer cancel()

	buffer := newDesignBuffer(designQueueLimit)
	defer buffer.close()

	subscriber := &designSubscriber{buffer: buffer, cancel: cancel}

	var loops sync.WaitGroup
	loops.Add(3)
	go func() {
		defer loops.Done()
		h.readLoop(connCtx, conn, buffer, cancel)
	}()
	go func() {
		defer loops.Done()
		h.writeLoop(connCtx, documentToken, conn, buffer, cancel)
	}()
	go func() {
		defer loops.Done()
		h.pingLoop(connCtx, conn, cancel)
	}()

	// Koneksinya dicatat, tetapi isi dokumen belum dikirim. Klien yang memutuskan
	// kapan memintanya, lewat pesan document.get.
	open, err := h.service.Attach(documentToken, ticket.UserID)
	if err != nil {
		h.logger.Warn("attach document design connection",
			"document", documentToken, "user", ticket.UserID, "open", open, "error", err)
		// Pesan error dari lapisan usecase memang disusun sebagai frasa yang aman
		// disampaikan ke klien; detail internalnya berhenti di log.
		conn.Close(attachCloseStatus(err), closeReason(err.Error()))
		return
	}
	defer h.service.Detach(documentToken, ticket.UserID, subscriber)

	// Handshake WebSocket dilayani di luar gin, sehingga tidak menghasilkan baris
	// access log seperti rute lain. Dua baris ini penggantinya — tanpanya koneksi
	// yang berhasil sama sekali tidak berjejak, dan hanya kegagalannya yang
	// terlihat.
	connectedAt := time.Now()
	h.logger.Info("document design connected",
		"document", documentToken, "user", ticket.UserID, "open", open)
	defer func() {
		h.logger.Info("document design disconnected",
			"document", documentToken, "user", ticket.UserID,
			"duration", time.Since(connectedAt).Round(time.Millisecond))
	}()

	h.dispatchLoop(connCtx, documentToken, buffer, subscriber)

	// Close frame dikirim lebih dulu, selagi koneksi masih utuh. Membatalkan
	// context membuat pustaka menutup socket seketika, sehingga alasannya tidak
	// akan pernah sampai bila urutannya dibalik.
	//
	// Room dapat memutus koneksi ini beserta alasannya, misalnya karena isinya
	// tidak mungkin lagi disimpan — frontend perlu tahu bahwa berhenti menyunting
	// memang keputusan yang benar, bukan sekadar gangguan jaringan.
	if reason, ok := subscriber.takeCloseReason(); ok {
		conn.Close(websocket.StatusInternalError, reason)
	} else {
		conn.Close(websocket.StatusNormalClosure, "")
	}

	// Bereskan ketiga loop. Penulis sempat menguras antreannya karena buffer
	// sudah ditutup sebelum titik ini.
	cancel()
	buffer.close()
	loops.Wait()
}

// readLoop memindahkan frame dari socket ke antrean masuk.
//
// Ia juga pemikul tanggung jawab menghentikan yang lain: karena hanya loop ini
// yang menunggu dengan context, hanya ia yang tahu aplikasi sedang berhenti, dan
// buffer.close() miliknya yang membangunkan penulis serta dispatcher dari
// cond.Wait(). Bila tanggung jawab ini dipindah, shutdown akan menggantung.
func (h *DocumentDesignHandler) readLoop(ctx context.Context, conn *websocket.Conn, buffer *designBuffer, cancel context.CancelFunc) {
	defer buffer.close()
	defer cancel()

	for {
		messageType, payload, err := conn.Read(ctx)
		if err != nil {
			// Termasuk klien menutup koneksi, context dibatalkan, dan pesan yang
			// melewati batas ukuran. Semuanya berarti koneksi ini selesai.
			return
		}
		if messageType != websocket.MessageText {
			// Kontraknya JSON, dan JSON dikirim sebagai text frame.
			conn.Close(websocket.StatusUnsupportedData, "only JSON text frames are supported")
			return
		}
		if !buffer.inbound.enqueue(payload) {
			// Klien mengirim lebih cepat daripada kemampuan memproses.
			return
		}
	}
}

// writeLoop memindahkan antrean keluar ke socket, menguras seluruh isinya tiap
// kali bangun.
func (h *DocumentDesignHandler) writeLoop(ctx context.Context, documentToken string, conn *websocket.Conn, buffer *designBuffer, cancel context.CancelFunc) {
	defer cancel()

	for {
		batch, ok := buffer.outbound.dequeue()
		if !ok {
			return
		}

		for _, payload := range batch {
			// Dicatat di sini, bukan saat pesan dimasukkan ke antrean. Antrean
			// diisi orchestrator, dan menulis log di sana berarti menahan seluruh
			// penyuntingan dokumen selama I/O log berlangsung.
			h.logMessage(documentToken, "out", payload)

			if err := writeMessage(ctx, conn, payload); err != nil {
				return
			}
		}
	}
}

// logMessage mencatat jejak satu pesan pada level debug.
//
// Jenis pesan diurai hanya bila level debug memang menyala, karena mengurai
// setiap payload keluar sia-sia ketika catatannya toh dibuang. Isi payload
// sengaja tidak ikut dicatat: snapshot dapat berukuran ratusan kilobyte dan
// memuat isi dokumen pengguna.
func (h *DocumentDesignHandler) logMessage(documentToken, direction string, payload []byte) {
	if !h.logger.Enabled(context.Background(), slog.LevelDebug) {
		return
	}

	h.logger.Debug("document design message",
		"document", documentToken,
		"direction", direction,
		"type", designMessageType(payload),
		"bytes", len(payload))
}

// designMessageType membaca field type saja. Payload yang tidak dapat diurai
// dilaporkan apa adanya sebagai "?" — itu sendiri sudah informasi.
func designMessageType(payload []byte) string {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil || envelope.Type == "" {
		return "?"
	}

	return envelope.Type
}

// writeMessage melepaskan penulisan dari pembatalan context koneksi.
//
// Kalau keduanya terikat, pembatalan yang menandai koneksi berakhir juga
// menggagalkan pengiriman pesan yang masih mengantre — dan readLoop memang
// membatalkan context sebelum menutup buffer, sehingga balasan terakhir tidak
// akan pernah sampai. Yang membatasi penulisan sekarang tenggatnya sendiri.
//
// Pengurasan tetap terbatas: writeLoop berhenti pada kegagalan pertama, jadi
// lawan bicara yang sudah pergi paling banyak membayar satu tenggat.
func writeMessage(ctx context.Context, conn *websocket.Conn, payload []byte) error {
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), designWriteTimeout)
	defer cancel()

	return conn.Write(writeCtx, websocket.MessageText, payload)
}

// pingLoop membuat koneksi yang mati diam-diam bisa terdeteksi.
//
// Tanpa ini, laptop yang ditutup atau wifi yang hilang tidak pernah membuat
// conn.Read gagal: socket tetap terbuka bagi kernel, sehingga keempat goroutine
// dan keanggotaan room bertahan selamanya.
//
// conn.Ping mengirim ping lalu menunggu pong-nya, jadi kegagalannya sudah
// berarti lawan bicara tidak responsif. Ia menuntut ada Reader yang berjalan
// bersamaan untuk membaca pong — dipenuhi oleh readLoop yang selalu berada di
// conn.Read. Memanggilnya bersamaan dengan conn.Write dari writeLoop aman:
// seluruh metode Conn boleh dipakai bersamaan kecuali Read dan Reader.
func (h *DocumentDesignHandler) pingLoop(ctx context.Context, conn *websocket.Conn, cancel context.CancelFunc) {
	defer cancel()

	ticker := time.NewTicker(designPingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancelPing := context.WithTimeout(ctx, designPongTimeout)
			err := conn.Ping(pingCtx)
			cancelPing()

			if err != nil {
				return
			}
		}
	}
}

// dispatchLoop berjalan di goroutine handler dan mengubah pesan mentah menjadi
// tindakan. Memisahkannya dari readLoop membuat pembacaan socket tidak ikut
// tertahan ketika pemrosesan sedang menunggu kunci room.
func (h *DocumentDesignHandler) dispatchLoop(ctx context.Context, documentToken string, buffer *designBuffer, subscriber *designSubscriber) {
	for {
		batch, ok := buffer.inbound.dequeue()
		if !ok {
			return
		}

		for _, payload := range batch {
			h.dispatch(ctx, documentToken, payload, subscriber)
		}
	}
}

func (h *DocumentDesignHandler) dispatch(ctx context.Context, documentToken string, payload []byte, subscriber *designSubscriber) {
	h.logMessage(documentToken, "in", payload)

	var inbound dto.DesignInbound
	if err := json.Unmarshal(payload, &inbound); err != nil {
		subscriber.sendError("malformed_message", "message is not valid JSON")
		return
	}

	switch inbound.Type {
	case "":
		subscriber.sendError("missing_message_type", "message type is required")
	case dto.DesignMessageDocumentGet:
		h.sendSnapshot(ctx, documentToken, subscriber)
	default:
		// Penerapan perubahan belum dibangun. Menolak dengan kode yang jelas lebih
		// baik daripada diam, supaya ketidakcocokan kontrak langsung terlihat.
		subscriber.sendError("unsupported_message_type",
			fmt.Sprintf("message type %q is not handled yet", inbound.Type))
	}
}

// sendSnapshot meneruskan permintaan ke room. Snapshot-nya sendiri dimasukkan ke
// antrean keluar oleh orchestrator, bukan di sini — itulah yang menjamin ia tidak
// pernah disalip siaran perubahan berikutnya.
func (h *DocumentDesignHandler) sendSnapshot(ctx context.Context, documentToken string, subscriber *designSubscriber) {
	if err := h.service.Sync(ctx, documentToken, subscriber); err != nil {
		h.logger.Warn("sync document design", "document", documentToken, "error", err)
		subscriber.sendError("document_unavailable", err.Error())
	}
}

// designSubscriber menjembatani room dengan satu koneksi. Room hanya mengenal
// Send dan Disconnect; segala hal tentang frame dan socket berhenti di sini.
type designSubscriber struct {
	buffer *designBuffer
	cancel context.CancelFunc

	// closeReason diisi Disconnect dan dibaca Connect setelah seluruh loop
	// selesai, untuk dijadikan alasan pada close frame. Dijaga mutex karena
	// penulisnya goroutine orchestrator dan pembacanya goroutine handler.
	mu          sync.Mutex
	closeReason string
}

// Send tidak pernah memblokir karena room memanggilnya sambil memegang kunci.
// Antrean yang penuh berarti klien tertinggal terlalu jauh, dan koneksinya
// dibatalkan lewat context — bukan ditutup langsung, karena menutup socket
// adalah I/O dan tidak boleh terjadi di bawah kunci room.
func (s *designSubscriber) Send(payload []byte) {
	if !s.buffer.outbound.enqueue(payload) {
		s.cancel()
	}
}

// Disconnect dipanggil orchestrator sambil memiliki state room, jadi ia tidak
// boleh melakukan I/O apa pun — hanya mencatat alasan lalu menutup buffer.
//
// Menutup buffer, bukan membatalkan context. Membatalkan context membuat
// coder/websocket menutup socket seketika, sehingga close frame beserta
// alasannya tidak akan pernah sampai ke klien. Menutup buffer membangunkan
// dispatcher, dan handler yang mengirim close frame selagi koneksi masih utuh.
func (s *designSubscriber) Disconnect(reason string) {
	s.mu.Lock()
	if s.closeReason == "" {
		s.closeReason = reason
	}
	s.mu.Unlock()

	s.buffer.close()
}

func (s *designSubscriber) takeCloseReason() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closeReason, s.closeReason != ""
}

func (s *designSubscriber) sendError(code, message string) {
	payload, err := dto.NewDesignErrorMessage(code, message)
	if err != nil {
		s.cancel()
		return
	}

	s.Send(payload)
}

// attachCloseStatus memetakan kegagalan menempel ke close code yang membedakan
// "coba lagi nanti" dari "ada yang salah di sisi kami".
func attachCloseStatus(err error) websocket.StatusCode {
	if errors.Is(err, domain.ErrTooManyRequests) {
		return websocket.StatusTryAgainLater
	}

	return websocket.StatusInternalError
}

// closeReason memangkas alasan agar muat pada close frame. Protokol WebSocket
// membatasi alasan sampai 123 byte, dan melewatinya membuat penutupan gagal
// sehingga klien justru tidak menerima alasan apa pun.
func closeReason(reason string) string {
	const maxCloseReasonBytes = 123

	if len(reason) <= maxCloseReasonBytes {
		return reason
	}

	return reason[:maxCloseReasonBytes]
}

// DesignMessageEncoder menyusun payload yang dikirim room ke klien.
//
// Implementasi dari port documentdesign.MessageEncoder. Bentuk kawatnya milik
// lapisan ini; room hanya menyerahkan isi dan nomor versinya.
type DesignMessageEncoder struct{}

func (DesignMessageEncoder) EncodeSnapshot(content json.RawMessage, version int64) ([]byte, error) {
	return dto.NewDesignSnapshotMessage(content, version)
}

// designOriginPatterns membatasi handshake ke host frontend.
//
// Bila tidak ada satu pun pola yang terkumpul, nil dikembalikan dan pustaka
// hanya mengizinkan same-origin — default paling aman.
func designOriginPatterns(cfg config.Config) []string {
	patterns := make([]string, 0, 7)

	if parsed, err := url.Parse(cfg.FrontendURL); err == nil && parsed.Host != "" {
		patterns = append(patterns, parsed.Host)
	}

	if cfg.IsLocal() {
		patterns = append(patterns, localOriginPatterns()...)
	}

	if len(patterns) == 0 {
		return nil
	}

	return patterns
}

// localOriginPatterns adalah host yang ikut diizinkan saat APP_ENV=local.
//
// Di mesin sendiri, port dan subdomain berganti mengikuti cara frontend
// dijalankan — localhost:3000, portal.localhost:3000, dan seterusnya — sehingga
// menyetel ulang FRONTEND_URL setiap kali hanya menghambat.
//
// Pelonggarannya tetap terkurung pada loopback. Akhiran .localhost adalah TLD
// yang dicadangkan RFC 6761 dan tidak dapat didaftarkan publik, jadi tidak ada
// host luar yang bisa menyamar lewat pola ini.
//
// Pencocokannya memakai path.Match, tempat * mencakup apa saja selain garis
// miring. Karena itu pola seperti "localhost*" sengaja dihindari: ia juga akan
// cocok dengan localhost.evil.com. Setiap pola di bawah menambatkan nama host
// secara utuh, dan hanya bagian port yang dibebaskan.
//
// IPv6 tidak disertakan: path.Match memperlakukan "[" sebagai pembuka kelas
// karakter, sehingga pola untuk [::1] akan diartikan sama sekali lain.
func localOriginPatterns() []string {
	return []string{
		"localhost",
		"localhost:*",
		"*.localhost",
		"*.localhost:*",
		"127.0.0.1",
		"127.0.0.1:*",
	}
}
