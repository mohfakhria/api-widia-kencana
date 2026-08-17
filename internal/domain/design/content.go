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
	ElementText    ElementType = "text"
	ElementRect    ElementType = "rect"
	ElementEllipse ElementType = "ellipse"
	ElementLine    ElementType = "line"
	ElementImage   ElementType = "image"
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
	DefaultStrokeStyle   = StrokeSolid
	DefaultOpacity       = 1.0
	DefaultVerticalAlign = VAlignTop
)

const (
	FontStyleNormal = "normal"
	FontStyleItalic = "italic"
)

// Perataan tegak isi teks di dalam kotaknya. Berbeda dari align, yang meratakan
// tiap baris secara mendatar.
const (
	VAlignTop    = "top"
	VAlignMiddle = "middle"
	VAlignBottom = "bottom"
)

// Gaya garis untuk rect dan line.
//
// "Tanpa garis" sengaja BUKAN anggota: itu sudah diucapkan strokeWidth nol, dan
// dua cara mengatakan satu hal adalah dua cara untuk tidak sepakat.
//
// Panjang segmennya kelipatan strokeWidth, bukan angka mutlak, supaya polanya
// tetap sebanding pada garis tebal — dan supaya frontend menggambar dari rumus
// yang sama tanpa tabel kedua. Angka persisnya ada di StrokeDashPattern.
const (
	StrokeSolid    = "solid"
	StrokeLongDash = "longdash"
	StrokeDash     = "dash"
	StrokeDot      = "dot"
)

const (
	AlignLeft    = "left"
	AlignCenter  = "center"
	AlignRight   = "right"
	AlignJustify = "justify"
)

