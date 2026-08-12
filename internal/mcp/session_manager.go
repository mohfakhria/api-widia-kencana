package mcp

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// SessionManager memegang satu socket per dokumen yang sedang dikerjakan.
//
// SATU, bukan satu per pemanggilan tool. Agent kerap menyunting dokumen yang
// sama berkali-kali dalam satu perintah, dan membuka socket tiap kali berarti
// agent muncul lalu hilang dari daftar penghuni berulang-ulang — persis yang
// membuat "menonton agent bekerja" tidak mungkin.
type SessionManager struct {
	api    *APIClient
	logger *slog.Logger

	mu       sync.Mutex
	sessions map[string]*DocumentSession
}

func NewSessionManager(api *APIClient, logger *slog.Logger) *SessionManager {
	return &SessionManager{
		api:      api,
		logger:   logger,
		sessions: make(map[string]*DocumentSession),
	}
}

// Session mengembalikan sesi hidup untuk satu dokumen, membukanya bila perlu.
//
// Sesi yang sudah mati DIBUANG lalu dibuka ulang, bukan dikembalikan apa
// adanya. Koneksi berakhir karena banyak sebab yang wajar — API dijalankan
// ulang, jaringan tersendat, klien dianggap tertinggal — dan pemanggil tidak
// semestinya menangani satu pun di antaranya.
func (m *SessionManager) Session(ctx context.Context, documentToken string) (*DocumentSession, error) {
	m.mu.Lock()
	ada, punya := m.sessions[documentToken]
	if punya && ada.alive() {
		m.mu.Unlock()
		return ada, nil
	}
	if punya {
		delete(m.sessions, documentToken)
	}
	m.mu.Unlock()

	if punya {
		ada.Close()
		m.logger.Info("sesi dokumen mati, membuka ulang", "document", documentToken)
	}

	// Membuka DI LUAR kunci. Penerbitan tiket dan handshake menyentuh jaringan,
	// dan menahan mutex selama itu akan membekukan seluruh dokumen lain hanya
	// karena satu koneksi lambat.
	baru, err := openDocumentSession(ctx, m.api, documentToken, m.logger)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Lomba: dua pemanggilan yang sama-sama menemukan sesi mati akan sama-sama
	// membuka. Yang kalah menutup miliknya sendiri, bukan menimpa — menimpa
	// akan meninggalkan socket menganggur tanpa pemilik, dan agent muncul dua
	// kali di daftar penghuni.
	if pemenang, punya := m.sessions[documentToken]; punya && pemenang.alive() {
		baru.Close()
		return pemenang, nil
	}

	m.sessions[documentToken] = baru

	return baru, nil
}

// Sweep menutup sesi yang menganggur atau sudah mati.
//
// Dipanggil denyut, bukan timer per sesi: satu goroutine untuk seluruh peta
// jauh lebih mudah ditalar daripada satu timer per dokumen yang harus
// dibatalkan dengan benar setiap kali sesinya dipakai.
func (m *SessionManager) Sweep(now time.Time) {
	m.mu.Lock()
	var tutup []*DocumentSession
	for token, s := range m.sessions {
		switch {
		case !s.alive():
			delete(m.sessions, token)
			tutup = append(tutup, s)
		case s.idleSince(now) > sessionIdleTimeout:
			delete(m.sessions, token)
			tutup = append(tutup, s)
			m.logger.Info("menutup sesi dokumen yang menganggur", "document", token)
		}
	}
	m.mu.Unlock()

	// Penutupan di luar kunci: Close menyentuh jaringan.
	for _, s := range tutup {
		s.Close()
	}
}

// Run menyapu berkala sampai ctx berakhir, lalu menutup seluruh sesi.
//
// Goroutine ini punya pemilik yang jelas — Server.Run — dan syarat berhenti yang
// jelas. Penutupan di akhir bukan kerapian: socket yang ditinggalkan membuat
// agent tetap tampak hadir di daftar penghuni sampai server mendeteksi ping yang
// tidak terbalas, dan selama itu orang melihat penyunting yang sudah tidak ada.
func (m *SessionManager) Run(ctx context.Context) {
	denyut := time.NewTicker(sessionSweepInterval)
	defer denyut.Stop()

	for {
		select {
		case now := <-denyut.C:
			m.Sweep(now)
		case <-ctx.Done():
			m.closeAll()
			return
		}
	}
}

func (m *SessionManager) closeAll() {
	m.mu.Lock()
	semua := make([]*DocumentSession, 0, len(m.sessions))
	for token, s := range m.sessions {
		semua = append(semua, s)
		delete(m.sessions, token)
	}
	m.mu.Unlock()

	for _, s := range semua {
		s.Close()
	}
}
