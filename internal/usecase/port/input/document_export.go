package input

import "context"

type DocumentExportUseCase interface {
	ExportPDF(ctx context.Context, documentToken string) (*DocumentExportResult, error)
}

// DocumentExportResult adalah berkas hasil ekspor beserta nama yang disarankan.
//
// Isinya berupa byte di memori, bukan pembaca. Dokumen desain berukuran halaman
// dan seluruhnya sudah berada di memori saat digambar, jadi mengalirkannya tidak
// menghemat apa pun sementara byte utuh membuat panjangnya dapat disebutkan pada
// response — yang membuat browser dapat menampilkan kemajuan unduhan.
type DocumentExportResult struct {
	Filename string
	Content  []byte
	// Version adalah revisi isi yang dipakai saat menggambar, untuk dicatat
	// pemanggil. Dengan begitu keluhan "hasil cetaknya tidak sesuai" dapat
	// ditelusuri ke keadaan dokumen yang mana.
	Version int64
}
