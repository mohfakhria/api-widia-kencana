package documentdesign

import (
	"math"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"

	"github.com/google/uuid"
)

// Tata letak benih dinyatakan sebagai bagian dari ukuran kertas, bukan angka
// tetap, supaya hasilnya seimbang pada kertas apa pun — dari A4 sampai struk
// termal selebar 58 mm.
const (
	seedSideMarginRatio = 0.10
	seedTopMarginRatio  = 0.08
)

// Ukuran huruf benih dinyatakan dalam titik dan tidak diturunkan dari ukuran
// kertas. Ukuran huruf memang angka mutlak: 11 pt tetap 11 pt, di kertas apa pun,
// karena yang menentukan keterbacaan adalah jarak baca mata, bukan lebar kertas.
const (
	seedTitleSize   = 24.0
	seedLeadSize    = 11.0
	seedHeadingSize = 13.0
	seedBodySize    = 10.5
)

// Tinggi baris dituliskan tegas, tidak dibiarkan memakai nilai bawaan. Benih ini
// menjadi rujukan pertama frontend saat menyelaraskan tampilannya, jadi ia harus
// menyebut seluruh properti yang menentukan tata letaknya.
const (
	seedHeadingLineHeight = 1.3
	seedBodyLineHeight    = 1.5
)

const seedInkColor = "#111827"

// seedBlock adalah satu blok teks pada dokumen benih.
//
// Tinggi dan jarak dinyatakan sebagai bagian dari tinggi kertas, sehingga irama
// vertikalnya tetap sama pada kertas mana pun.
type seedBlock struct {
	text        string
	fontSize    float64
	fontWeight  int
	lineHeight  float64
	heightRatio float64
	gapRatio    float64
}

// seedBlocks adalah panduan singkat yang muncul pada dokumen yang masih kosong.
//
// Isinya sengaja memperagakan kaidah yang dijelaskannya: tiga tingkat hierarki,
// perataan seragam, dan seperlima halaman dibiarkan kosong di bawah.
func seedBlocks() []seedBlock {
	return []seedBlock{
		{
			text:        "Kaidah Dokumen yang Baik",
			fontSize:    seedTitleSize,
			fontWeight:  700,
			lineHeight:  seedHeadingLineHeight,
			heightRatio: 0.06,
			gapRatio:    0.02,
		},
		{
			text:        "Halaman ini adalah panduan bawaan. Ubah atau hapus isinya kapan saja — ia hanya muncul pada dokumen yang masih kosong.",
			fontSize:    seedLeadSize,
			fontWeight:  400,
			lineHeight:  seedBodyLineHeight,
			heightRatio: 0.07,
			gapRatio:    0.05,
		},
		{
			text:        "1. Hierarki",
			fontSize:    seedHeadingSize,
			fontWeight:  700,
			lineHeight:  seedHeadingLineHeight,
			heightRatio: 0.04,
			gapRatio:    0.01,
		},
		{
			text:        "Ukuran dan ketebalan huruf menuntun mata pembaca. Judul paling besar, subjudul lebih kecil, isi paling kecil. Tiga tingkat sudah cukup; lebih dari itu justru membingungkan.",
			fontSize:    seedBodySize,
			fontWeight:  400,
			lineHeight:  seedBodyLineHeight,
			heightRatio: 0.09,
			gapRatio:    0.05,
		},
		{
			text:        "2. Ruang Kosong",
			fontSize:    seedHeadingSize,
			fontWeight:  700,
			lineHeight:  seedHeadingLineHeight,
			heightRatio: 0.04,
			gapRatio:    0.01,
		},
		{
			text:        "Jangan penuhi seluruh halaman. Ruang kosong memberi napas dan memisahkan gagasan. Perhatikan sisa ruang di bawah halaman ini — itu disengaja.",
			fontSize:    seedBodySize,
			fontWeight:  400,
			lineHeight:  seedBodyLineHeight,
			heightRatio: 0.09,
			gapRatio:    0.05,
		},
		{
			text:        "3. Konsistensi",
			fontSize:    seedHeadingSize,
			fontWeight:  700,
			lineHeight:  seedHeadingLineHeight,
			heightRatio: 0.04,
			gapRatio:    0.01,
		},
		{
			text:        "Satu jenis huruf sudah cukup. Samakan perataan, jarak antarblok, dan ukuran pada elemen yang sederajat. Keseragaman membuat dokumen terasa rapi tanpa usaha tambahan.",
			fontSize:    seedBodySize,
			fontWeight:  400,
			lineHeight:  seedBodyLineHeight,
			heightRatio: 0.09,
		},
	}
}

// defaultDocumentContent menyusun satu halaman panduan untuk dokumen yang masih
// kosong.
//
// Ukuran kertas dikonversi ke titik lebih dulu, karena seluruh koordinat dalam
// model isi dokumen memakai titik. Satuan asli kertas — milimeter untuk A4, inci
// untuk Letter — berhenti di sini dan tidak pernah masuk ke isi dokumen.
//
// Kertas dengan satuan yang tidak dikenal menghasilkan dokumen tanpa halaman,
// bukan benih dengan koordinat yang salah. Kanvas kosong jelas terlihat keliru
// dan mudah dilaporkan; benih yang ukurannya meleset diam-diam justru akan
// dikira memang begitu bentuknya.
func defaultDocumentContent(paper entity.DocumentPaperSize) *design.Content {
	width, height, ok := design.PaperPoints(paper.Width, paper.Height, paper.Unit)
	if !ok {
		return &design.Content{Pages: []design.Page{}}
	}

	sideMargin := width * seedSideMarginRatio
	contentWidth := width - 2*sideMargin
	offsetY := height * seedTopMarginRatio

	blocks := seedBlocks()
	elements := make([]design.Element, 0, len(blocks))

	for _, block := range blocks {
		blockHeight := height * block.heightRatio

		elements = append(elements, design.Element{
			ID:         uuid.NewString(),
			Type:       design.ElementText,
			X:          round3(sideMargin),
			Y:          round3(offsetY),
			W:          round3(contentWidth),
			H:          round3(blockHeight),
			Text:       block.text,
			FontFamily: design.DefaultFontFamily,
			FontSize:   block.fontSize,
			FontWeight: block.fontWeight,
			FontStyle:  design.FontStyleNormal,
			Color:      seedInkColor,
			Align:      design.AlignLeft,
			LineHeight: block.lineHeight,
		})

		offsetY += blockHeight + height*block.gapRatio
	}

	return &design.Content{
		Pages: []design.Page{{
			ID:       uuid.NewString(),
			Elements: elements,
		}},
	}
}

// round3 membulatkan ke tiga desimal supaya penumpukan pecahan biner tidak
// menghasilkan koordinat seperti 23.760000000000002 di dalam JSON yang tersimpan.
// Tiga desimal titik setara sekitar satu per seribu milimeter — jauh lebih halus
// daripada yang dapat dicetak printer mana pun.
func round3(value float64) float64 {
	return math.Round(value*1000) / 1000
}
