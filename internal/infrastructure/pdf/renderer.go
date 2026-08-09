package pdf

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/go-pdf/fpdf"
)

// maxPageSide adalah batas ukuran halaman yang dapat dinyatakan PDF, yaitu 14400
// titik atau 200 inci per sisi.
const maxPageSide = 14400

// Renderer menggambar isi dokumen menjadi PDF.
//
// Aman dipakai bersamaan dari banyak goroutine: setiap pemanggilan RenderPDF
// membuat dokumen fpdf sendiri, dan Fonts hanya dibaca setelah dimuat saat start.
type Renderer struct {
	fonts *Fonts
}

func NewRenderer(fonts *Fonts) *Renderer {
	return &Renderer{fonts: fonts}
}

// canvas adalah keadaan satu kali penggambaran.
//
// Font dan gambar didaftarkan malas — hanya yang benar-benar terpakai, dan hanya
// sekali walau dipakai puluhan elemen. Mendaftarkan seluruh font yang tersedia di
// muka akan menyematkan berkas yang tidak dipakai ke dalam setiap PDF.
type canvas struct {
	pdf    *fpdf.Fpdf
	fonts  *Fonts
	images map[string]output.RenderImage

	registeredFonts  map[faceKey]selectedFont
	registeredImages map[string]imageRegistration

	// textEncoder mengikuti font yang sedang terpasang, sama seperti fpdf yang
	// juga menyimpan font terpilih sebagai keadaan. Disetel di drawText, sebelum
	// satu pun huruf diukur.
	textEncoder func(string) string
}

type imageRegistration struct {
	name  string
	ratio float64
}

func (r *Renderer) RenderPDF(ctx context.Context, document output.RenderDocument) ([]byte, error) {
	width, height := document.PageWidth, document.PageHeight
	if width <= 0 || height <= 0 {
		return nil, domain.NewError(domain.ErrInvalidInput, "document paper size is not usable for export")
	}
	if width > maxPageSide || height > maxPageSide {
		return nil, domain.NewError(domain.ErrInvalidInput, fmt.Sprintf(
			"document paper is larger than the %d pt PDF limit", maxPageSide))
	}

	size := fpdf.SizeType{Wd: width, Ht: height}
	doc := fpdf.NewCustom(&fpdf.InitType{
		// Orientasi selalu potret, dan ukuran diberikan apa adanya. Kertas
		// lanskap sudah terwakili oleh lebar yang melebihi tinggi, jadi
		// menyerahkan pemutaran ke fpdf hanya akan menukar sisinya dua kali.
		OrientationStr: "P",
		UnitStr:        "pt",
		Size:           size,
	})
	// Isi dokumen menempatkan elemennya sendiri secara mutlak. Pemenggalan
	// halaman otomatis dan margin bawaan justru akan menggeser apa yang sudah
	// ditempatkan.
	doc.SetAutoPageBreak(false, 0)
	doc.SetMargins(0, 0, 0)
	doc.SetCompression(true)

	c := &canvas{
		pdf:              doc,
		fonts:            r.fonts,
		images:           document.Images,
		registeredFonts:  make(map[faceKey]selectedFont),
		registeredImages: make(map[string]imageRegistration),
	}

	// Disaring lebih dulu, bukan dilewati di dalam perulangan. Penjaga di bawah
	// menghitung dari daftar ini, sehingga dokumen yang SELURUH halamannya
	// tersembunyi ikut tertangkap — kalau penyaringannya di dalam perulangan,
	// penjaga itu lolos dan hasilnya PDF tanpa halaman sama sekali.
	pages := document.Content.VisiblePages()
	if len(pages) == 0 {
		// PDF tanpa halaman bukan berkas yang sah. Dokumen yang masih kosong —
		// atau yang seluruh halamannya disembunyikan — menghasilkan satu halaman
		// kosong, yang juga jawaban yang benar bagi pengguna: itu memang yang akan
		// tercetak.
		doc.AddPageFormat("P", size)
	}

	for _, page := range pages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		doc.AddPageFormat("P", size)
		for index := range page.Elements {
			if err := c.drawElement(&page.Elements[index]); err != nil {
				return nil, err
			}
		}
	}

	var buffer bytes.Buffer
	if err := doc.Output(&buffer); err != nil {
		return nil, fmt.Errorf("write pdf: %w", err)
	}

	return buffer.Bytes(), nil
}

// drawElement menggambar satu elemen. Urutan penggambaran mengikuti urutan
// elemen di dalam halaman, sehingga yang belakangan menutupi yang terdahulu —
// aturan yang sama dengan urutan DOM di frontend.
func (c *canvas) drawElement(element *design.Element) error {
	switch element.Type {
	case design.ElementText:
		return c.drawText(element)
	case design.ElementRect:
		c.drawRect(element)
	case design.ElementLine:
		c.drawLine(element)
	case design.ElementImage:
		return c.drawImage(element)
	}

	return nil
}

func (c *canvas) drawRect(element *design.Element) {
	style := ""
	if element.Fill != "" {
		red, green, blue, _ := design.ParseColor(element.Fill)
		c.pdf.SetFillColor(red, green, blue)
		style += "F"
	}
	if element.Stroke != "" && element.StrokeWidth > 0 {
		c.applyStroke(element)
		style += "D"
	}
	if style == "" || element.W <= 0 || element.H <= 0 {
		// Tanpa isi maupun garis tepi tidak ada yang terlihat. Menggambarnya
		// hanya menambah perintah ke dalam berkas.
		return
	}

	if element.Radius > 0 {
		// Radius yang melebihi separuh sisi terpendek akan membuat lengkungnya
		// saling menembus. Browser membatasinya dengan cara yang sama.
		radius := math.Min(element.Radius, math.Min(element.W, element.H)/2)
		c.pdf.RoundedRect(element.X, element.Y, element.W, element.H, radius, "1234", style)
		return
	}

	c.pdf.Rect(element.X, element.Y, element.W, element.H, style)
}

