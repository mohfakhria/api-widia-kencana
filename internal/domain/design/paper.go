package design

// Faktor pengali dari tiap satuan kertas ke titik. Titik adalah 1/72 inci, dan
// piksel CSS adalah 1/96 inci — karena itu px bernilai 0.75, bukan 1.
var paperUnitToPoints = map[string]float64{
	"pt": 1,
	"in": 72,
	"mm": 72.0 / 25.4,
	"cm": 72.0 / 2.54,
	"m":  7200.0 / 2.54,
	"px": 0.75,
}

// PaperPoints mengubah ukuran kertas ke titik.
//
// Tabel document_papers menyimpan ukuran dalam satuan yang berbeda-beda — A4
// dalam milimeter, Letter dalam inci — sedangkan seluruh model isi dokumen dan
// PDF bekerja dalam titik. Konversi terjadi sekali di sini, di batas antara
// keduanya, sehingga tidak ada satu pun tempat lain yang perlu tahu satuan asli
// kertas.
func PaperPoints(width, height float64, unit string) (widthPt, heightPt float64, ok bool) {
	factor, ok := paperUnitToPoints[unit]
	if !ok {
		return 0, 0, false
	}

	return width * factor, height * factor, true
}
