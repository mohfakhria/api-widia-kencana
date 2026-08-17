package design

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
)

// coordinateLimit membatasi seberapa jauh sebuah elemen boleh ditempatkan atau
// seberapa besar ia boleh dibuat, dalam titik. Seratus ribu titik setara sekitar
// 35 meter — jauh melampaui kertas apa pun, tetapi tetap menutup nilai liar yang
// membuat renderer menghabiskan waktu menggambar sesuatu yang tak akan terlihat.
const coordinateLimit = 100_000

// Validate memeriksa seluruh isi dokumen.
//
// Yang dijamin setelah ini lolos: setiap halaman dan elemen punya id tak kosong
// dan unik, jenis elemen dikenal, seluruh angka terhingga dan dalam batas wajar,
// dan setiap nilai enumerasi berada dalam himpunannya. Renderer karenanya tidak
// perlu memeriksa apa pun lagi selain ketersediaan font dan aset.
func (c *Content) Validate() error {
	seenPages := make(map[string]struct{}, len(c.Pages))
	// Id elemen dijaga unik lintas halaman, bukan hanya di dalam halamannya.
	// Dengan begitu elemen dapat berpindah halaman tanpa risiko bentrok id.
	seenElements := make(map[string]struct{})

	for index, page := range c.Pages {
		if page.ID == "" {
			return invalidf("page %d must have a non-empty id", index)
		}
		if _, exists := seenPages[page.ID]; exists {
			return invalidf("duplicate page id %q", page.ID)
		}
		seenPages[page.ID] = struct{}{}

		for elementIndex, element := range page.Elements {
			if element.ID == "" {
				return invalidf("element %d of page %q must have a non-empty id", elementIndex, page.ID)
			}
			if _, exists := seenElements[element.ID]; exists {
				return invalidf("duplicate element id %q", element.ID)
			}
			seenElements[element.ID] = struct{}{}

			if err := element.validate(); err != nil {
				return err
			}
		}

		if !IsColor(page.Background) {
			return invalidf("page %q has background %q, expected #rgb or #rrggbb",
				page.ID, page.Background)
		}
	}

	// Elemen master ikut diperiksa, dan keunikannya berbagi peta yang sama dengan
	// elemen halaman: elemen dicari lewat id saja, jadi id yang sama di master dan
	// di sebuah halaman membuat update maupun delete menyasar salah satunya
	// secara acak.
	for index, element := range c.Master.Elements {
		if element.ID == "" {
			return invalidf("master element %d must have a non-empty id", index)
		}
		if _, exists := seenElements[element.ID]; exists {
			return invalidf("duplicate element id %q", element.ID)
		}
		seenElements[element.ID] = struct{}{}

		if err := element.validate(); err != nil {
			return err
		}
	}

	// Guide diperiksa di sini juga, bukan hanya saat dibuat: isi yang dimuat dari
	// database melewati jalur ini, dan baris yang cacat di sana harus ketahuan
	// saat room dibuka — bukan belakangan sebagai guide yang menempel di tempat
	// yang mustahil.
	seenGuides := make(map[string]struct{}, len(c.Guides))
	for index, guide := range c.Guides {
		if guide.ID == "" {
			return invalidf("guide %d must have a non-empty id", index)
		}
		if _, exists := seenGuides[guide.ID]; exists {
			return invalidf("duplicate guide id %q", guide.ID)
		}
		seenGuides[guide.ID] = struct{}{}

		if err := guide.validate(); err != nil {
			return err
		}
	}

	return nil
}

// validate memeriksa satu guide.
//
// Axis TIDAK ditafsirkan, hanya dicocokkan dengan dua nilai yang sah. Nilai di
// luar keduanya DITOLAK, bukan diberi bawaan: seluruh guna sebuah guide adalah
// untuk dipercaya ketika sesuatu diluruskan padanya, dan guide yang diam-diam
// dipindahkan ke sumbu lain lebih buruk daripada guide yang tidak pernah ada.
func (g Guide) validate() error {
	if g.Axis != GuideAxisX && g.Axis != GuideAxisY {
		return invalidf("guide %q has axis %q, expected one of %s, %s",
			g.ID, g.Axis, GuideAxisX, GuideAxisY)
	}

	return finite(g.ID, "position", g.Position)
}

