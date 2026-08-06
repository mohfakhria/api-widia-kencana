package pdf

import (
	"fmt"
	"math"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// selectedFont adalah font yang sudah terdaftar pada dokumen, dalam bentuk yang
// dikenal fpdf, beserta metrik vertikal yang dibutuhkan untuk menempatkan garis
// dasar.
//
// Ascent dan descent dibawa di sini, bukan diambil ulang tiap baris, karena
// keduanya sifat font dan bukan sifat elemen — dan mengambilnya sekali membuat
// jalur font inti dan font tersemat bertemu di satu tempat.
type selectedFont struct {
	name  string
	style string
	// Keduanya dalam seperseribu em. Descent bernilai negatif, mengikuti
	// perjanjian FontDescriptor pada PDF.
	ascent  int
	descent int
	// encode mengubah teks ke pengkodean yang dipahami font ini. Nil berarti
	// teksnya diserahkan apa adanya, yang berlaku untuk font Unicode yang
	// disematkan dari berkas.
	encode func(string) string
}

// drawText menggambar satu blok teks di dalam kotaknya.
//
// Tata letaknya mengikuti aturan CSS untuk kotak dengan white-space: pre-line dan
// overflow: hidden — dan frontend wajib memakai kedua nilai itu. Pergantian baris
// yang ditulis pengguna dihormati, deretan spasi dipadatkan jadi satu, dan teks
// yang melebihi kotaknya terpotong alih-alih meluber ke elemen tetangga.
func (c *canvas) drawText(element *design.Element) error {
	if element.Text == "" || element.W <= 0 || element.H <= 0 {
		return nil
	}

	font, err := c.selectFont(element.ResolvedFontFamily(), element.ResolvedFontWeight(), element.ResolvedFontStyle())
	if err != nil {
		return err
	}

	size := element.ResolvedFontSize()
	c.pdf.SetFont(font.name, font.style, size)
	c.textEncoder = font.encode

	red, green, blue, _ := design.ParseColor(element.ResolvedColor())
	c.pdf.SetTextColor(red, green, blue)

	baseline, step := c.baseline(font, element)

	lines := c.wrapText(element.Text, element.W, element.LetterSpacing)
	align := element.ResolvedAlign()

	c.pdf.ClipRect(element.X, element.Y, element.W, element.H, false)
	defer c.pdf.ClipEnd()

	for index, line := range lines {
		// Baris terakhir sebuah paragraf tidak diratakan penuh, sama seperti di
		// browser. Meratakannya akan meregangkan sisa kalimat pendek sampai
		// selebar kotaknya.
		last := index == len(lines)-1
		c.drawTextLine(line, element.X, baseline, element.W, align, element.LetterSpacing, last)
		baseline += step
	}

	return nil
}

// baseline menghitung posisi garis dasar baris pertama dan jarak antar baris.
//
// Sisa ruang antara tinggi baris dan tinggi huruf dibagi rata di atas dan di
// bawah — inilah yang di CSS disebut half-leading, dan mengabaikannya adalah
// penyebab paling umum teks tercetak bergeser beberapa titik dari tampilan layar.
//
// Ascent dan descent diambil dari FontDescriptor pada berkas font. Perlu dicatat
// bahwa browser tidak selalu memakai nilai yang sama: sebagian mengambil metrik
// hhea, sebagian OS/2. Bila teks tampak bergeser vertikal secara konsisten pada
// satu keluarga font, di sinilah tempat memeriksanya.
func (c *canvas) baseline(font selectedFont, element *design.Element) (first, step float64) {
	size := element.ResolvedFontSize()
	ascent := float64(font.ascent) / 1000 * size
	descent := math.Abs(float64(font.descent)) / 1000 * size

	step = size * element.ResolvedLineHeight()
	halfLeading := (step - (ascent + descent)) / 2

	return element.Y + halfLeading + ascent, step
}

// wrapText memenggal teks menjadi baris-baris yang muat di dalam lebar kotak.
//
// Algoritmanya rakus: kata ditambahkan selama masih muat, dan dipindahkan ke
// baris berikutnya begitu tidak. Sengaja sesederhana itu, karena frontend harus
// menghasilkan pemenggalan yang sama persis — algoritma yang lebih pandai, seperti
// perataan Knuth-Plass, tidak akan pernah cocok dengan yang dilakukan browser.
//
// Kata yang lebih lebar daripada kotaknya dibiarkan meluber, tidak dipatahkan di
// tengah. Itu perilaku bawaan CSS dengan overflow-wrap: normal.
func (c *canvas) wrapText(text string, maxWidth, letterSpacing float64) []string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")

	var lines []string
	for paragraph := range strings.SplitSeq(normalized, "\n") {
		lines = append(lines, c.wrapParagraph(paragraph, maxWidth, letterSpacing)...)
	}

	return lines
}

