package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
)

// DocumentContentSource memberikan isi dokumen yang paling mutakhir.
//
// Ada terpisah dari DocumentRepository karena keduanya menjawab pertanyaan yang
// berbeda. Repository menjawab "apa yang tersimpan"; sumber ini menjawab "apa
// yang sedang dilihat pengguna". Selama dokumen dibuka, keduanya dapat berbeda
// sampai satu denyut penyimpanan penuh — dan ekspor harus mengikuti yang kedua,
// kalau tidak pengguna dapat menggeser sebuah elemen lalu menerima PDF yang belum
// memuat geseran itu.
type DocumentContentSource interface {
	Snapshot(ctx context.Context, documentToken string) (*entity.DocumentContent, error)
}
