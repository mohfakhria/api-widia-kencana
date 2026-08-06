package output

import "github.com/mohfakhria/api-widia-kencana/internal/domain/design"

// FontCatalog menyebut font yang dapat dipakai isi dokumen.
//
// Tidak memakai context karena isinya dimuat sekali saat aplikasi start dan tidak
// pernah berubah setelahnya — tidak ada I/O, tidak ada yang dapat dibatalkan.
type FontCatalog interface {
	Catalog() []design.FontFamily
}
