package documentdesign

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

// roomIdleGrace adalah berapa lama room yang sudah ditinggalkan dipertahankan
// sebelum dibuang.
//
// Gunanya menutup kasus yang paling sering terjadi: refresh halaman dan wifi
// yang kedip sesaat. Tanpa jeda ini, setiap refresh berarti room dibongkar lalu
// dibangun ulang — dan setelah ada persistensi nanti, juga satu pembacaan
// database tiap kali. Dengan jeda, penyambungan kembali mendarat di room yang
// masih hangat berikut isinya.
//
// Menutup tab lalu pergi lebih lama dari ini tetap kehilangan room-nya; untuk
// itu yang dibutuhkan persistensi, bukan jeda yang lebih panjang.
const roomIdleGrace = 10 * time.Second

// roomEntry membungkus room beserta catatan daur hidupnya.
//
// members di sini bukan daftar penghuni — daftar itu dimiliki orchestrator untuk
// menyiarkan. Angka ini hanya menjawab satu pertanyaan: apakah masih ada yang
// memakai room ini. Ia berada di bawah kunci yang sama dengan peta agar keputusan
// "buat" dan "buang" tidak pernah berlomba.
//
// emptyAt bernilai nol selama room masih berpenghuni.
type roomEntry struct {
	room    *Room
	members int
	emptyAt time.Time
}

// manager memegang peta token dokumen ke room yang sedang hidup.
//
// Peta tetap dijaga mutex, bukan diubah jadi actor: kunci mengungkapkan
// invariannya lebih jelas di sini, dan yang benar-benar butuh kepemilikan
// tunggal adalah state dokumen di dalam Room, bukan petanya.
type manager struct {
	// appCtx adalah context aplikasi, bukan context request. Orchestrator hidup
	// lebih lama daripada koneksi mana pun, jadi context miliknya harus mengikuti
	// umur aplikasi — inilah kasus di mana menyimpan context di struct memang
	// dituntut oleh daur hidupnya.
	appCtx    context.Context
	documents output.DocumentRepository
	encoder   MessageEncoder
	logger    *slog.Logger

	mu    sync.Mutex
	rooms map[string]*roomEntry
}

func newManager(appCtx context.Context, documents output.DocumentRepository, encoder MessageEncoder, logger *slog.Logger) *manager {
	return &manager{
		appCtx:    appCtx,
		documents: documents,
		encoder:   encoder,
		logger:    logger,
		rooms:     make(map[string]*roomEntry),
	}
}

// attach mencari-atau-membuat room lalu mencatat satu koneksi memakainya.
//
// Pencarian, pembuatan, dan penambahan pencacah terjadi di bawah satu penguncian.
// Memecahnya jadi "periksa" lalu "buat" akan membuat dua koneksi yang datang
// bersamaan pada dokumen yang sama menghasilkan dua room dengan state bercabang.
//
// Ini belum menjadikan koneksinya anggota room. Keanggotaan — dan karenanya hak
// menerima siaran — baru terjadi saat klien meminta dokumen lewat sync.
func (m *manager) attach(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.rooms[token]
	if !ok {
		entry = &roomEntry{room: newRoom(token, m.documents, m.encoder, m.logger)}
		m.rooms[token] = entry
		go entry.room.run(m.appCtx)
	}

	entry.members++
	// Room ini terpakai lagi, jadi hitung mundur pembuangannya dibatalkan.
	entry.emptyAt = time.Time{}
}

// sync meminta room mengirimkan snapshot ke satu koneksi dan menjadikannya
// anggota.
//
// Room dicari di bawah kunci, tetapi permintaannya dikirim setelah kunci dilepas:
// menunggu inbox di bawah kunci peta akan membekukan seluruh dokumen lain.
func (m *manager) sync(ctx context.Context, token string, member Member) error {
	m.mu.Lock()
	entry, ok := m.rooms[token]
	m.mu.Unlock()

	if !ok {
		return domain.NewError(domain.ErrUnavailable, "document design room is closed")
	}

	return entry.room.sync(ctx, member)
}

