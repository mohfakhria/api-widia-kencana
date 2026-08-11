package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/documentdesign"

	"github.com/coder/websocket"
)

// Daur hidup satu koneksi WebSocket — keempat loop-nya, dan pengubahan pesan
// masuk menjadi tindakan.

const (
	// Batas ukuran satu pesan masuk. Tanpa ini satu klien dapat mengirim frame
	// sebesar apa pun dan backend akan menampungnya di memori.
	designMaxMessageBytes = 1 << 20 // 1 MB

	// Batas panjang antrean per arah, per klien. Melewatinya berarti klien
	// tertinggal terlalu jauh atau membanjiri lebih cepat daripada kemampuan
	// proses, dan koneksinya diputus.
	//
	// Angka ini menakar TERSENDAT, bukan laju. Klien yang benar-benar lebih lambat
	// daripada arus perubahan tidak akan menyusul berapa pun antreannya; yang
	// ditutup batas ini adalah jeda sesaat — satu siklus GC, satu render berat —
	// yang setelahnya klien mengejar dengan cepat. Menaikkannya memperpanjang jeda
	// yang dapat ditahan, bukan menambah kemampuan siapa pun.
	//
	// Dinaikkan dari 64 ketika penyuntingan elemen dan halaman masuk: siaran
	// perubahan memakai Send, yang MEMUTUS koneksi saat antreannya penuh, dan
	// penyuntingan ramai menghasilkan siaran jauh lebih deras daripada snapshot
	// dan presence saja. Slot antrean hanya berisi pointer, jadi ongkosnya sepele.
	designQueueLimit = 256

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
	case dto.DesignMessageCursorMove:
		h.moveCursor(documentToken, payload, subscriber)
	case dto.DesignMessageTextMeasure:
		h.measureText(ctx, payload, subscriber)
	case dto.DesignMessageElementCreate:
		h.createElement(ctx, documentToken, payload, subscriber)
	case dto.DesignMessageElementUpdate:
		h.updateElement(documentToken, payload, subscriber)
	case dto.DesignMessageElementDelete:
		h.deleteElement(documentToken, payload, subscriber)
	case dto.DesignMessageElementReorder:
		h.reorderElement(documentToken, payload, subscriber)
	case dto.DesignMessagePageCreate:
		h.createPage(ctx, documentToken, payload, subscriber)
	case dto.DesignMessagePageUpdate:
		h.updatePage(documentToken, payload, subscriber)
	case dto.DesignMessagePageDelete:
		h.deletePage(ctx, documentToken, payload, subscriber)
	case dto.DesignMessagePageReorder:
		h.reorderPage(documentToken, payload, subscriber)
	case dto.DesignMessageUndo:
		// Tanpa muatan sama sekali, jadi tidak ada yang perlu diurai. Yang
		// dibatalkan selalu kelompok perubahan terakhir pada dokumen ini, siapa pun
		// yang membuatnya.
		h.service.Undo(documentToken, subscriber)
	case dto.DesignMessageRedo:
		h.service.Redo(documentToken, subscriber)
	default:
		// Menolak dengan kode yang jelas, bukan diam. Jenis yang tidak dikenal
		// hampir selalu berarti frontend dan backend memegang kontrak yang berbeda,
		// dan itu jauh lebih murah ditemukan sekarang daripada lewat gejalanya.
		subscriber.sendError("unsupported_message_type",
			fmt.Sprintf("message type %q is not handled yet", inbound.Type))
	}
}

// moveCursor mengurai letak kursor dari muatannya.
//
// Diurai ulang di sini karena dispatch hanya membaca amplopnya. Ongkosnya kecil
// dan hanya dibayar oleh jenis pesan yang memang membutuhkannya.
//
// Muatan yang cacat dijatuhkan DIAM-DIAM, tanpa pesan error — berbeda dari jenis
// pesan lain. Kursor datang puluhan kali per detik; membalas kesalahan pada laju
// itu hanya akan membanjiri klien dengan pesan yang tidak dapat ia perbuat
// apa-apa, dan pada gilirannya memenuhi antrean keluarnya sendiri.
func (h *DocumentDesignHandler) moveCursor(documentToken string, payload []byte, subscriber *designSubscriber) {
	var move dto.DesignCursorMove
	if err := json.Unmarshal(payload, &move); err != nil || move.Page == "" {
		return
	}

	h.service.MoveCursor(documentToken, subscriber.userID, move.Page, move.X, move.Y)
}

// sendSnapshot meneruskan permintaan ke room. Snapshot-nya sendiri dimasukkan ke
// antrean keluar oleh orchestrator, bukan di sini — itulah yang menjamin ia tidak
// pernah disalip siaran perubahan berikutnya.
func (h *DocumentDesignHandler) sendSnapshot(ctx context.Context, documentToken string, subscriber *designSubscriber) {
	member := documentdesign.Member{
		Subscriber: subscriber,
		UserID:     subscriber.userID,
		UserName:   subscriber.userName,
	}

	if err := h.service.Sync(ctx, documentToken, member); err != nil {
		h.logger.Warn("sync document design", "document", documentToken, "error", err)
		subscriber.sendError("document_unavailable", err.Error())
	}
}