func (c *canvas) wrapParagraph(paragraph string, maxWidth, letterSpacing float64) []string {
	// Fields memadatkan deretan spasi dan tab menjadi satu pemisah, sama seperti
	// yang dilakukan browser pada teks biasa.
	words := strings.Fields(paragraph)
	if len(words) == 0 {
		// Baris kosong tetap memakan satu tinggi baris. Menghapusnya akan membuat
		// jarak antar paragraf yang sengaja dibuat pengguna lenyap saat dicetak.
		return []string{""}
	}

	lines := make([]string, 0, 1)
	current := words[0]

	for _, word := range words[1:] {
		candidate := current + " " + word
		if c.measure(candidate, letterSpacing) > maxWidth {
			lines = append(lines, current)
			current = word
			continue
		}
		current = candidate
	}

	return append(lines, current)
}

// measure menghitung lebar teks dalam titik.
//
// Jarak antar huruf ditambahkan setelah SETIAP huruf, termasuk yang terakhir.
// Itu yang dilakukan CSS, dan menghilangkan yang terakhir akan membuat lebar
// baris meleset sebesar satu jarak — cukup untuk menggeser pemenggalan baris pada
// teks yang panjang.
func (c *canvas) measure(text string, letterSpacing float64) float64 {
	width := c.pdf.GetStringWidth(c.encode(text))
	if letterSpacing != 0 {
		width += letterSpacing * float64(len([]rune(text)))
	}

	return width
}

// encode menyiapkan teks untuk font yang sedang dipakai.
//
// Seluruh teks yang menyeberang ke fpdf — baik untuk diukur maupun digambar —
// wajib lewat sini. Mengukur teks yang belum dikodekan lalu menggambar yang sudah,
// atau sebaliknya, akan menghasilkan lebar yang tidak sesuai dengan yang tergambar.
func (c *canvas) encode(text string) string {
	if c.textEncoder == nil {
		return text
	}

	return c.textEncoder(text)
}

// drawTextLine menempatkan satu baris menurut perataannya.
func (c *canvas) drawTextLine(line string, x, baseline, maxWidth float64, align string, letterSpacing float64, last bool) {
	if line == "" {
		return
	}

	if align == design.AlignJustify && !last {
		c.drawJustified(line, x, baseline, maxWidth, letterSpacing)
		return
	}

	switch align {
	case design.AlignCenter:
		x += (maxWidth - c.measure(line, letterSpacing)) / 2
	case design.AlignRight:
		x += maxWidth - c.measure(line, letterSpacing)
	}

	c.drawString(line, x, baseline, letterSpacing)
}

// drawJustified meregangkan jarak antar kata sampai baris memenuhi lebar kotak.
//
// Yang diregangkan hanya jarak antar kata, bukan jarak antar huruf. Browser
// melakukan hal yang sama pada text-align: justify bawaan.
func (c *canvas) drawJustified(line string, x, baseline, maxWidth, letterSpacing float64) {
	words := strings.Fields(line)
	if len(words) < 2 {
		c.drawString(line, x, baseline, letterSpacing)
		return
	}

	total := 0.0
	widths := make([]float64, len(words))
	for index, word := range words {
		widths[index] = c.measure(word, letterSpacing)
		total += widths[index]
	}

	gap := (maxWidth - total) / float64(len(words)-1)
	if gap < 0 {
		// Barisnya sudah lebih lebar daripada kotaknya, biasanya karena satu kata
		// panjang yang tidak dapat dipenggal. Merapatkan kata sampai tumpang tindih
		// hanya membuatnya tidak terbaca.
		c.drawString(line, x, baseline, letterSpacing)
		return
	}

	for index, word := range words {
		c.drawString(word, x, baseline, letterSpacing)
		x += widths[index] + gap
	}
}