// snapshot mengambil isi terkini bila dokumen ini sedang disunting seseorang.
//
// found bernilai false ketika tidak ada room sama sekali. Itu bukan kegagalan
// melainkan keadaan yang paling umum — dokumen yang tidak sedang dibuka siapa
// pun — dan artinya database sudah memuat keadaan terakhirnya.
func (m *manager) snapshot(ctx context.Context, token string) (result snapshotResult, found bool, err error) {
	m.mu.Lock()
	entry, ok := m.rooms[token]
	m.mu.Unlock()

	if !ok {
		return snapshotResult{}, false, nil
	}

	result, err = entry.room.snapshot(ctx)
	if errors.Is(err, errRoomClosed) {
		// Room berhenti tepat di antara pencarian di peta dan pertanyaan ini —
		// bisa terjadi bila penghuni terakhir baru saja pergi dan masa tenggangnya
		// habis. Orchestrator selalu menyimpan sekali lagi sebelum berhenti, jadi
		// database sudah mutakhir dan pemanggil dapat membacanya seperti dokumen
		// yang memang tidak sedang dibuka. Meneruskan kegagalan di sini hanya akan
		// menolak ekspor yang sebenarnya bisa dilayani.
		return snapshotResult{}, false, nil
	}

	// Kegagalan lain diteruskan apa adanya. Room yang masih hidup tetapi tidak
	// dapat dibaca berarti keadaan sebenarnya tidak diketahui, dan database bisa
	// saja tertinggal — menghasilkan PDF dari sana akan memberi pengguna dokumen
	// yang tampak wajar padahal sudah usang.
	return result, true, err
}

// detach mengurangi pencacah dan mulai menghitung mundur begitu koneksi terakhir
// pergi. Room-nya sendiri belum dibuang di sini — itu tugas sweepIdle setelah
// roomIdleGrace terlampaui.
func (m *manager) detach(token string, sub Subscriber) {
	m.mu.Lock()
	entry, ok := m.rooms[token]
	if !ok {
		m.mu.Unlock()
		return
	}

	entry.members--
	if entry.members <= 0 {
		entry.members = 0
		entry.emptyAt = time.Now()
	}
	room := entry.room
	m.mu.Unlock()

	// Aman walau room sudah dihentikan: leave memakai done sebagai jalan keluar.
	room.leave(sub)
}

// sweepIdle membuang room yang sudah kosong melewati masa tenggangnya.
//
// Pemeriksaan dan pembuangan terjadi di bawah kunci yang sama dengan join,
// sehingga koneksi yang menempel pada saat bersamaan tidak bisa mendarat di room
// yang sedang dibuang: join yang lebih dulu mendapat kunci akan menaikkan
// members dan menghapus emptyAt, membuat room ini tidak lagi memenuhi syarat.
//
// Penutupan stop menyatu dengan penghapusan entry, jadi tidak ada jalan untuk
// menutupnya dua kali.
func (m *manager) sweepIdle(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for token, entry := range m.rooms {
		if entry.members > 0 || entry.emptyAt.IsZero() {
			continue
		}
		if now.Sub(entry.emptyAt) < roomIdleGrace {
			continue
		}

		delete(m.rooms, token)
		close(entry.room.stop)
	}
}

// stopAll menghentikan seluruh room lalu menunggu orchestrator-nya benar-benar
// berhenti. Dipanggil saat aplikasi berhenti, termasuk untuk room yang masih
// menunggu masa tenggangnya habis.
//
// Menunggu, bukan sekadar memberi aba-aba, karena begitu persistensi masuk
// orchestrator masih punya pekerjaan menyimpan saat diminta berhenti. Batas waktu
// menjaga agar shutdown tidak pernah menggantung bila satu room tersendat.
func (m *manager) stopAll(timeout time.Duration) {
	m.mu.Lock()
	rooms := make([]*Room, 0, len(m.rooms))
	for token, entry := range m.rooms {
		close(entry.room.stop)
		rooms = append(rooms, entry.room)
		delete(m.rooms, token)
	}
	m.mu.Unlock()

	// Menunggu di luar kunci: menahannya akan membekukan koneksi yang sedang
	// menempel atau keluar pada saat yang sama.
	deadline := time.After(timeout)
	for _, room := range rooms {
		select {
		case <-room.done:
		case <-deadline:
			return
		}
	}
}