func (c *canvas) drawLine(element *design.Element) {
	if element.Stroke == "" || element.StrokeWidth <= 0 {
		return
	}

	c.applyStroke(element)
	// Garis ditarik dari pangkal ke pangkal ditambah simpangan, dengan ketebalan
	// terbagi rata di kedua sisi jalurnya — perilaku yang sama dengan stroke pada
	// SVG.
	c.pdf.Line(element.X, element.Y, element.X+element.W, element.Y+element.H)
}

// applyStroke menyetel warna, ketebalan, DAN pola putus-putus sekaligus.
//
// Ketiganya disetel bersama, tidak sebagian, karena pola putus-putus pada fpdf
// adalah keadaan yang bertahan sampai diganti. Menyetelnya hanya ketika elemen
// memang bergaris putus akan membuat pola itu menetes ke elemen berikutnya —
// dan ke halaman berikutnya — sampai ada yang kebetulan menggantinya. Gejalanya
// adalah garis yang seharusnya utuh menjadi putus, di tempat yang tidak ada
// hubungannya dengan elemen yang menyebabkannya.
//
// Dengan menyetel ketiganya di setiap penggambaran, urutan elemen tidak lagi
// dapat memengaruhi hasil. Pola kosong berarti utuh, jadi elemen solid pun
// menyatakan dirinya secara eksplisit.
func (c *canvas) applyStroke(element *design.Element) {
	red, green, blue, _ := design.ParseColor(element.Stroke)
	c.pdf.SetDrawColor(red, green, blue)
	c.pdf.SetLineWidth(element.StrokeWidth)
	c.pdf.SetDashPattern(design.StrokeDashPattern(element.ResolvedStrokeStyle(), element.StrokeWidth), 0)
}

func (c *canvas) drawImage(element *design.Element) error {
	if element.W <= 0 || element.H <= 0 {
		return nil
	}

	registration, ok, err := c.registerImage(element.AssetToken)
	if err != nil {
		return err
	}
	if !ok {
		// Aset yang tidak tersedia dilewati. Pilihan ini disengaja: dokumen yang
		// kehilangan satu gambar masih berguna, sedangkan ekspor yang gagal total
		// karena satu aset terhapus tidak.
		return nil
	}

	x, y, width, height := fitBox(element, registration.ratio)

	// Kliping menahan gambar di dalam kotaknya. Tanpa ini fit "cover" akan
	// meluber ke elemen di sekitarnya, karena ia memang sengaja menghasilkan
	// gambar yang lebih besar daripada kotaknya.
	c.pdf.ClipRect(element.X, element.Y, element.W, element.H, false)
	c.pdf.ImageOptions(registration.name, x, y, width, height, false, fpdf.ImageOptions{}, 0, "")
	c.pdf.ClipEnd()

	return nil
}

// fitBox menghitung penempatan gambar di dalam kotak elemen.
//
// Ketiga mode meniru object-fit pada CSS: contain memuat seluruh gambar dan
// menyisakan ruang, cover memenuhi kotak dan memotong sisanya, fill meregangkan
// tanpa memedulikan rasio asli.
func fitBox(element *design.Element, ratio float64) (x, y, width, height float64) {
	if element.ResolvedFit() == design.FitFill || ratio <= 0 {
		return element.X, element.Y, element.W, element.H
	}

	boxRatio := element.W / element.H
	wider := ratio > boxRatio
	if element.ResolvedFit() == design.FitCover {
		wider = !wider
	}

	if wider {
		width = element.W
		height = element.W / ratio
	} else {
		height = element.H
		width = element.H * ratio
	}

	return element.X + (element.W-width)/2, element.Y + (element.H-height)/2, width, height
}

func (c *canvas) registerImage(token string) (imageRegistration, bool, error) {
	if registration, ok := c.registeredImages[token]; ok {
		return registration, true, nil
	}

	image, ok := c.images[token]
	if !ok {
		return imageRegistration{}, false, nil
	}

	imageType, ok := imageTypeOf(image.MimeType)
	if !ok {
		return imageRegistration{}, false, nil
	}

	name := "asset-" + token
	info := c.pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: imageType}, bytes.NewReader(image.Data))
	if err := c.pdf.Error(); err != nil {
		return imageRegistration{}, false, domain.NewError(domain.ErrInvalidInput,
			fmt.Sprintf("image asset %s could not be decoded", token))
	}

	registration := imageRegistration{name: name}
	if info != nil && info.Height() > 0 {
		registration.ratio = info.Width() / info.Height()
	}
	c.registeredImages[token] = registration

	return registration, true, nil
}

// imageTypeOf memetakan tipe MIME ke nama format yang dikenal fpdf. Format di
// luar ketiganya tidak dapat disematkan ke PDF tanpa dikonversi lebih dulu.
func imageTypeOf(mimeType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		return "JPG", true
	case "image/png":
		return "PNG", true
	case "image/gif":
		return "GIF", true
	default:
		return "", false
	}
}