// Cara angka di dalam teks ditampilkan.
//
// FormatPlain adalah bawaannya dan berarti apa adanya — ia ada sebagai anggota
// yang disebut, bukan hanya sebagai ketiadaan, supaya editor dapat
// mengembalikan teks ke bentuk polos secara eksplisit alih-alih mengosongkan
// field dan berharap bawaannya sama.
const (
	FormatPlain    = "plain"
	FormatGrouped  = "grouped"
	FormatCurrency = "currency"
	FormatPercent  = "percent"
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
	ID string `json:"id"`
	// Title adalah sebutan halaman di dalam editor — daftar halaman, panel
	// thumbnail, dan sejenisnya. Renderer TIDAK menggambarnya: judul yang tampil
	// di atas kertas adalah elemen teks biasa, bukan field ini.
	//
	// Kosong berarti belum diberi judul, dan itu keadaan yang sah. Frontend yang
	// menampilkannya bertanggung jawab menyediakan sebutan cadangan sendiri.
	Title string `json:"title,omitempty"`
	// Hidden mengeluarkan halaman dari hasil cetak, bukan sekadar dari layar.
	// Ekspor melewatinya seluruhnya — termasuk tidak mengunduh asetnya.
	//
	// Locked hanya penanda; backend TIDAK menegakkannya. Ia mencegah kecelakaan di
	// editor, bukan mencegah orang yang berniat, dan menegakkannya di sini berarti
	// menambah jalur penolakan pada jalur penyuntingan yang paling ramai.
	Hidden bool `json:"hidden,omitempty"`
	Locked bool `json:"locked,omitempty"`

	// Background adalah warna latar halaman, digambar sebelum elemen mana pun.
	// Kosong berarti tidak digambar sama sekali — kertasnya sendiri yang terlihat,
	// dan itu tidak sama dengan putih: PDF di atas kertas berwarna akan berbeda.
	//
	// Ada di halaman, bukan sebagai elemen rect seukuran halaman, karena latar
	// bukan sesuatu yang boleh ikut terpilih, tergeser, atau terhapus saat orang
	// menekan pilih-semua.
	Background string `json:"background,omitempty"`

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

	// Locked dan GroupID tidak memengaruhi gambar sama sekali; renderer
	// mengabaikan keduanya. Mereka ada di model ini semata-mata supaya editor
	// punya tempat menyimpannya, dan supaya keduanya ikut tersalin ketika elemen
	// dipindahkan atau dokumen digandakan.
	//
	// GroupID datar, tidak bersarang. Grup di dalam grup butuh pohon, dan itu
	// keputusan yang belum diambil.
	//
	// PERHATIAN: element.update mengganti elemen SELURUHNYA. Klien yang tidak
	// mengirim balik kedua field ini akan membuka kuncinya dan mengeluarkannya
	// dari grup, diam-diam.
	Locked  bool   `json:"locked,omitempty"`
	GroupID string `json:"groupId,omitempty"`

	// Rotation adalah putaran elemen dalam derajat, SEARAH jarum jam, terhadap
	// titik tengah kotaknya. Sama seperti transform: rotate() di CSS dengan
	// transform-origin: center — disamakan supaya frontend tidak perlu
	// menerjemahkan apa pun.
	//
	// Berlaku untuk semua jenis elemen. Pada garis, titik tengah kotak kebetulan
	// juga titik tengah garisnya, karena w dan h di sana adalah simpangan ujung
	// terhadap pangkal.
	//
	// Nol berarti tegak. Nilainya tidak dinormalkan: 370 dan 10 menghasilkan
	// gambar yang sama, dan memaksa salah satunya hanya akan membuat nilai yang
	// dikirim klien berbeda dari yang dikembalikan.
	Rotation float64 `json:"rotation,omitempty"`

	// Opacity adalah ketembusan elemen, 0 tembus pandang sampai 1 pekat.
	//
	// POINTER, dan itu disengaja. Nol adalah nilai yang sah — elemen yang
	// sepenuhnya tembus pandang — sehingga ia tidak dapat dipakai untuk menandai
	// "tidak disebutkan" seperti properti lain di struct ini. nil berarti tidak
	// disebutkan, dan yang berlaku DefaultOpacity.
	//
	// Karena ini tipe rujukan, Content.Clone WAJIB menyalinnya secara eksplisit.
	// Lihat catatan di sana.
	Opacity *float64 `json:"opacity,omitempty"`

	// Properti teks.
	Text       string  `json:"text,omitempty"`
	FontFamily string  `json:"fontFamily,omitempty"`
	FontSize   float64 `json:"fontSize,omitempty"`
	FontWeight int     `json:"fontWeight,omitempty"`
	FontStyle  string  `json:"fontStyle,omitempty"`
	Color      string  `json:"color,omitempty"`
	Align      string  `json:"align,omitempty"`
	// Underline dan Strikethrough berlaku pada SELURUH isi elemen, bukan sebagian.
	// Menggarisbawahi satu kata di tengah paragraf menuntut teks kaya, dan itu
	// belum ada — lihat catatan rich text di dokumen arsitektur.
	Underline     bool `json:"underline,omitempty"`
	Strikethrough bool `json:"strikethrough,omitempty"`
	// VerticalAlign meratakan seluruh blok teks di dalam kotaknya. Bawaan "top".
	VerticalAlign string `json:"verticalAlign,omitempty"`

	// Format menandai teks ini sebagai angka yang ditampilkan dengan aturan
	// tertentu. Bawaan "plain", yang berarti apa adanya.
	//
	// PENANDA, BUKAN PERINTAH. Text sudah berisi hasil formatnya — frontend
	// mengirim "Rp 1.234.567" beserta format "currency", bukan "1234567" —
	// sehingga renderer menggambar Text apa adanya dan tidak pernah menafsirkan
	// field ini. Ia dibawa supaya editor tahu aturan apa yang sedang berlaku
	// pada sebuah teks, dan supaya ia ikut tersalin saat elemen digandakan.
	//
	// Dua sebab, keduanya soal layar dan cetak yang harus sama. Pertama, aturan
	// angka — pemisah ribuan, jumlah desimal, lambang mata uang, arti
	// "percent" — hanya hidup di satu tempat; dua implementasi yang menerjemahkan
	// angka yang sama akan berselisih suatu hari tanpa satu pun galat. Kedua,
	// lebar teks menentukan pemenggalan baris, dan tinggi kotak teks di editor
	// diturunkan dari isinya: backend yang menggambar string berbeda dari yang
	// diukur frontend menghasilkan kotak yang tidak lagi pas.
	Format string `json:"format,omitempty"`

	// TIDAK ADA sisi dalam di sini, dan itu keputusan yang sengaja diambil
	// setelah sempat ada. Kotak elemen teks tidak pernah digambar — tanpa isi
	// maupun garis tepi — sehingga menggeser teks ke dalam dengan sisi dalam
	// menghasilkan piksel yang persis sama dengan mengecilkan kotaknya lalu
	// memindahkan pangkalnya. Dua cara menyatakan satu hal, dan yang kedua sudah
	// dimiliki setiap elemen.
	LineHeight    float64 `json:"lineHeight,omitempty"`
	LetterSpacing float64 `json:"letterSpacing,omitempty"`

	// Properti bidang dan garis. Fill dan Stroke kosong berarti tidak digambar,
	// bukan hitam — sehingga kotak tanpa isi maupun garis tepi dapat dinyatakan.
	Fill        string  `json:"fill,omitempty"`
	Stroke      string  `json:"stroke,omitempty"`
	StrokeWidth float64 `json:"strokeWidth,omitempty"`
	// Radius membulatkan sudut, pada rect maupun gambar. Yang melebihi separuh
	// sisi terpendek dijepit, sama seperti border-radius di browser.
	//
	// Pada gambar ia MEMOTONG, bukan menggambar bingkai: sudut yang terbuang
	// benar-benar hilang, termasuk pada fit "cover" yang gambarnya memang lebih
	// besar daripada kotaknya.
	Radius float64 `json:"radius,omitempty"`
	// StrokeStyle hanya berarti bila StrokeWidth > 0. Kosong berarti solid.
	StrokeStyle string `json:"strokeStyle,omitempty"`

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

// Clone menyalin isi dokumen sedalam-dalamnya.
//
// BENAR HANYA SELAMA Element tidak memuat tipe rujukan. Seluruh fieldnya
// sekarang string, angka, dan boolean, sehingga menyalin nilainya sudah menyalin
// isinya. Menambahkan slice, map, atau pointer ke Element membuat salinan ini
// diam-diam menjadi dangkal — dan gejalanya bukan galat melainkan riwayat undo
// yang ikut berubah ketika dokumennya disunting.
//
// Dipakai riwayat undo, yang menyimpan keadaan sebelum tiap kelompok perubahan.
func (c *Content) Clone() *Content {
	out := &Content{Pages: make([]Page, len(c.Pages))}
	for i := range c.Pages {
		out.Pages[i] = c.Pages[i]
		out.Pages[i].Elements = make([]Element, len(c.Pages[i].Elements))
		copy(out.Pages[i].Elements, c.Pages[i].Elements)

		// copy di atas menyalin nilai field satu per satu, sehingga field
		// bertipe RUJUKAN berakhir menunjuk objek yang sama dengan aslinya.
		// Setiap field semacam itu harus disalin sendiri di bawah ini.
		//
		// Melewatkannya tidak menghasilkan galat apa pun: cuplikan undo hanya
		// akan diam-diam ikut berubah bersama isi yang hidup, dan Ctrl+Z
		// mengembalikan keadaan yang sudah tidak ada lagi.
		for j := range out.Pages[i].Elements {
			if opacity := out.Pages[i].Elements[j].Opacity; opacity != nil {
				salinan := *opacity
				out.Pages[i].Elements[j].Opacity = &salinan
			}
		}
	}

	return out
}

// IsEmpty menandai dokumen yang belum punya halaman sama sekali — kandidat untuk
// diisi benih.
func (c *Content) IsEmpty() bool {
	return len(c.Pages) == 0
}
