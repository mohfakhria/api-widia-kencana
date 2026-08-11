package input

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// DocumentMeasureUseCase menjawab setinggi apa elemen teks setelah dipenggal.
//
// Sengaja TIDAK menerima token dokumen. Pengukuran tidak menyentuh dokumen mana
// pun: yang dibutuhkannya hanya elemen beserta lebarnya, dan katalog font yang
// dipegang server. Menuntut dokumen berarti penyusun harus membuat dokumen lebih
// dulu sebelum boleh bertanya berapa tinggi paragrafnya — padahal urutan yang
// wajar justru sebaliknya.
type DocumentMeasureUseCase interface {
	MeasureText(ctx context.Context, elements []design.Element) ([]design.TextMeasurement, error)
}
