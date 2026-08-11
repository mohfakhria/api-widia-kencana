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

	// contentY tidak dipakai di sini: baseline() menghitungnya sendiri dari kotak
	// isi yang sama, supaya tidak ada dua tempat yang menurunkan titik awal teks.
	contentX, _, contentWidth, contentHeight := element.ContentBox()
	if contentWidth <= 0 || contentHeight <= 0 {
		// Sisi dalam menghabiskan kotaknya. Tidak ada ruang untuk satu huruf pun,
		// dan memaksakannya hanya menghasilkan teks yang seluruhnya terpotong.
		return nil
	}

	lines, font, err := c.layoutText(element)
	if err != nil {
		return err
	}

	red, green, blue, _ := design.ParseColor(element.ResolvedColor())
	c.pdf.SetTextColor(red, green, blue)
	// Hiasan digambar sebagai bidang terisi, bukan sebagai teks, sehingga ia
	// memakai warna ISI — bukan warna teks. Disetel sekali di sini, bukan di tiap
	// baris, karena tidak ada yang mengubahnya di antara baris.
	c.pdf.SetFillColor(red, green, blue)

	baseline, step := c.baseline(font, element)
	baseline += verticalOffset(element.ResolvedVerticalAlign(),
		contentHeight, float64(len(lines))*step)
	align := element.ResolvedAlign()

	c.pdf.ClipRect(element.X, element.Y, element.W, element.H, false)
	defer c.pdf.ClipEnd()

	for index, line := range lines {
		// Baris terakhir sebuah paragraf tidak diratakan penuh, sama seperti di
		// browser. Meratakannya akan meregangkan sisa kalimat pendek sampai
		// selebar kotaknya.
		last := index == len(lines)-1
		c.drawTextLine(element, line, contentX, baseline, contentWidth, align, element.LetterSpacing, last)
		baseline += step
	}

	return nil
}

// verticalOffset menggeser seluruh blok teks di dalam kotak isinya.
//
// Blok yang lebih tinggi daripada kotaknya menghasilkan geseran negatif pada
// middle dan bottom, sehingga bagian atasnya yang terpotong kliping — sama
// seperti kotak flex yang isinya meluap di CSS. Tidak dijepit ke nol: menjepitnya
// berarti middle diam-diam berubah menjadi top tepat ketika isinya bertambah satu
// baris, dan pergeseran yang tidak diminta itu lebih membingungkan daripada
// terpotong.
func verticalOffset(align string, contentHeight, blockHeight float64) float64 {
	switch align {
	case design.VAlignMiddle:
		return (contentHeight - blockHeight) / 2
	case design.VAlignBottom:
		return contentHeight - blockHeight
	default:
		return 0
	}
}