// validate memeriksa keempat koordinat satu per satu, tidak lewat peta.
//
// Iterasi peta di Go berurutan acak, sehingga elemen dengan dua koordinat cacat
// akan menghasilkan pesan yang berbeda-beda tiap kali dijalankan. Pesan yang
// tidak dapat diulang membuat laporan dari pengguna sulit dicocokkan dengan
// penyebabnya.
func (e *Element) validate() error {
	if err := finite(e.ID, "x", e.X); err != nil {
		return err
	}
	if err := finite(e.ID, "y", e.Y); err != nil {
		return err
	}
	if err := finite(e.ID, "w", e.W); err != nil {
		return err
	}
	if err := finite(e.ID, "h", e.H); err != nil {
		return err
	}

	// Rotation dan opacity berlaku untuk SEMUA jenis, jadi diperiksa di sini
	// alih-alih diulang di keempat pemeriksa di bawah.
	if err := finite(e.ID, "rotation", e.Rotation); err != nil {
		return err
	}
	if e.Opacity != nil {
		if err := finite(e.ID, "opacity", *e.Opacity); err != nil {
			return err
		}
		if *e.Opacity < 0 || *e.Opacity > 1 {
			return invalidf("element %q has opacity %v, expected between 0 and 1", e.ID, *e.Opacity)
		}
	}

	switch e.Type {
	case ElementText:
		return e.validateText()
	case ElementRect:
		return e.validateRect()
	case ElementEllipse:
		return e.validateEllipse()
	case ElementLine:
		return e.validateLine()
	case ElementImage:
		return e.validateImage()
	case ElementTable:
		return e.validateTable()
	default:
		return invalidf("element %q has unknown type %q", e.ID, e.Type)
	}
}

// validateText tidak menolak teks kosong: blok yang baru dibuat memang belum
// berisi apa-apa, dan memaksanya terisi akan membuat frontend perlu mengarang
// teks sementara.
func (e *Element) validateText() error {
	if err := e.requireBox(); err != nil {
		return err
	}

	// Nol berarti properti tidak disebutkan dan nilai bawaan yang berlaku. Yang
	// ditolak hanyalah nilai negatif, yang selalu keliru dan bukan penghilangan.
	if e.FontSize < 0 {
		return invalidf("element %q has a negative fontSize", e.ID)
	}
	if err := finite(e.ID, "fontSize", e.FontSize); err != nil {
		return err
	}
	if e.LineHeight < 0 {
		return invalidf("element %q has a negative lineHeight", e.ID)
	}
	if err := finite(e.ID, "lineHeight", e.LineHeight); err != nil {
		return err
	}
	if err := finite(e.ID, "letterSpacing", e.LetterSpacing); err != nil {
		return err
	}

	if e.FontWeight != 0 && (e.FontWeight < 100 || e.FontWeight > 900 || e.FontWeight%100 != 0) {
		return invalidf("element %q has fontWeight %d, expected a multiple of 100 between 100 and 900", e.ID, e.FontWeight)
	}
	if err := oneOf(e.ID, "fontStyle", e.FontStyle, FontStyleNormal, FontStyleItalic); err != nil {
		return err
	}
	if err := oneOf(e.ID, "align", e.Align, AlignLeft, AlignCenter, AlignRight, AlignJustify); err != nil {
		return err
	}
	if err := oneOf(e.ID, "verticalAlign", e.VerticalAlign, VAlignTop, VAlignMiddle, VAlignBottom); err != nil {
		return err
	}
	if err := oneOf(e.ID, "format", e.Format, FormatPlain, FormatGrouped, FormatCurrency, FormatPercent); err != nil {
		return err
	}

	return color(e.ID, "color", e.Color)
}

// shareTolerance adalah kelonggaran saat memeriksa jumlah proporsi kolom.
//
// Jumlahnya seharusnya tepat satu, tetapi proporsi lahir dari pembagian di sisi
// klien dan melewati JSON sebagai float — enam kolom yang masing-masing
// sepertiga tidak akan pernah berjumlah persis 1. Menolak selisih sekecil itu
// berarti menolak tabel yang benar; menerima selisih besar berarti menerima
// tabel yang tidak memenuhi lebarnya.
const shareTolerance = 0.001

