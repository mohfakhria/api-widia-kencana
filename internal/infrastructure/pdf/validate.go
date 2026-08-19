package pdf

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/sfnt"
)

// FontInspector membaca jati diri font dari berkasnya, lalu membuktikan berkas
// itu benar-benar dapat disematkan ke PDF.
//
// Pembuktiannya bukan memeriksa magic byte melainkan MENYEMATKANNYA ke dokumen
// sekali pakai. Yang ingin dijawab bukan "apakah ini tampak seperti TTF"
// melainkan "apakah pustaka yang kelak menggambar dapat memakainya" — dan
// satu-satunya yang tahu jawabannya adalah pustaka itu sendiri. Berkas rusak
// yang lolos akan merusak SETIAP ekspor sesudahnya, jauh dari sebabnya.
type FontInspector struct{}

func NewFontInspector() FontInspector { return FontInspector{} }

func (FontInspector) Inspect(data []byte) (output.FontIdentity, error) {
	if len(data) == 0 {
		return output.FontIdentity{}, domain.NewError(domain.ErrInvalidInput, "font file is empty")
	}

	// Diperiksa SEBELUM diurai, dan dari tabelnya sendiri. Font variabel memuat
	// seluruh rentang bobot dalam satu berkas; subfamily-nya tetap menyebut satu
	// nama — Archivo-VariableFont menyebut "SemiBold" — sehingga bila lolos ia
	// tersimpan sebagai 600 lalu MENIMPA SemiBold statik yang sungguhan, dan
	// yang tergambar sesudahnya instance bawaannya. Kegagalan itu tidak
	// menghasilkan galat apa pun, hanya bobot yang salah di setiap ekspor.
	if hasVariations(data) {
		return output.FontIdentity{}, domain.NewError(domain.ErrInvalidInput,
			"variable font has no single weight; use the static files")
	}

	font, err := sfnt.Parse(data)
	if err != nil {
		return output.FontIdentity{}, domain.NewError(domain.ErrInvalidInput,
			fmt.Sprintf("font file could not be parsed: %v", err))
	}

	var buffer sfnt.Buffer

	// Nama TIPOGRAFIS didahulukan. Pada keluarga berbobot banyak, NameIDFamily
	// dipecah demi keterbatasan lama — "Barlow SemiBold" menjadi keluarga
	// tersendiri dengan subfamily "Regular" — sedangkan NameIDTypographicFamily
	// tetap berbunyi "Barlow" dengan subfamily "SemiBold". Yang pertama akan
	// menghasilkan satu keluarga per bobot di daftar font.
	family := name(font, &buffer, sfnt.NameIDTypographicFamily, sfnt.NameIDFamily)
	if family == "" {
		return output.FontIdentity{}, domain.NewError(domain.ErrInvalidInput,
			"font file has no family name")
	}

	sub := name(font, &buffer, sfnt.NameIDTypographicSubfamily, sfnt.NameIDSubfamily)
	weight, style := parseSubfamily(sub)

	// Setelah diketahui jati dirinya, barulah dibuktikan dapat digambar.
	doc := fpdf.New("P", "pt", "A4", "")
	doc.AddUTF8FontFromBytes("probe", "", data)
	if err := doc.Error(); err != nil {
		return output.FontIdentity{}, domain.NewError(domain.ErrInvalidInput,
			fmt.Sprintf("font file cannot be embedded into a PDF: %v", err))
	}

	return output.FontIdentity{Family: family, Weight: weight, Style: style}, nil
}

// hasVariations menjawab apakah berkas memuat tabel fvar.
//
// Direktori tabel dibaca langsung, bukan lewat sfnt: pustaka itu tidak membuka
// fvar sama sekali. Bentuknya tetap sejak awal format — sfntVersion, jumlah
// tabel di offset 4, lalu catatan 16 byte yang diawali tag empat huruf.
//
// Alternatifnya menebak dari NAMA berkas, dan itu justru yang hendak dihindari
// seluruh rancangan ini: nama berkas bukan fakta tentang isinya.
func hasVariations(data []byte) bool {
	const (
		awalDirektori = 12
		ukuranCatatan = 16
	)

	if len(data) < awalDirektori {
		return false
	}

	jumlah := int(binary.BigEndian.Uint16(data[4:6]))
	for index := range jumlah {
		awal := awalDirektori + index*ukuranCatatan
		if awal+4 > len(data) {
			// Direktori yang terpotong bukan urusan di sini — sfnt.Parse yang
			// akan menolaknya, dengan pesan yang menyebut sebabnya.
			return false
		}

		if string(data[awal:awal+4]) == "fvar" {
			return true
		}
	}

	return false
}

// name mengembalikan entri pertama yang terisi di antara id yang diberikan.
func name(font *sfnt.Font, buffer *sfnt.Buffer, ids ...sfnt.NameID) string {
	for _, id := range ids {
		if value, err := font.Name(buffer, id); err == nil {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}

	return ""
}

// weightNames memetakan nama subfamily ke bobot numerik.
//
// Dari subfamily, bukan dari OS/2: pustaka sfnt tidak membuka usWeightClass, dan
// subfamily adalah nama yang sama dengan yang dipakai berkasnya sendiri. Yang
// tidak dikenal jatuh ke 400 — bukan ditolak, karena subfamily bebas bentuk dan
// menolak font yang sah hanya karena namanya tidak lazim jauh lebih merugikan
// daripada bobot yang meleset satu tingkat.
var weightNames = map[string]int{
	"thin":       100,
	"hairline":   100,
	"extralight": 200,
	"ultralight": 200,
	"light":      300,
	"regular":    400,
	"normal":     400,
	"book":       400,
	"medium":     500,
	"semibold":   600,
	"demibold":   600,
	"bold":       700,
	"extrabold":  800,
	"ultrabold":  800,
	"black":      900,
	"heavy":      900,
}

// parseSubfamily membaca "SemiBold Italic" menjadi 600 dan italic.
func parseSubfamily(sub string) (weight int, style string) {
	normalized := strings.ToLower(strings.NewReplacer(" ", "", "-", "", "_", "").Replace(sub))

	style = design.FontStyleNormal
	for _, penanda := range []string{"italic", "oblique"} {
		if strings.Contains(normalized, penanda) {
			style = design.FontStyleItalic
			normalized = strings.ReplaceAll(normalized, penanda, "")

			break
		}
	}

	if normalized == "" {
		return 400, style
	}
	if weight, ok := weightNames[normalized]; ok {
		return weight, style
	}

	return 400, style
}