// layoutText memilih font, memasangnya pada kanvas, lalu memenggal teks menjadi
// baris-baris.
//
// Ketiganya berurutan dan tidak boleh dipisah. Font dipasang sebagai KEADAAN pada
// fpdf, dan c.textEncoder mengikutinya — keduanya wajib disetel sebelum satu huruf
// pun diukur. Memenggal baris dengan font yang belum terpasang menghasilkan lebar
// milik font sebelumnya, dan gejalanya berupa pemenggalan yang meleset hanya pada
// elemen yang kebetulan berbeda fontnya dari elemen di atasnya.
func (c *canvas) layoutText(element *design.Element) ([]string, selectedFont, error) {
	font, err := c.selectFont(
		element.ResolvedFontFamily(), element.ResolvedFontWeight(), element.ResolvedFontStyle())
	if err != nil {
		return nil, selectedFont{}, err
	}

	c.pdf.SetFont(font.name, font.style, element.ResolvedFontSize())
	c.textEncoder = font.encode

	_, _, contentWidth, _ := element.ContentBox()

	return c.wrapText(element.Text, contentWidth, element.LetterSpacing), font, nil
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

	_, contentY, _, _ := element.ContentBox()

	return contentY + halfLeading + ascent, step
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
func (c *canvas) drawTextLine(element *design.Element, line string, x, baseline, maxWidth float64, align string, letterSpacing float64, last bool) {
	if line == "" {
		return
	}

	if align == design.AlignJustify && !last {
		width := c.drawJustified(line, x, baseline, maxWidth, letterSpacing)
		c.decorate(element, x, baseline, width)
		return
	}

	width := c.measure(line, letterSpacing)
	switch align {
	case design.AlignCenter:
		x += (maxWidth - width) / 2
	case design.AlignRight:
		x += maxWidth - width
	}

	c.drawString(line, x, baseline, letterSpacing)
	c.decorate(element, x, baseline, width)
}

// decorate menarik garis bawah dan garis coret sepanjang baris yang baru
// digambar.
//
// DIGAMBAR SENDIRI, tidak memakai penanda "U" dan "S" milik fpdf. Penanda itu
// menempelkan garisnya pada tiap pemanggilan Text, dan lebarnya dihitung dari
// GetStringWidth — sehingga ia benar HANYA pada kasus paling sederhana. Dua
// fitur yang sudah ada merusaknya: dengan letterSpacing, teks digambar per huruf
// sehingga garisnya terputus di tiap sela; dengan perataan penuh, teks digambar
// per kata sehingga garisnya bolong di antara kata. Keduanya menghasilkan garis
// putus-putus yang tidak pernah diminta siapa pun, dan bedanya dari layar baru
// terlihat setelah dicetak.
//
// Menggambarnya sendiri menyatukan seluruh kasus itu menjadi satu bidang
// menerus, dengan harga metrik yang dipatok — lihat underlinePosition.
func (c *canvas) decorate(element *design.Element, x, baseline, width float64) {
	if width <= 0 || (!element.Underline && !element.Strikethrough) {
		return
	}

	size := element.ResolvedFontSize()
	thickness := underlineThickness / 1000.0 * size

	if element.Underline {
		c.pdf.Rect(x, baseline-underlinePosition/1000.0*size, width, thickness, "F")
	}
	if element.Strikethrough {
		c.pdf.Rect(x, baseline+strikeoutMultiple*underlinePosition/1000.0*size, width, thickness, "F")
	}
}

// drawJustified meregangkan jarak antar kata sampai baris memenuhi lebar kotak.
//
// Yang diregangkan hanya jarak antar kata, bukan jarak antar huruf. Browser
// melakukan hal yang sama pada text-align: justify bawaan.
// drawJustified mengembalikan lebar yang BENAR-BENAR ditempati baris itu.
//
// Bukan selalu maxWidth: baris berisi satu kata, dan baris yang sudah lebih lebar
// daripada kotaknya, digambar apa adanya tanpa diregangkan. Hiasan memakai lebar
// ini, dan tanpa pembedaan itu garis bawah pada baris semacam itu akan menjulur
// melewati hurufnya.
func (c *canvas) drawJustified(line string, x, baseline, maxWidth, letterSpacing float64) float64 {
	words := strings.Fields(line)
	if len(words) < 2 {
		c.drawString(line, x, baseline, letterSpacing)
		return c.measure(line, letterSpacing)
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
		return total
	}

	for index, word := range words {
		c.drawString(word, x, baseline, letterSpacing)
		x += widths[index] + gap
	}

	return maxWidth
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
	asked := faceKey{family: family, weight: weight, style: style}

	// resolve dijalankan untuk SETIAP elemen, bukan sekali lalu disimpan bersama
	// hasil pendaftaran. Kalau ia ikut dilewati oleh cache, penggantian hanya
	// terhitung pada elemen pertama — dan catatan yang menyebut satu elemen
	// padahal dua ratus yang terpengaruh lebih menyesatkan daripada tidak ada
	// catatan sama sekali. Ongkosnya satu pencarian map pada peta yang isinya
	// sedikit, di jalur yang toh sedang menggambar PDF.
	chosen := c.fonts.resolve(family, weight, style)
	if chosen.used != asked {
		c.substitutions[substitution{asked: asked, used: chosen.used}]++
	}

	// Yang di-cache pendaftarannya, dikunci pada potongan yang DIPAKAI. Beberapa
	// permintaan berbeda kerap bermuara ke potongan yang sama — 500 dan 100
	// keduanya menjadi 400 — dan mendaftarkannya sekali sudah cukup.
	if registered, ok := c.registeredFonts[chosen.used]; ok {
		return registered, nil
	}

	font, err := c.registerFont(chosen.used, chosen.data, chosen.core)
	if err != nil {
		return selectedFont{}, err
	}
	c.registeredFonts[chosen.used] = font

	return font, nil
}

// registerFont mendaftarkan satu potongan yang SUDAH terpilih.
//
// key di sini selalu potongan yang benar-benar ada — resolve yang memastikannya.
// Galat yang tersisa hanya galat sungguhan: berkas font terdaftar yang ternyata
// tidak dapat diurai fpdf, atau yang tidak punya metrik vertikal. Keduanya bug
// pemasangan, bukan kesalahan pemakai, dan memang harus menggagalkan ekspor.
func (c *canvas) registerFont(key faceKey, data []byte, core bool) (selectedFont, error) {
	if core {
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
