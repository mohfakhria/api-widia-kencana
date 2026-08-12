package pdf

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"math"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// SVG dirasterkan menjadi PNG lalu disematkan seperti gambar biasa.
//
// PDF sebenarnya sanggup membawa vektor, dan fpdf punya SVGBasicWrite — tetapi
// ia hanya memahami data path pada SATU elemen <path>: tanpa isi, tanpa warna,
// tanpa transform, tanpa <rect> maupun <circle>. Logo sungguhan akan tergambar
// salah alih-alih tidak tergambar, dan "salah" jauh lebih sulit disadari
// daripada "hilang".
//
// Perender SVG yang setia — resvg, librsvg — menuntut cgo, dan itu menggugurkan
// CGO_ENABLED=0 yang menghasilkan binary statis untuk image scratch. Jadi
// pilihannya rasterisasi Go murni.

const (
	// svgRasterLongestSide adalah sisi terpanjang hasil raster, dalam piksel.
	//
	// Rasterisasi menukar keluwesan vektor dengan resolusi tetap, jadi angkanya
	// menentukan setajam apa hasilnya saat dicetak. Dua ribu piksel yang
	// direntang selebar A4 (595 pt) setara sekitar 248 dpi — di atas 150 dpi yang
	// biasa dianggap batas bawah cetak, dan di bawah 300 dpi yang akan
	// melipatgandakan memorinya tanpa terlihat bedanya di layar mana pun.
	//
	// Ongkos terburuknya bujur sangkar: 2048² × 4 byte ≈ 16 MB per aset, hanya
	// selama ekspor berlangsung.
	svgRasterLongestSide = 2048

	// svgFallbackSide dipakai ketika SVG tidak menyebutkan ukuran maupun viewBox.
	// Tanpa keduanya tidak ada yang dapat disimpulkan tentang bentuknya, dan
	// bujur sangkar adalah tebakan yang paling tidak merugikan.
	svgFallbackSide = 512
)

// rasterizeSVG mengubah SVG menjadi PNG beserta rasio aslinya.
//
// Rasio dikembalikan terpisah karena fitBox membutuhkannya untuk contain dan
// cover, dan rasio hasil raster dibulatkan ke piksel bulat — perbedaannya kecil
// tetapi cukup untuk menggeser gambar beberapa titik pada kotak yang besar.
func rasterizeSVG(data []byte) (encoded []byte, ratio float64, err error) {
	icon, err := oksvg.ReadIconStream(bytes.NewReader(data))
	if err != nil {
		return nil, 0, err
	}

	// oksvg TIDAK menganggap masukan yang bukan SVG sebagai galat: teks acak
	// menghasilkan ikon tanpa satu path pun, dan tanpa penjaga ini ia dirasterkan
	// menjadi PNG transparan lalu disematkan diam-diam — tidak terlihat, tidak
	// tercatat, yaitu persis kegagalan yang jalur ini diadakan untuk menghapus.
	//
	// Nol path juga berarti SVG yang memang kosong. Keduanya sama-sama tidak
	// menghasilkan piksel apa pun, jadi tidak ada gunanya dibedakan.
	if len(icon.SVGPaths) == 0 {
		return nil, 0, errors.New("svg has no drawable content")
	}

	srcW, srcH := icon.ViewBox.W, icon.ViewBox.H
	if srcW <= 0 || srcH <= 0 {
		srcW, srcH = svgFallbackSide, svgFallbackSide
	}
	ratio = srcW / srcH

	width, height := rasterSize(srcW, srcH)

	// SetTarget merentang gambar ke kotak ini, sekaligus menerapkan viewBox.
	icon.SetTarget(0, 0, float64(width), float64(height))

	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	// Latar dibiarkan transparan, bukan diisi putih. SVG yang memang bertransparansi
	// — hampir semua logo — akan menumpuk kotak putih di atas latar halaman bila
	// diisi, dan itu terlihat justru pada dokumen yang latarnya bukan putih.
	draw.Draw(canvas, canvas.Bounds(), image.Transparent, image.Point{}, draw.Src)

	scanner := rasterx.NewScannerGV(width, height, canvas, canvas.Bounds())
	icon.Draw(rasterx.NewDasher(width, height, scanner), 1.0)

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		return nil, 0, err
	}
	if buffer.Len() == 0 {
		return nil, 0, errors.New("svg produced an empty raster")
	}

	return buffer.Bytes(), ratio, nil
}

// rasterSize menskalakan ukuran asli sehingga sisi terpanjangnya menyentuh
// svgRasterLongestSide — MEMBESARKAN yang kecil sekaligus mengecilkan yang besar.
//
// Membesarkan yang kecil itu yang penting, dan mudah terlewat: ikon kerap
// berukuran asli 24×24, dan merasterkannya apa adanya lalu merentangnya ke
// seperempat halaman menghasilkan gambar yang kabur tanpa ada yang tahu
// sebabnya.
func rasterSize(srcW, srcH float64) (width, height int) {
	scale := svgRasterLongestSide / math.Max(srcW, srcH)

	width = int(math.Round(srcW * scale))
	height = int(math.Round(srcH * scale))

	// Sisi yang membulat menjadi nol pada SVG yang sangat pipih. Satu piksel
	// tetap gambar yang sah; nol membuat rasterx panik.
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}

	return width, height
}

// svgHasText menjawab apakah SVG ini memuat teks hidup.
//
// oksvg tidak menggambar <text> sama sekali — ia menuntut pemuatan font dan
// penataan glyph yang memang di luar cakupannya. Tanpa pemeriksaan ini, logo
// bertulisan akan kehilangan tulisannya DIAM-DIAM, yaitu persoalan yang sama
// persis dengan SVG yang dulu dilewati tanpa jejak.
//
// Pencocokan teks mentah, bukan penguraian XML: yang dicari cuma keberadaannya,
// dan mengurai seluruh dokumen hanya untuk pertanyaan itu jauh lebih mahal
// daripada nilainya. Positif palsu — kata "<text" di dalam komentar — paling
// banter menghasilkan satu peringatan yang tidak perlu.
func svgHasText(data []byte) bool {
	return bytes.Contains(data, []byte("<text")) || bytes.Contains(data, []byte("<tspan"))
}
