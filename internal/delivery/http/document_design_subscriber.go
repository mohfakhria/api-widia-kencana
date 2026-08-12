package http

import (
	"context"
	"errors"
	"sync"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/domain"

	"github.com/coder/websocket"
)

// designSubscriber menjembatani room dengan satu koneksi. Implementasi port
// documentdesign.Subscriber.

// designSubscriber menjembatani room dengan satu koneksi. Room hanya mengenal
// Send dan Disconnect; segala hal tentang frame dan socket berhenti di sini.
type designSubscriber struct {
	buffer *designBuffer
	cancel context.CancelFunc

	// Identitas pemilik koneksi, disalin dari tiket saat handshake. Hanya dibaca,
	// tidak pernah berubah selama koneksi hidup, jadi tidak butuh penguncian.
	userID   int64
	userName string

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

// SendEphemeral menggantikan pesan sejenis yang masih mengantre, dan tidak
// pernah memutus koneksi.
//
// Dua sifat, keduanya disengaja. Menggantikan: klien yang tersendat menerima
// posisi terkini saat ia menyusul, bukan tumpukan posisi basi — dan karena
// penggantian terjadi di tempat, satu aliran hanya pernah menempati satu slot.
// Tidak memutus: bila antrean terlanjur penuh oleh pesan lain, kiriman ini
// dilewati begitu saja, karena gerakan berikutnya akan datang beberapa milidetik
// lagi.
func (s *designSubscriber) SendEphemeral(stream string, payload []byte) {
	_ = s.buffer.outbound.conflate(stream, payload)
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
