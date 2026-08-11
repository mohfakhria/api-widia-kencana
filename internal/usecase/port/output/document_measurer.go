package output

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// DocumentMeasurer menghitung tinggi elemen teks tanpa menggambar apa pun.
//
// Terpisah dari DocumentRenderer walau dilayani objek yang sama, karena
// pemanggilnya berbeda dan kebutuhannya berbeda: ekspor menghasilkan berkas,
// pengukuran menjawab pertanyaan. Antarmuka yang memuat keduanya memaksa
// pemakai salah satunya bergantung pada yang tidak ia butuhkan.
type DocumentMeasurer interface {
	MeasureText(ctx context.Context, elements []design.Element) ([]design.TextMeasurement, error)
}
