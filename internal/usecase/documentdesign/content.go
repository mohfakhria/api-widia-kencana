package documentdesign

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
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
