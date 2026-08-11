package usecase

import (
	"context"
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

const (
	// measureBatchLimit membatasi berapa elemen yang boleh diukur sekali panggil.
	//
	// Penyusun yang mengirim sepuluh ribu elemen hampir pasti sedang keliru, dan
	// menolaknya lebih menolong daripada melayaninya perlahan.
	//
	// Angkanya berdiri sendiri, tidak dipinjam dari batas halaman. Keduanya
	// kebetulan sama hari ini, tetapi menautkannya berarti menaikkan batas halaman
	// diam-diam mengubah batas pengukuran juga.
	measureBatchLimit = 200

	// measureTextLimit membatasi TOTAL panjang teks dalam satu permintaan.
	//
	// Ini penjaga biaya, bukan penjaga bentuk, dan alasannya terukur. Pemenggalan
	// baris mengukur ulang seluruh baris berjalan setiap kali satu kata
	// ditambahkan, sehingga ongkosnya kuadratik terhadap JUMLAH KATA PER BARIS —
	// dan jumlah itu ditentukan lebar kotak, yang boleh mencapai batas koordinat.
	//
	// Terukur pada kotak selebar 100000 pt, satu paragraf:
	//
	//	976 KB → 5,87 dtk      8 KB →  19 ms
	//	 64 KB →  392 ms      16 KB →  79 ms
	//
	// Pesan sebesar itu murah dikirim dan dapat diulang terus-menerus, sedangkan
	// pengukurannya dikerjakan di goroutine koneksi pengirimnya sendiri. Batas
	// ukuran satu pesan WebSocket (1 MB) TIDAK cukup menjaganya: satu pesan yang
	// muat di dalamnya sudah memadai untuk membakar detik-detik CPU.
	//
	// 16 KB dipilih karena pemakaian yang wajar tidak mendekatinya — satu halaman
	// penuh teks padat sekitar 3 KB, dan terukur 178 µs pada lebar kotak yang
	// masuk akal. Yang melampauinya memecah permintaannya menjadi beberapa, dan
	// pesan galatnya menyebutkan itu.
	measureTextLimit = 16 << 10
)

type documentMeasureUseCase struct {
	measurer output.DocumentMeasurer
}

func NewDocumentMeasureUseCase(measurer output.DocumentMeasurer) input.DocumentMeasureUseCase {
	return &documentMeasureUseCase{measurer: measurer}
}

// MeasureText memeriksa yang khas pengukuran, lalu menyerahkannya ke pengukur.
//
// Keabsahan elemen ITU SENDIRI sudah dijamin pemanggil: lapisan delivery
// menguraikannya lewat design.DecodeElement, pintu yang sama yang dipakai
// element.create, sehingga model tertutup dan seluruh aturan nilainya berlaku
// sama persis. Mengulangnya di sini berarti dua tempat yang harus sepakat.
func (uc *documentMeasureUseCase) MeasureText(ctx context.Context, elements []design.Element) ([]design.TextMeasurement, error) {
	if len(elements) == 0 {
		return nil, domain.NewError(domain.ErrInvalidInput, "no elements to measure")
	}
	if len(elements) > measureBatchLimit {
		return nil, domain.NewError(domain.ErrInvalidInput, fmt.Sprintf(
			"too many elements to measure: %d, limit is %d", len(elements), measureBatchLimit))
	}

	total := 0
	for index := range elements {
		element := &elements[index]
		total += len(element.Text)
		if total > measureTextLimit {
			return nil, domain.NewError(domain.ErrInvalidInput, fmt.Sprintf(
				"too much text to measure: over %d bytes in one request", measureTextLimit))
		}
		if element.Type != design.ElementText {
			return nil, domain.NewError(domain.ErrInvalidInput, fmt.Sprintf(
				"element %q has type %q, only text can be measured", element.ID, element.Type))
		}
		// Lebar nol membuat pemenggalan tidak punya arti: setiap kata melampaui
		// batasnya, dan hasilnya satu baris per kata. Ditolak alih-alih dijawab
		// dengan angka yang menyesatkan.
		if element.W <= 0 {
			return nil, domain.NewError(domain.ErrInvalidInput, fmt.Sprintf(
				"element %q must have w greater than zero to be measured", element.ID))
		}
	}

	return uc.measurer.MeasureText(ctx, elements)
}
