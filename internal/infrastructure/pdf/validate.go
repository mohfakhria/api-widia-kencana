package pdf

import (
	"fmt"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"

	"github.com/go-pdf/fpdf"
)

// FontValidator memeriksa berkas font dengan cara yang paling jujur: mencoba
// menyematkannya ke dokumen PDF sungguhan.
//
// Bukan memeriksa magic byte atau ekstensi. Yang ingin dijawab bukan "apakah ini
// tampak seperti TTF" melainkan "apakah pustaka yang kelak menggambar dapat
// memakainya" — dan satu-satunya yang tahu jawabannya adalah pustaka itu
// sendiri.
type FontValidator struct{}

func NewFontValidator() FontValidator { return FontValidator{} }

func (FontValidator) Validate(data []byte) error {
	if len(data) == 0 {
		return domain.NewError(domain.ErrInvalidInput, "font file is empty")
	}

	// Dokumen sekali pakai, dibuang segera. Ongkosnya sepele dibanding satu
	// berkas rusak yang lolos ke penyimpanan.
	doc := fpdf.New("P", "pt", "A4", "")
	doc.AddUTF8FontFromBytes("probe", "", data)
	if err := doc.Error(); err != nil {
		return domain.NewError(domain.ErrInvalidInput,
			fmt.Sprintf("font file cannot be embedded into a PDF: %v", err))
	}

	return nil
}
