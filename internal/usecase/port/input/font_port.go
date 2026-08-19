package input

import "context"

type FontUseCase interface {
	// Register membuka arsip, membaca setiap berkas font di dalamnya, lalu
	// menyimpannya.
	//
	// Mengembalikan hasil PER BERKAS, bukan satu galat untuk seluruh arsip. Satu
	// berkas yang tidak dapat dibaca — atau yang memang bukan font, seperti
	// OFL.txt — adalah kejadian biasa di dalam arsip yang sah, dan menggagalkan
	// lima belas muka huruf lain karenanya jauh lebih merugikan daripada
	// melaporkan mana yang dilewati beserta sebabnya.
	Register(ctx context.Context, cmd RegisterFontCommand) ([]FontFaceResult, error)

	// List menyebut font yang terpasang, dikelompokkan per keluarga.
	//
	// Sumbernya isi object storage, bukan tabel — nama objeknya fungsi murni
	// dari family, bobot, dan style, jadi bucket itulah daftarnya. Tidak ada
	// keadaan kedua yang bisa melenceng dari berkas yang sungguh ada.
	List(ctx context.Context) ([]FontFamilyListing, error)

	// Content mengembalikan alamat sementara untuk satu muka huruf, dipakai rute
	// publik yang dituju @font-face.
	Content(ctx context.Context, family string, weight int, style string) (string, error)
}

// RegisterFontCommand membawa arsip yang diunggah.
//
// Isinya berkas ZIP apa adanya — yang diunduh orang dari fonts.google.com, atau
// dari mana pun font itu berasal. Tidak ada nama keluarga maupun daftar bobot:
// keduanya dibaca dari dalam tiap berkas font, sehingga tidak ada yang perlu
// diketik ulang dan tidak ada yang bisa salah ketik.
type RegisterFontCommand struct {
	Archive []byte
}

// FontFaceResult melaporkan nasib satu entri arsip.
type FontFaceResult struct {
	// Entry adalah path DI DALAM arsip, dan ia satu-satunya field yang selalu
	// terisi. Ia yang menghubungkan baris ini kembali ke berkas yang dipilih
	// pengunggah — satu-satunya nama yang ia kenali.
	Entry  string
	Family string
	Weight int
	Style  string
	// ObjectName adalah nama objek di penyimpanan, dan ia terisi hanya setelah
	// jati diri font terbaca. Sengaja TERPISAH dari Entry: keduanya sempat satu
	// field, dan itu membuat satu kunci berarti dua hal yang berbeda di dalam
	// larik yang sama — persis jenis hal yang kelak dibaca keliru.
	ObjectName string
	Size       int64
	Stored     bool
	// Reason terisi hanya bila Stored bernilai false, dan berisi kalimat yang
	// aman ditampilkan kepada pengguna.
	Reason string
}

// FontFamilyListing adalah satu keluarga beserta muka huruf yang terpasang.
type FontFamilyListing struct {
	// Family adalah slug, dan itu nama KANONIKnya. Elemen boleh mengirim
	// "Barlow Condensed" maupun "barlow-condensed"; keduanya di-slug dengan
	// aturan yang sama sebelum dicari, sehingga slug inilah satu-satunya bentuk
	// yang dijamin berjalan bolak-balik.
	Family string
	Faces  []FontFaceListing
}

type FontFaceListing struct {
	Weight int
	Style  string
	Size   int64
	// ContentPath adalah alamat stabil untuk @font-face. Ia BUKAN presigned URL:
	// presign berumur menit, sedangkan aturan @font-face hidup selama halaman
	// terbuka dan dapat mengambil berkasnya kapan saja.
	ContentPath string
}
