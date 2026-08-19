package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// DocumentRenderer menggambar isi dokumen menjadi berkas.
//
// Renderer sengaja tidak mengambil apa pun sendiri. Gambar sudah diambil
// pemanggil dan diserahkan lewat Images, sehingga renderer tidak pernah menyentuh
// jaringan maupun database — dan karenanya tidak pernah dapat diarahkan mengambil
// alamat yang ditentukan klien lewat isi dokumen. Satu-satunya keluarannya selain
// berkas PDF adalah catatan log tentang font yang harus diganti.
type DocumentRenderer interface {
	RenderPDF(ctx context.Context, document RenderDocument) ([]byte, error)
}

// RenderDocument adalah satu dokumen siap gambar.
//
// Ukuran halaman sudah dalam titik, sama seperti seluruh koordinat di dalam
// Content. Konversi dari satuan asli kertas terjadi sebelum sampai ke sini.
type RenderDocument struct {
	// Token dipakai renderer hanya untuk menyebut dokumen mana yang fontnya
	// harus diganti. Tanpa itu, catatannya tidak dapat ditelusuri kembali ke
	// dokumen yang bersangkutan pada server yang melayani banyak orang.
	Token      string
	Content    *design.Content
	PageWidth  float64
	PageHeight float64

	// Images dikunci token aset. Elemen gambar yang tokennya tidak ada di sini
	// dilewati, bukan menggagalkan seluruh ekspor: satu aset yang hilang tidak
	// sepadan dengan dokumen yang tidak bisa dicetak sama sekali.
	Images map[string]RenderImage

	// Fonts diserahkan dengan alasan yang sama persis seperti Images, dan itu
	// sebabnya ia pindah ke sini dari konstruktor renderer: berkasnya kini
	// tinggal di object storage, dan renderer tidak boleh menjadi pihak yang
	// mengambilnya.
	//
	// Berisi SELURUH muka huruf dari keluarga yang dipakai dokumen, bukan hanya
	// yang persis diminta. Renderer menyelesaikan bobot yang tidak ada dengan
	// memilih yang terdekat DI DALAM keluarga itu, dan pilihan tersebut hanya
	// benar bila ia melihat seluruh isinya — diberi sepotong, ia akan jatuh ke
	// Helvetica padahal keluarganya sendiri masih punya bobot yang berdekatan.
	Fonts map[FontFace][]byte
}

// FontFace mengidentifikasi satu muka huruf sebagaimana elemen menyebutnya.
//
// Family di sini bentuk yang dipakai elemen setelah dihuruf-kecilkan — "barlow
// condensed" dengan spasinya — BUKAN slug nama objeknya. Renderer mencari dengan
// nama yang dibawa elemen, dan penerjemahan ke nama objek terjadi sebelum sampai
// ke sini.
type FontFace struct {
	Family string
	Weight int
	Style  string
}

type RenderImage struct {
	Data     []byte
	MimeType string
}
