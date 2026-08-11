package http

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/documentdesign"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

// Permukaan HTTP untuk document design: penerbitan tiket dan handshake WebSocket.

type DocumentDesignHandler struct {
	service *documentdesign.Service
	// measure TIDAK lewat Service maupun orchestrator. Pengukuran tidak menyentuh
	// isi dokumen, jadi ia dilayani di goroutine koneksi pengirimnya sendiri —
	// menaruhnya di antrean room berarti menghitung tata letak di jalur yang sama
	// dengan setiap geseran elemen orang lain, dan geseran itulah yang paling
	// tidak boleh tersendat.
	measure input.DocumentMeasureUseCase
	logger  *slog.Logger
	origins []string

	// appCtx adalah context aplikasi, bukan context request. Ia disimpan karena
	// setiap koneksi harus ikut mati saat aplikasi berhenti, sedangkan context
	// request tidak dapat dipakai untuk itu.
	appCtx context.Context
}

func NewDocumentDesignHandler(
	appCtx context.Context,
	service *documentdesign.Service,
	measure input.DocumentMeasureUseCase,
	cfg config.Config,
	logger *slog.Logger,
) *DocumentDesignHandler {
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
		measure: measure,
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

	subscriber := &designSubscriber{
		buffer:   buffer,
		cancel:   cancel,
		userID:   ticket.UserID,
		userName: ticket.UserName,
	}

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

// designOriginPatterns membatasi handshake ke host frontend.
//
// Daftarnya sama persis dengan yang dipakai CORS — lihat origin.go. Bila tidak
// ada satu pun pola yang terkumpul, nil dikembalikan dan pustaka hanya
// mengizinkan same-origin, default paling aman.
func designOriginPatterns(cfg config.Config) []string {
	patterns := allowedOriginPatterns(cfg)
	if len(patterns) == 0 {
		return nil
	}

	return patterns
}
