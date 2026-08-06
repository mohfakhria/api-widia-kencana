package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
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

	return &DocumentDesignHandler{
		service: service,
		logger:  logger,
		origins: designOriginPatterns(cfg),
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

	// Tiket ditukar sebelum upgrade, sehingga kegagalannya berupa respons HTTP
	// biasa yang punya status jelas, bukan koneksi yang terlanjur terbuka.
	if _, err := h.service.Redeem(r.URL.Query().Get("ticket"), documentToken); err != nil {
		writeDesignError(w, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	// Accept menolak Origin asing dengan HTTP 403 dan menulis responsnya sendiri.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: h.origins,
	})
	if err != nil {
		h.logger.Warn("document design handshake failed", "document", documentToken, "error", err)
		return
	}
	defer conn.CloseNow()

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
		h.writeLoop(connCtx, conn, buffer, cancel)
	}()
	go func() {
		defer loops.Done()
		h.pingLoop(connCtx, conn, cancel)
	}()

	room := h.service.Join(documentToken, subscriber)
	defer h.service.Leave(documentToken, subscriber)

	content, version := room.Snapshot()
	snapshot, err := dto.NewDesignSnapshotMessage(content, version)
	if err != nil {
		h.logger.Error("encode document design snapshot", "document", documentToken, "error", err)
		conn.Close(websocket.StatusInternalError, "failed to encode snapshot")
		return
	}
	subscriber.Send(snapshot)

	h.dispatchLoop(buffer, subscriber)

	// Bereskan kedua loop sebelum menutup koneksi, supaya penulis sempat
	// menguras antrean dan tidak ada yang menulis ke socket yang sudah tutup.
	cancel()
	buffer.close()
	loops.Wait()

	conn.Close(websocket.StatusNormalClosure, "")
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
func (h *DocumentDesignHandler) writeLoop(ctx context.Context, conn *websocket.Conn, buffer *designBuffer, cancel context.CancelFunc) {
	defer cancel()

	for {
		batch, ok := buffer.outbound.dequeue()
		if !ok {
			return
		}

		for _, payload := range batch {
			if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
				return
			}
		}
	}
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
func (h *DocumentDesignHandler) dispatchLoop(buffer *designBuffer, subscriber *designSubscriber) {
	for {
		batch, ok := buffer.inbound.dequeue()
		if !ok {
			return
		}

		for _, payload := range batch {
			h.dispatch(payload, subscriber)
		}
	}
}

func (h *DocumentDesignHandler) dispatch(payload []byte, subscriber *designSubscriber) {
	var inbound dto.DesignInbound
	if err := json.Unmarshal(payload, &inbound); err != nil {
		subscriber.sendError(0, "malformed_message", "message is not valid JSON")
		return
	}
	if inbound.Type == "" {
		subscriber.sendError(inbound.Seq, "missing_message_type", "message type is required")
		return
	}

	// Penerapan perubahan belum dibangun. Menolak dengan kode yang jelas lebih
	// baik daripada diam, supaya ketidakcocokan kontrak langsung terlihat.
	subscriber.sendError(inbound.Seq, "unsupported_message_type",
		fmt.Sprintf("message type %q is not handled yet", inbound.Type))
}

// designSubscriber menjembatani room dengan satu koneksi. Room hanya mengenal
// metode Send; segala hal tentang frame dan socket berhenti di sini.
type designSubscriber struct {
	buffer *designBuffer
	cancel context.CancelFunc
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

func (s *designSubscriber) sendError(seq int64, code, message string) {
	payload, err := dto.NewDesignErrorMessage(seq, code, message)
	if err != nil {
		s.cancel()
		return
	}

	s.Send(payload)
}

// writeDesignError memakai bentuk error yang sama dengan endpoint REST lain,
// karena kegagalan handshake terjadi sebelum upgrade dan masih berupa HTTP biasa.
func writeDesignError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(dto.APIErrorResponse{
		Status:  "error",
		Message: message,
	})
}

// designOriginPatterns membatasi handshake ke host frontend. Bila FrontendURL
// tidak dapat diurai, nil dikembalikan dan pustaka hanya mengizinkan same-origin,
// yang merupakan default paling aman.
func designOriginPatterns(cfg config.Config) []string {
	parsed, err := url.Parse(cfg.FrontendURL)
	if err != nil || parsed.Host == "" {
		return nil
	}

	return []string{parsed.Host}
}
