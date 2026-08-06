package documentdesign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"

	"github.com/google/uuid"
)

// Kunci yang dikenal backend. Selebihnya diperlakukan sebagai data yang lewat
// begitu saja.
const (
	contentKeyPages    = "pages"
	contentKeyElements = "elements"
	contentKeyID       = "id"
	contentKeyType     = "type"
)

// documentContent adalah isi kanvas satu dokumen, disimpan sebagai struktur
// generik apa adanya.
//
// Sengaja bukan struct bertipe. Struct akan membuang field yang tidak dikenalnya
// saat round-trip: frontend menambah satu properti visual baru di luar props,
// backend memuat lalu menyimpannya lagi, dan properti itu lenyap tanpa error
// maupun log. Peta generik membuat sifat "backend tidak memahami model elemen"
// berlaku secara struktural, bukan bergantung pada disiplin.
//
// Penerapan patch nanti juga jatuh langsung dari bentuk ini: menggabungkan patch
// parsial ke dalam peta adalah persis aturan penulis-terakhir-menang per
// properti, tanpa perlu mengenumerasi properti apa pun.
type documentContent struct {
	root map[string]any
}

func emptyDocumentContent() *documentContent {
	return &documentContent{
		root: map[string]any{contentKeyPages: []any{}},
	}
}

// isEmpty menandai dokumen yang belum punya halaman sama sekali — kandidat untuk
// diisi benih.
func (c *documentContent) isEmpty() bool {
	pages, ok := c.root[contentKeyPages].([]any)

	return ok && len(pages) == 0
}

// Tata letak benih dinyatakan sebagai bagian dari ukuran kertas, bukan angka
// tetap, supaya hasilnya seimbang pada kertas apa pun.
const (
	seedSideMarginRatio = 0.10
	seedTopMarginRatio  = 0.08
)

// seedBlock adalah satu blok teks pada dokumen benih.
//
// Tinggi dan jarak dinyatakan sebagai bagian dari tinggi kertas; ukuran huruf
// tidak, karena ukuran huruf lazimnya angka mutlak dan bukan turunan ukuran
// kertas.
type seedBlock struct {
	text        string
	fontSize    float64
	fontWeight  string
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
			fontSize:    24,
			fontWeight:  "bold",
			heightRatio: 0.06,
			gapRatio:    0.02,
		},
		{
			text:        "Halaman ini adalah panduan bawaan. Ubah atau hapus isinya kapan saja — ia hanya muncul pada dokumen yang masih kosong.",
			fontSize:    11,
			fontWeight:  "regular",
			heightRatio: 0.07,
			gapRatio:    0.05,
		},
		{
			text:        "1. Hierarki",
			fontSize:    13,
			fontWeight:  "bold",
			heightRatio: 0.04,
			gapRatio:    0.01,
		},
		{
			text:        "Ukuran dan ketebalan huruf menuntun mata pembaca. Judul paling besar, subjudul lebih kecil, isi paling kecil. Tiga tingkat sudah cukup; lebih dari itu justru membingungkan.",
			fontSize:    10.5,
			fontWeight:  "regular",
			heightRatio: 0.09,
			gapRatio:    0.05,
		},
		{
			text:        "2. Ruang Kosong",
			fontSize:    13,
			fontWeight:  "bold",
			heightRatio: 0.04,
			gapRatio:    0.01,
		},
		{
			text:        "Jangan penuhi seluruh halaman. Ruang kosong memberi napas dan memisahkan gagasan. Perhatikan sisa ruang di bawah halaman ini — itu disengaja.",
			fontSize:    10.5,
			fontWeight:  "regular",
			heightRatio: 0.09,
			gapRatio:    0.05,
		},
		{
			text:        "3. Konsistensi",
			fontSize:    13,
			fontWeight:  "bold",
			heightRatio: 0.04,
			gapRatio:    0.01,
		},
		{
			text:        "Satu jenis huruf sudah cukup. Samakan perataan, jarak antarblok, dan ukuran pada elemen yang sederajat. Keseragaman membuat dokumen terasa rapi tanpa usaha tambahan.",
			fontSize:    10.5,
			fontWeight:  "regular",
			heightRatio: 0.09,
		},
	}
}

// defaultDocumentContent menyusun satu halaman panduan untuk dokumen yang masih
// kosong.
//
// Koordinat dan ukuran memakai satuan kertas itu sendiri — itu satu-satunya
// pilihan yang konsisten tanpa mengarang konversi, karena backend tidak tahu
// skala yang dipakai kanvas frontend.
func defaultDocumentContent(paper entity.DocumentPaperSize) *documentContent {
	sideMargin := paper.Width * seedSideMarginRatio
	contentWidth := paper.Width - 2*sideMargin
	offsetY := paper.Height * seedTopMarginRatio

	blocks := seedBlocks()
	elements := make([]any, 0, len(blocks))

	for _, block := range blocks {
		height := paper.Height * block.heightRatio

		elements = append(elements, map[string]any{
			contentKeyID:   uuid.NewString(),
			contentKeyType: "text",
			"x":            number(sideMargin),
			"y":            number(offsetY),
			"w":            number(contentWidth),
			"h":            number(height),
			"props": map[string]any{
				"text":       block.text,
				"fontSize":   number(block.fontSize),
				"fontWeight": block.fontWeight,
				"align":      "left",
			},
		})

		offsetY += height + paper.Height*block.gapRatio
	}

	page := map[string]any{
		contentKeyID:       uuid.NewString(),
		contentKeyElements: elements,
	}

	return &documentContent{
		root: map[string]any{contentKeyPages: []any{page}},
	}
}

