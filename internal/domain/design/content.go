// Package design memuat model isi kanvas dokumen.
//
// Model ini sengaja TERTUTUP: hanya properti yang benar-benar digambar oleh
// renderer yang boleh ada, dan properti di luar daftar ditolak saat diurai.
// Sebelumnya isi elemen diperlakukan sebagai data buram yang lewat begitu saja,
// karena yang merender berada di sisi frontend. Begitu backend ikut menggambar —
// untuk ekspor PDF — sifat buram itu berubah menjadi jebakan: properti yang tidak
// dipahami backend akan tampil di layar tetapi hilang di hasil cetak, tanpa error
// maupun log. Menolaknya di pintu masuk membuat perbedaan itu mustahil.
//
// # Satuan
//
// Seluruh koordinat dan ukuran memakai titik (pt), 1/72 inci. Itu satuan asli
// PDF, sehingga renderer tidak pernah mengonversi apa pun, sekaligus satuan sah
// di CSS — frontend cukup menempelkan "pt" pada angka yang sama. Browser dan PDF
// karenanya membaca ukuran yang persis sama.
//
// # Nilai bawaan
//
// Field opsional yang kosong memakai nilai bawaan di paket ini. Nilainya harus
// sama persis dengan nilai bawaan di frontend; bila berbeda, elemen yang tidak
// menyebutkan properti itu akan tampil berbeda antara layar dan cetak. Karena itu
// seluruh nilai bawaan dinyatakan sebagai konstanta bernama di bawah, bukan angka
// yang tersebar di dalam renderer.
package design

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ElementType adalah jenis elemen yang dikenal renderer.
type ElementType string

const (
	ElementText  ElementType = "text"
	ElementRect  ElementType = "rect"
	ElementLine  ElementType = "line"
	ElementImage ElementType = "image"
)

// Nilai bawaan untuk properti teks yang tidak disebutkan. Wajib sama dengan
// nilai bawaan frontend.
const (
	DefaultFontFamily    = "helvetica"
	DefaultFontSize      = 12.0
	DefaultFontWeight    = 400
	DefaultFontStyle     = FontStyleNormal
	DefaultColor         = "#000000"
	DefaultAlign         = AlignLeft
	DefaultLineHeight    = 1.2
	DefaultLetterSpacing = 0.0
	DefaultImageFit      = FitContain
)

const (
	FontStyleNormal = "normal"
	FontStyleItalic = "italic"
)

const (
	AlignLeft    = "left"
	AlignCenter  = "center"
	AlignRight   = "right"
	AlignJustify = "justify"
)

const (
	// FitContain memuat seluruh gambar di dalam kotak tanpa terpotong, menyisakan
	// ruang pada sisi yang tidak pas. FitCover memenuhi kotak dan memotong sisa.
	// FitFill meregangkan gambar mengikuti kotak, mengabaikan rasio aslinya.
	FitContain = "contain"
	FitCover   = "cover"
	FitFill    = "fill"
)

// Content adalah isi kanvas satu dokumen.
type Content struct {
	Pages []Page `json:"pages"`
}

// Page adalah satu halaman kertas beserta elemen di atasnya.
//
// Ukuran halaman tidak disimpan di sini melainkan diambil dari kertas dokumen,
// supaya seluruh halaman satu dokumen dijamin seukuran dan tidak ada jalan bagi
// data untuk bertentangan dengan kertas yang dipilih pengguna.
type Page struct {
	ID       string    `json:"id"`
	Elements []Element `json:"elements"`
}

// Element adalah satu objek di atas halaman.
//
// Seluruh properti dari keempat jenis elemen berada dalam satu struct datar,
// bukan union bertingkat. Bentuk datar membuat patch parsial nanti menjadi
// penggabungan satu tingkat tanpa aturan khusus yang perlu dihafal, dan membuat
// pesan pembaruan dari frontend cukup menyebut nama properti apa adanya.
//
// Properti yang bukan milik jenis elemennya diabaikan saat menggambar, tidak
// ditolak. Properti semacam itu tidak tampil di layar maupun di cetak, jadi ia
// tidak dapat menimbulkan perbedaan antara keduanya — dan menolaknya hanya akan
// mempersulit frontend yang mengubah jenis sebuah elemen.
type Element struct {
	ID   string      `json:"id"`
	Type ElementType `json:"type"`

	// X dan Y adalah sudut kiri atas kotak elemen, relatif terhadap sudut kiri
	// atas halaman. W dan H adalah lebar dan tingginya.
	//
	// Khusus ElementLine, W dan H bukan ukuran melainkan simpangan ujung garis
	// terhadap pangkalnya, sehingga keduanya boleh negatif.
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`

	// Properti teks.
	Text          string  `json:"text,omitempty"`
	FontFamily    string  `json:"fontFamily,omitempty"`
	FontSize      float64 `json:"fontSize,omitempty"`
	FontWeight    int     `json:"fontWeight,omitempty"`
	FontStyle     string  `json:"fontStyle,omitempty"`
	Color         string  `json:"color,omitempty"`
	Align         string  `json:"align,omitempty"`
	LineHeight    float64 `json:"lineHeight,omitempty"`
	LetterSpacing float64 `json:"letterSpacing,omitempty"`

	// Properti bidang dan garis. Fill dan Stroke kosong berarti tidak digambar,
	// bukan hitam — sehingga kotak tanpa isi maupun garis tepi dapat dinyatakan.
	Fill        string  `json:"fill,omitempty"`
	Stroke      string  `json:"stroke,omitempty"`
	StrokeWidth float64 `json:"strokeWidth,omitempty"`
	Radius      float64 `json:"radius,omitempty"`

	// Properti gambar. AssetToken menunjuk aset yang sudah terunggah, bukan URL,
	// supaya renderer tidak pernah mengambil alamat yang ditentukan klien.
	AssetToken string `json:"assetToken,omitempty"`
	Fit        string `json:"fit,omitempty"`
}

// Decode mengurai isi dokumen dan memvalidasinya.
//
// DisallowUnknownFields adalah inti dari model tertutup ini: properti yang tidak
// dikenal menghasilkan error, bukan diam-diam dibuang. Tanpa itu frontend dapat
// mengirim properti yang tidak pernah digambar backend dan baru menyadarinya
// ketika hasil ekspor berbeda dari layar.
//
// Isi kosong diperlakukan sebagai dokumen tanpa halaman, bukan error: kolom
// content pada dokumen yang baru dibuat memang belum berisi apa-apa.
func Decode(raw json.RawMessage) (*Content, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return &Content{Pages: []Page{}}, nil
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()

	var content Content
	if err := decoder.Decode(&content); err != nil {
		return nil, invalidf("document content is not valid: %s", err)
	}

	if content.Pages == nil {
		content.Pages = []Page{}
	}

	if err := content.Validate(); err != nil {
		return nil, err
	}

	return &content, nil
}

// Encode mengubah isi kembali menjadi JSON.
func (c *Content) Encode() (json.RawMessage, error) {
	encoded, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("encode document content: %w", err)
	}

	return encoded, nil
}

// IsEmpty menandai dokumen yang belum punya halaman sama sekali — kandidat untuk
// diisi benih.
func (c *Content) IsEmpty() bool {
	return len(c.Pages) == 0
}