// validateTable memeriksa tabel beserta seluruh kolom, baris, dan selnya.
//
// Yang TIDAK diperiksa di sini: apakah tinggi baris cocok dengan teksnya. Tinggi
// datang dari pengukuran frontend dan sengaja dipakai apa adanya — memeriksanya
// berarti mengukur ulang, dan pengukuran kedua itulah yang justru menjadi sumber
// perselisihan yang hendak dihindari.
func (e *Element) validateTable() error {
	if err := e.requireBox(); err != nil {
		return err
	}
	if len(e.Columns) == 0 {
		return invalidf("table %q has no columns", e.ID)
	}

	total := 0.0
	for index, column := range e.Columns {
		if column.Share <= 0 {
			return invalidf("table %q column %d has share %v, expected greater than zero",
				e.ID, index, column.Share)
		}
		if err := finite(e.ID, "column share", column.Share); err != nil {
			return err
		}
		if err := oneOf(e.ID, "column align", column.Align,
			AlignLeft, AlignCenter, AlignRight, AlignJustify); err != nil {
			return err
		}
		total += column.Share
	}
	if math.Abs(total-1) > shareTolerance {
		return invalidf("table %q column shares add up to %v, expected 1", e.ID, total)
	}

	for index, row := range e.Rows {
		if row.Height < 0 {
			return invalidf("table %q row %d has a negative height", e.ID, index)
		}
		if err := finite(e.ID, "row height", row.Height); err != nil {
			return err
		}
		// Lebih PENDEK boleh — sel yang tidak ada digambar kosong. Lebih panjang
		// berarti model klien tidak sepakat dengan kolomnya sendiri, dan sel yang
		// tidak punya kolom tidak dapat digambar di mana pun.
		if len(row.Cells) > len(e.Columns) {
			return invalidf("table %q row %d has %d cells but only %d columns",
				e.ID, index, len(row.Cells), len(e.Columns))
		}

		for cellIndex, cell := range row.Cells {
			if cell.FontWeight != 0 &&
				(cell.FontWeight < 100 || cell.FontWeight > 900 || cell.FontWeight%100 != 0) {
				return invalidf("table %q row %d cell %d has fontWeight %d, expected a multiple of 100 between 100 and 900",
					e.ID, index, cellIndex, cell.FontWeight)
			}
			if err := oneOf(e.ID, "cell format", cell.Format,
				FormatPlain, FormatGrouped, FormatCurrency, FormatPercent); err != nil {
				return err
			}
		}
	}

	if e.BorderWidth < 0 {
		return invalidf("table %q has a negative borderWidth", e.ID)
	}
	if err := finite(e.ID, "borderWidth", e.BorderWidth); err != nil {
		return err
	}
	if e.CellPadding < 0 {
		return invalidf("table %q has a negative cellPadding", e.ID)
	}
	if err := finite(e.ID, "cellPadding", e.CellPadding); err != nil {
		return err
	}
	if e.FontSize < 0 {
		return invalidf("table %q has a negative fontSize", e.ID)
	}
	if err := finite(e.ID, "fontSize", e.FontSize); err != nil {
		return err
	}
	if e.LineHeight < 0 {
		return invalidf("table %q has a negative lineHeight", e.ID)
	}
	if err := finite(e.ID, "lineHeight", e.LineHeight); err != nil {
		return err
	}
	if err := oneOf(e.ID, "verticalAlign", e.VerticalAlign,
		VAlignTop, VAlignMiddle, VAlignBottom); err != nil {
		return err
	}

	// Kosong berarti TANPA ISIAN, bukan putih — konvensi yang sama dengan fill
	// pada rect dan background pada halaman, dan IsColor sudah menerimanya.
	for name, value := range map[string]string{
		"headerFill":  e.HeaderFill,
		"headerColor": e.HeaderColor,
		"bodyFill":    e.BodyFill,
		"stripeFill":  e.StripeFill,
		"borderColor": e.BorderColor,
		"textColor":   e.TextColor,
	} {
		if err := color(e.ID, name, value); err != nil {
			return err
		}
	}

	return nil
}

func (e *Element) validateRect() error {
	if err := e.validateRadius(); err != nil {
		return err
	}

	return e.validateShape()
}

// validateRadius dipakai rect dan gambar. Keduanya membulatkan sudut dengan
// aturan yang sama; yang berbeda hanya akibatnya — rect menggambar bentuknya,
// gambar memotong dirinya menurut bentuk itu.
func (e *Element) validateRadius() error {
	if e.Radius < 0 {
		return invalidf("element %q has a negative radius", e.ID)
	}

	return finite(e.ID, "radius", e.Radius)
}

// validateEllipse tidak memeriksa radius: sudut membulat tidak berarti apa-apa
// pada bidang yang memang tidak bersudut. Nilainya diabaikan renderer, bukan
// ditolak — menolaknya berarti mengubah rect menjadi ellipse mustahil dilakukan
// tanpa membersihkan properti yang tidak terlihat pengaruhnya.
func (e *Element) validateEllipse() error {
	return e.validateShape()
}