// drawString menggambar teks apa adanya mulai dari x.
//
// Jarak antar huruf tidak punya perintah tersendiri di fpdf, jadi teks yang
// memakainya digambar per huruf pada posisi yang dihitung sendiri. Itu membuat
// berkasnya sedikit lebih besar, tetapi memakai lebar yang persis sama dengan
// yang dipakai measure — dan kesamaan itu yang menentukan pemenggalan barisnya
// benar.
func (c *canvas) drawString(text string, x, baseline, letterSpacing float64) {
	if letterSpacing == 0 {
		c.pdf.Text(x, baseline, c.encode(text))
		return
	}

	for _, symbol := range text {
		glyph := c.encode(string(symbol))
		c.pdf.Text(x, baseline, glyph)
		x += c.pdf.GetStringWidth(glyph) + letterSpacing
	}
}

// selectFont mendaftarkan font ke dokumen bila belum, lalu mengembalikan namanya
// dalam bentuk yang dikenal fpdf.
//
// Setiap potongan font didaftarkan sebagai keluarga tersendiri di mata fpdf,
// karena fpdf hanya menyediakan empat slot gaya per keluarga sedangkan model isi
// dokumen mengenal sembilan tingkat ketebalan.
func (c *canvas) selectFont(family string, weight int, style string) (selectedFont, error) {
	key := faceKey{family: family, weight: weight, style: style}
	if registered, ok := c.registeredFonts[key]; ok {
		return registered, nil
	}

	data, core, err := c.fonts.resolve(family, weight, style)
	if err != nil {
		return selectedFont{}, domain.NewError(domain.ErrInvalidInput, err.Error())
	}

	font, err := c.registerFont(key, data, core)
	if err != nil {
		return selectedFont{}, err
	}
	c.registeredFonts[key] = font

	return font, nil
}

func (c *canvas) registerFont(key faceKey, data []byte, core bool) (selectedFont, error) {
	if core {
		// Keluarga inti hanya punya tegak dan tebal. Ketebalan lain ditolak alih-alih
		// dibulatkan ke yang terdekat, supaya tidak ada dokumen yang tercetak dengan
		// ketebalan berbeda dari yang tampil di layar.
		if key.weight != 400 && key.weight != 700 {
			return selectedFont{}, domain.NewError(domain.ErrInvalidInput, fmt.Sprintf(
				"font %s only provides weight 400 and 700, not %d", CoreFamily, key.weight))
		}

		style := ""
		if key.weight == 700 {
			style += "B"
		}
		if key.style == design.FontStyleItalic {
			style += "I"
		}

		return selectedFont{
			name:    CoreFamily,
			style:   style,
			ascent:  coreAscent,
			descent: coreDescent,
			encode:  encodeCP1252,
		}, nil
	}

	name := fmt.Sprintf("%s-%d-%s", key.family, key.weight, key.style)
	c.pdf.AddUTF8FontFromBytes(name, "", data)
	if err := c.pdf.Error(); err != nil {
		return selectedFont{}, domain.NewError(domain.ErrInternalFailure, fmt.Sprintf(
			"font %s %d %s could not be embedded", key.family, key.weight, key.style))
	}

	// Metrik dibaca dari FontDescriptor hasil penguraian berkasnya. Font tanpa
	// metrik vertikal tidak dapat ditempatkan garis dasarnya, dan menebaknya akan
	// menghasilkan teks yang bergeser tanpa penjelasan.
	descriptor := c.pdf.GetFontDesc(name, "")
	if descriptor.Ascent == 0 {
		return selectedFont{}, domain.NewError(domain.ErrInternalFailure, fmt.Sprintf(
			"font %s %d %s has no usable vertical metrics", key.family, key.weight, key.style))
	}

	return selectedFont{
		name:    name,
		ascent:  descriptor.Ascent,
		descent: descriptor.Descent,
	}, nil
}
