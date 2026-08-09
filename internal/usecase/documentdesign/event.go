package documentdesign

import (
	"encoding/json"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
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

func (syncEvent) isRoomEvent() {}

type leaveEvent struct {
	subscriber Subscriber
}

func (leaveEvent) isRoomEvent() {}

// snapshotEvent meminta salinan isi terkini, dipakai ekspor.
//
// Ekspor wajib lewat sini dan bukan membaca database, karena penyimpanan bersifat
// tunda: perubahan terakhir bisa tertinggal sampai satu denyut flush penuh. Tanpa
// jalur ini pengguna dapat menggeser sebuah elemen lalu mengekspor dan menerima
// PDF yang belum memuat geseran itu.
type snapshotEvent struct {
	reply chan<- snapshotResult
}

func (snapshotEvent) isRoomEvent() {}

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

func (cursorMoveEvent) isRoomEvent() {}

// Seluruh kejadian penyuntingan membawa subscriber pengirimnya, semata-mata
// untuk memeriksa keanggotaan. Koneksi yang terbuka tetapi belum meminta dokumen belum
// pernah melihat isinya, jadi suntingan darinya menunjuk keadaan yang tidak pernah
// ia miliki.
//
// Pemeriksaannya per KONEKSI, bukan per orang seperti kursor. Kursor dikunci per
// orang sehingga tab mana pun boleh menggerakkannya; suntingan menunjuk isi
// dokumen, dan hanya koneksi yang sudah menerima snapshot yang punya dasar untuk
// menunjuknya.

// elementCreateEvent satu-satunya yang punya reply.
//
// Hanya penambahan yang dapat gagal di dalam orchestrator — halaman tidak ada,
// atau id sudah dipakai — dan kegagalan itu wajib sampai ke pengirimnya, kalau
// tidak elemen yang sudah tergambar optimistis di layarnya tidak akan pernah ada
// di dokumen. Ketiga sisanya paling banter tidak berlaku, dan itu menyatu dengan
// sendirinya.
type elementCreateEvent struct {
	subscriber Subscriber
	page       string
	element    design.Element
	reply      chan<- error
}

type elementUpdateEvent struct {
	subscriber Subscriber
	element    design.Element
}

type elementDeleteEvent struct {
	subscriber Subscriber
	id         string
}

type elementReorderEvent struct {
	subscriber Subscriber
	id         string
	index      int
}

func (elementCreateEvent) isRoomEvent() {}

func (elementUpdateEvent) isRoomEvent() {}

func (elementDeleteEvent) isRoomEvent() {}

func (elementReorderEvent) isRoomEvent() {}

// Penyuntingan halaman. Dua dari tiga membalas, karena dua dari tiga dapat
// ditolak: id halaman sudah dipakai atau batas jumlah halaman tersentuh pada
// pembuatan, dan halaman terakhir pada penghapusan.
//
// Perhatikan bedanya dengan elemen: elementDeleteEvent tidak membalas, sedangkan
// pageDeleteEvent membalas. Menghapus elemen tidak punya jalur penolakan sama
// sekali; menghapus halaman punya satu, dan akibat mendiamkannya adalah dokumen
// yang ditimpa panduan bawaan ketika orang berikutnya membukanya.

type pageCreateEvent struct {
	subscriber Subscriber
	id         string
	// index kosong berarti di akhir. Pointer, karena "tidak menyebut posisi" dan
	// "posisi nol" adalah dua maksud yang berbeda dan keduanya sah.
	index *int
	reply chan<- error
}

type pageDeleteEvent struct {
	subscriber Subscriber
	id         string
	reply      chan<- error
}

// pageUpdateEvent tidak membalas, sama seperti elementUpdateEvent. Muatan yang
// tidak lengkap sudah ditolak di lapisan delivery sebelum menyentuh orchestrator,
// sehingga yang tersisa di sini paling banter tidak berlaku.
type pageUpdateEvent struct {
	subscriber Subscriber
	id         string
	title      string
	hidden     bool
	locked     bool
}

func (pageUpdateEvent) isRoomEvent() {}

type pageReorderEvent struct {
	subscriber Subscriber
	id         string
	index      int
}

func (pageCreateEvent) isRoomEvent() {}

func (pageDeleteEvent) isRoomEvent() {}

func (pageReorderEvent) isRoomEvent() {}

// undoEvent dan redoEvent memindahkan SELURUH dokumen satu langkah, bukan langkah
// pengirimnya sendiri. Tidak ada argumen: yang dibatalkan selalu kelompok
// perubahan terakhir, siapa pun yang membuatnya.
//
// subscriber tetap dibawa untuk memeriksa keanggotaan, sama seperti penyuntingan.

type undoEvent struct {
	subscriber Subscriber
}

type redoEvent struct {
	subscriber Subscriber
}

func (undoEvent) isRoomEvent() {}

func (redoEvent) isRoomEvent() {}

// saveResult dikirim goroutine penyimpan kembali ke orchestrator.
type saveResult struct {
	version int64
	err     error
}