// validateShape adalah bagian yang sama antara rect dan ellipse.
func (e *Element) validateShape() error {
	if err := e.requireBox(); err != nil {
		return err
	}
	if e.StrokeWidth < 0 {
		return invalidf("element %q has a negative strokeWidth", e.ID)
	}
	if err := finite(e.ID, "strokeWidth", e.StrokeWidth); err != nil {
		return err
	}
	if err := strokeStyle(e.ID, e.StrokeStyle); err != nil {
		return err
	}
	if err := color(e.ID, "fill", e.Fill); err != nil {
		return err
	}

	return color(e.ID, "stroke", e.Stroke)
}

// validateLine sengaja tidak memakai requireBox: w dan h pada garis adalah
// simpangan ujung terhadap pangkal, sehingga negatif berarti menurun atau ke
// kiri, bukan ukuran yang keliru.
func (e *Element) validateLine() error {
	if e.StrokeWidth < 0 {
		return invalidf("element %q has a negative strokeWidth", e.ID)
	}
	if err := finite(e.ID, "strokeWidth", e.StrokeWidth); err != nil {
		return err
	}
	if err := strokeStyle(e.ID, e.StrokeStyle); err != nil {
		return err
	}

	return color(e.ID, "stroke", e.Stroke)
}

func (e *Element) validateImage() error {
	if err := e.requireBox(); err != nil {
		return err
	}
	if e.AssetToken == "" {
		return invalidf("element %q must have an assetToken", e.ID)
	}
	if err := e.validateRadius(); err != nil {
		return err
	}

	return oneOf(e.ID, "fit", e.Fit, FitContain, FitCover, FitFill)
}

func (e *Element) requireBox() error {
	if e.W < 0 || e.H < 0 {
		return invalidf("element %q has a negative size", e.ID)
	}

	return nil
}

func finite(id, name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return invalidf("element %q has a non-finite %s", id, name)
	}
	if math.Abs(value) > coordinateLimit {
		return invalidf("element %q has %s out of range: %g", id, name, value)
	}

	return nil
}

// oneOf memeriksa nilai enumerasi. Nilai kosong selalu lolos — itu berarti
// properti tidak disebutkan dan nilai bawaan yang berlaku.
func oneOf(id, name, value string, allowed ...string) error {
	if value == "" || slices.Contains(allowed, value) {
		return nil
	}

	return invalidf("element %q has %s %q, expected one of %s", id, name, value, strings.Join(allowed, ", "))
}

// color menerima #rgb dan #rrggbb. Bentuk lain — nama warna CSS, rgba(), hsl() —
// sengaja ditolak: menerimanya berarti backend harus menafsirkan sebagian CSS,
// dan justru penafsiran sebagian itulah yang membuat layar dan cetak berbeda.
func color(id, name, value string) error {
	if IsColor(value) {
		return nil
	}

	return invalidf("element %q has %s %q, expected #rgb or #rrggbb", id, name, value)
}

// IsColor menjawab apakah nilainya warna yang dikenal. Kosong dianggap sah:
// "tidak digambar" adalah keadaan yang berarti, bukan nilai yang hilang.
//
// Diekspor karena lapisan delivery memakainya juga: page.update memeriksa
// background di sana, sejalan dengan cara ia menolak muatan cacat lainnya —
// sebelum menyentuh orchestrator, dan tanpa membuat page.update perlu membalas.
func IsColor(value string) bool {
	if value == "" {
		return true
	}

	digits, ok := strings.CutPrefix(value, "#")
	if !ok || (len(digits) != 3 && len(digits) != 6) {
		return false
	}
	for _, char := range digits {
		isHex := (char >= '0' && char <= '9') ||
			(char >= 'a' && char <= 'f') ||
			(char >= 'A' && char <= 'F')
		if !isHex {
			return false
		}
	}

	return true
}

// strokeStyle hanya berlaku bagi rect dan line. Teks maupun gambar yang
// membawanya tidak ditolak, sama seperti properti lain yang bukan miliknya —
// yang tidak digambar juga tidak dapat membuat layar dan cetak berbeda.
func strokeStyle(id, value string) error {
	return oneOf(id, "strokeStyle", value, StrokeSolid, StrokeLongDash, StrokeDash, StrokeDot)
}

func invalidf(format string, args ...any) error {
	return domain.NewError(domain.ErrInvalidInput, fmt.Sprintf(format, args...))
}
