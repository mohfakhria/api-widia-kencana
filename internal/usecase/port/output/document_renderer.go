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
}

type RenderImage struct {
	Data     []byte
	MimeType string
}