// number menghasilkan literal angka yang sama bentuknya dengan hasil penguraian
// UseNumber, sehingga benih dan isi yang dimuat dari database diperlakukan sama.
//
// Dibulatkan ke tiga desimal — presisi yang sama dengan ukuran kertas di
// database — supaya penumpukan pecahan biner tidak menghasilkan literal seperti
// 23.760000000000002. FormatFloat dengan presisi -1 lalu memberi bentuk
// terpendek yang tetap bulat-balik: 105 tetap "105", bukan "105.000".
func number(value float64) json.Number {
	rounded := math.Round(value*1000) / 1000

	return json.Number(strconv.FormatFloat(rounded, 'f', -1, 64))
}

// parseDocumentContent mengurai isi dokumen dan memvalidasi rangkanya.
//
// UseNumber menyimpan angka sebagai literal aslinya, bukan float64, sehingga
// 0.50 kembali sebagai 0.50 dan bukan 0.5. Tanpa itu setiap round-trip diam-diam
// menulis ulang angka pengguna.
func parseDocumentContent(raw json.RawMessage) (*documentContent, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return emptyDocumentContent(), nil
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()

	var root map[string]any
	if err := decoder.Decode(&root); err != nil {
		return nil, domain.NewError(domain.ErrInvalidInput, "document content is not a JSON object")
	}

	if err := validateDocumentContent(root); err != nil {
		return nil, err
	}

	return &documentContent{root: root}, nil
}

// encode mengubah isi kembali menjadi JSON.
//
// Urutan kunci akan tersusun menurut abjad karena Go mengurutkan kunci peta saat
// marshal. Itu tidak berpengaruh: urutan kunci JSON tidak bermakna, dan Postgres
// pun menormalkannya sendiri saat menyimpan sebagai jsonb.
func (c *documentContent) encode() (json.RawMessage, error) {
	encoded, err := json.Marshal(c.root)
	if err != nil {
		return nil, fmt.Errorf("encode document content: %w", err)
	}

	return encoded, nil
}

// validateDocumentContent memeriksa rangkanya saja — yang dibutuhkan backend
// untuk menelusuri dan menerapkan patch. Isi di luar itu tidak disentuh.
//
// Yang dijamin setelah ini lolos: pages adalah array berisi objek, tiap halaman
// dan tiap elemen punya id tak kosong, tiap elemen punya type tak kosong, dan
// seluruh id unik dalam satu dokumen. Penelusuran berdasarkan id karenanya tidak
// pernah ambigu.
func validateDocumentContent(root map[string]any) error {
	pagesValue, ok := root[contentKeyPages]
	if !ok {
		return domain.NewError(domain.ErrInvalidInput, "document content must have a pages array")
	}

	pages, ok := pagesValue.([]any)
	if !ok {
		return domain.NewError(domain.ErrInvalidInput, "document content pages must be an array")
	}

	seenPages := make(map[string]struct{}, len(pages))
	// Id elemen dijaga unik lintas halaman, bukan hanya di dalam halamannya.
	// Dengan begitu elemen dapat berpindah halaman tanpa risiko bentrok id.
	seenElements := make(map[string]struct{})

	for index, pageValue := range pages {
		page, ok := pageValue.(map[string]any)
		if !ok {
			return invalidContent("page %d must be an object", index)
		}

		pageID, ok := nonEmptyString(page, contentKeyID)
		if !ok {
			return invalidContent("page %d must have a non-empty id", index)
		}
		if _, exists := seenPages[pageID]; exists {
			return invalidContent("duplicate page id %q", pageID)
		}
		seenPages[pageID] = struct{}{}

		if err := validatePageElements(page, pageID, seenElements); err != nil {
			return err
		}
	}

	return nil
}

func validatePageElements(page map[string]any, pageID string, seenElements map[string]struct{}) error {
	elementsValue, ok := page[contentKeyElements]
	if !ok {
		// Halaman tanpa elemen sah: kanvas yang masih kosong.
		return nil
	}

	elements, ok := elementsValue.([]any)
	if !ok {
		return invalidContent("elements of page %q must be an array", pageID)
	}

	for index, elementValue := range elements {
		element, ok := elementValue.(map[string]any)
		if !ok {
			return invalidContent("element %d of page %q must be an object", index, pageID)
		}

		elementID, ok := nonEmptyString(element, contentKeyID)
		if !ok {
			return invalidContent("element %d of page %q must have a non-empty id", index, pageID)
		}
		if _, exists := seenElements[elementID]; exists {
			return invalidContent("duplicate element id %q", elementID)
		}
		seenElements[elementID] = struct{}{}

		if _, ok := nonEmptyString(element, contentKeyType); !ok {
			return invalidContent("element %q must have a non-empty type", elementID)
		}
	}

	return nil
}

// nonEmptyString membaca field bertipe string yang tidak kosong. Selalu bentuk
// comma-ok: struktur generik berarti tidak ada jaminan tipe dari kompilator, dan
// assertion telanjang akan berubah jadi panic pada masukan yang cacat.
func nonEmptyString(object map[string]any, key string) (string, bool) {
	value, ok := object[key].(string)
	if !ok || value == "" {
		return "", false
	}

	return value, true
}

func invalidContent(format string, args ...any) error {
	return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf(format, args...))
}
