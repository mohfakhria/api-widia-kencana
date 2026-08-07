package documentdesign

import (
	"encoding/json"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

// roomEvent adalah pesan yang diproses orchestrator satu per satu. Antarmuka
// penanda ini membuat himpunan kejadiannya tertutup: hanya tipe di paket ini
// yang bisa masuk ke inbox.
type roomEvent interface {
	isRoomEvent()
}

// syncEvent adalah permintaan klien atas isi dokumen.
//
// Klien baru menjadi anggota — dan karenanya penerima siaran — pada langkah ini,
// bukan saat socket terbuka. Dengan begitu mustahil ada delta yang sampai
// sebelum snapshot yang menjadi dasarnya.
type syncEvent struct {
	member Member
	// Arah channel dinyatakan lewat tipe: orchestrator hanya boleh mengirim.
	reply chan<- error
}

type leaveEvent struct {
	subscriber Subscriber
}

// snapshotEvent meminta salinan isi terkini, dipakai ekspor.
//
// Ekspor wajib lewat sini dan bukan membaca database, karena penyimpanan bersifat
// tunda: perubahan terakhir bisa tertinggal sampai satu denyut flush penuh. Tanpa
// jalur ini pengguna dapat menggeser sebuah elemen lalu mengekspor dan menerima
// PDF yang belum memuat geseran itu.
type snapshotEvent struct {
	reply chan<- snapshotResult
}

type snapshotResult struct {
	content json.RawMessage
	version int64
	paper   entity.DocumentPaperSize
	err     error
}

// cursorMoveEvent tidak punya reply. Pengirimnya tidak menunggu apa pun, dan
// tidak ada yang berguna untuk dikembalikan — kursor adalah keadaan sesaat.
type cursorMoveEvent struct {
	cursor Cursor
}

func (syncEvent) isRoomEvent() {}

func (leaveEvent) isRoomEvent() {}

func (snapshotEvent) isRoomEvent() {}

func (cursorMoveEvent) isRoomEvent() {}

// saveResult dikirim goroutine penyimpan kembali ke orchestrator.
type saveResult struct {
	version int64
	err     error
}
