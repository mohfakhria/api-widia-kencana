package documentdesign

import "encoding/json"

// Subscriber adalah satu koneksi yang menerima siaran dari room.
//
// Send wajib tidak memblokir. Orchestrator memanggilnya saat menyiarkan, dan
// implementasi yang menunggu I/O akan menghentikan seluruh penyuntingan dokumen
// itu hanya karena satu klien lambat. Disconnect tunduk pada aturan yang sama.
type Subscriber interface {
	// Send untuk pesan yang wajib sampai. Antrean yang penuh berarti klien
	// tertinggal terlalu jauh, dan koneksinya diakhiri.
	Send(payload []byte)

	// SendEphemeral untuk pesan yang hanya berarti nilai terkininya.
	//
	// stream menandai pesan mana yang saling menggantikan: payload baru menimpa
	// pesan sejenis yang masih mengantre, alih-alih menumpuk di belakangnya.
	// Klien yang tersendat karenanya menerima posisi terkini saat ia menyusul,
	// bukan tumpukan posisi basi.
	//
	// Pemisahan dari Send bukan kerapian melainkan keharusan. Kursor yang memakai
	// Send akan MENJATUHKAN sesi penyuntingan begitu klien tersendat sesaat —
	// jeda GC atau satu render berat sudah cukup menumpuk lebih dari kapasitas
	// antrean.
	SendEphemeral(stream string, payload []byte)

	// Disconnect memutus koneksi ini beserta alasannya. Dipanggil ketika room
	// tidak lagi dapat melayani — membiarkan orang menyunting sesuatu yang tidak
	// akan pernah tersimpan jauh lebih buruk daripada memutusnya.
	Disconnect(reason string)
}

// MessageEncoder menyusun payload yang dikirim room ke klien.
//
// Room tidak mengenal bentuk kawatnya; lapisan delivery yang menentukan. Port
// ini ada supaya orchestrator dapat memasukkan snapshot ke antrean keluar
// sendiri, pada langkah yang sama dengan pendaftaran anggota. Bila penyusunan
// payload diserahkan ke goroutine handler, siaran dari perubahan berikutnya
// dapat menyalip snapshot dan klien menerima delta untuk keadaan yang belum ia
// punya.
type MessageEncoder interface {
	EncodeSnapshot(content json.RawMessage, version int64, page PageSize) ([]byte, error)
	EncodePresence(users []PresenceUser) ([]byte, error)
	EncodeCursors(cursors []Cursor) ([]byte, error)
}

// Cursor adalah letak kursor satu orang di atas dokumen.
//
// Dikunci per orang, bukan per koneksi — sama seperti kehadiran. Satu orang
// dengan beberapa tab punya satu kursor, yaitu posisi dari tab yang terakhir
// bergerak; dua kursor berlabel nama yang sama justru membingungkan pembacanya.
type Cursor struct {
	UserID string
	Page   string
	X      float64
	Y      float64
}

// Member adalah satu koneksi beserta pemiliknya.
//
// Identitas dibawa dari tiket, bukan dicari saat koneksi terjadi. Membaca tabel
// user di dalam orchestrator berarti satu query lambat membekukan penyuntingan
// seluruh dokumen itu; tiket sudah diterbitkan lewat endpoint terautentikasi di
// luar jalur realtime, dan umurnya cuma tiga puluh detik sehingga namanya tidak
// mungkin basi.
type Member struct {
	Subscriber Subscriber
	UserID     string
	UserName   string
}

// PresenceUser adalah satu orang yang sedang membuka dokumen.
//
// Yang dihitung orang, bukan koneksi: satu orang dengan tiga tab tetap satu
// entri, dan ia baru hilang ketika tab terakhirnya tertutup. Menghitung koneksi
// akan menampilkan satu orang sebagai beberapa penyunting — dan itu bukan
// kemungkinan teoretis, karena frontend membuka lebih dari satu koneksi tiap kali
// halaman dimuat.
type PresenceUser struct {
	ID   string
	Name string
}

// PageSize adalah ukuran halaman dalam titik, ikut dikirim bersama snapshot.
//
// Tanpa ini frontend tidak punya cara menentukan ukuran kanvas: isi dokumen
// sengaja tidak memuat ukuran halaman, dan satu-satunya sumber lain — endpoint
// detail dokumen — mengembalikan ukuran dalam satuan asli kertas. Menyerahkan
// konversinya ke frontend berarti membuka satu kelas kesalahan baru untuk
// pertanyaan yang jawabannya sudah dipegang backend.
type PageSize struct {
	Width  float64
	Height float64
}
