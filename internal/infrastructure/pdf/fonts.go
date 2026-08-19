// Package pdf menggambar isi dokumen menjadi berkas PDF.
//
// Renderer di sini adalah pasangan dari komponen render di frontend: keduanya
// membaca model yang sama dan harus menghasilkan tata letak yang sama. Karena
// keduanya mesin yang berbeda, kesamaan itu tidak datang sendiri — ia dijaga oleh
// tiga hal yang harus ditegakkan di kedua sisi:
//
//  1. Font dengan LEBAR MAJU yang sama. Bukan sekadar nama keluarga yang sama,
//     tetapi juga bukan harus berkas yang identik: Arial justru diciptakan
//     sebagai pengganti Helvetica dengan lebar maju yang sama persis, dan
//     Liberation Sans serta Nimbus Sans dirancang selebar itu pula. Yang
//     berbeda pada ketiganya bentuk glifnya, bukan lebarnya.
//
//     Yang merusak adalah font yang lebarnya memang lain — DejaVu Sans, yang
//     menjadi pilihan sans-serif bawaan di banyak Linux, dan Roboto di Android.
//     Karena itu frontend harus memaku font-family ke daftar yang lebarnya
//     sepadan, bukan menyerahkannya ke `sans-serif` telanjang.
//
//  2. Kerning dan ligatur dimatikan di frontend, lewat font-kerning: none dan
//     font-feature-settings: "liga" 0. Renderer ini menjumlahkan lebar glif
//     apa adanya tanpa penyesuaian pasangan huruf, jadi browser harus diminta
//     berhenti melakukannya juga.
//
//  3. Aturan pemenggalan baris yang sama: rakus, patah di spasi, tanpa tanda
//     hubung otomatis.
//
// Tanpa ketiganya, hasil ekspor akan mirip tetapi tidak sama — dan perbedaannya
// justru paling terlihat pada dokumen yang paling penting, yang teksnya panjang.
package pdf

import (
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

// CoreFamily adalah satu-satunya keluarga yang selalu tersedia tanpa berkas apa
// pun, karena metriknya melekat pada spesifikasi PDF.
//
// Ia dua hal sekaligus: keluarga bawaan bila tidak ada font yang didaftarkan,
// dan penadah terakhir bagi permintaan yang tidak dapat dipenuhi apa adanya.
// Lihat resolve.
const CoreFamily = "helvetica"

// Metrik vertikal Helvetica, dalam seperseribu em, dari berkas AFM Adobe yang
// mendefinisikan 14 font baku PDF.
//
// Nilainya dituliskan di sini karena fpdf tidak menyertakannya: font inti tidak
// perlu disematkan ke dalam berkas, sehingga pustaka itu hanya menyimpan lebar
// glifnya saja. Penempatan garis dasar tetap membutuhkan keduanya. Keempat gaya
// Helvetica — tegak, tebal, miring, tebal miring — memakai nilai yang sama.
const (
	coreAscent  = 718
	coreDescent = -207
)

// Letak dan tebal garis hiasan, dalam seperseribu em, dari berkas AFM Helvetica.
//
// Dipatok dengan alasan yang sama seperti coreAscent dan coreDescent di atas:
// fpdf menyimpan Up dan Ut pada struct yang tidak diekspor, dan GetFontDesc tidak
// membawanya, sehingga tidak ada cara membacanya lewat API publik.
//
// Untuk keluarga inti nilainya PERSIS benar. Untuk font tersemat ia hampiran —
// kebanyakan font berada di sekitar -75 sampai -150 untuk letak dan 50 sampai 75
// untuk tebal, jadi selisihnya sepersekian titik pada ukuran huruf yang wajar.
// Hampiran yang sama dipakai untuk semua font, sehingga satu dokumen tidak pernah
// menggambar garis bawah di dua ketinggian berbeda.
const (
	underlinePosition  = -100
	underlineThickness = 50
	// Coret memakai kelipatan letak garis bawah, sama seperti yang dilakukan fpdf:
	// empat kali di atas garis dasar jatuh kira-kira di tengah tinggi huruf kecil.
	strikeoutMultiple = 4
)

// faceKey adalah satu potongan font: satu keluarga, satu ketebalan, satu gaya.
type faceKey struct {
	family string
	weight int
	style  string
}

// Fonts adalah kumpulan font yang tersedia bagi SATU penggambaran.
//
// Dulu ia dimuat sekali saat start dari sebuah direktori beserta manifesnya.
// Berkasnya kini tinggal di object storage dan didaftarkan lewat API, sehingga
// menyimpannya di memori proses berarti font yang baru diunggah tidak akan
// terpakai sampai aplikasi dinyalakan ulang — dan pada dua proses yang berjalan
// berdampingan, hasil ekspornya bahkan dapat berbeda.
type Fonts struct {
	faces map[faceKey][]byte
}

// newFonts menyalin muka huruf yang diserahkan pemanggil menjadi peta pencarian.
//
// Yang tidak dapat dipakai DILEWATI, bukan menggagalkan penggambaran: keluarga
// inti tidak boleh ditimpa karena metriknya melekat pada spesifikasi PDF, dan
// bobot atau gaya yang mustahil hanya dapat lahir dari objek yang ditaruh tangan
// manusia ke dalam bucket. Keduanya menyisakan resolve sebagai penjaring, dan ia
// memang dirancang tidak pernah gagal.
func newFonts(faces map[output.FontFace][]byte) *Fonts {
	fonts := &Fonts{faces: make(map[faceKey][]byte, len(faces))}

	for face, data := range faces {
		family := strings.ToLower(strings.TrimSpace(face.Family))
		if family == "" || family == CoreFamily || len(data) == 0 {
			continue
		}
		if face.Weight < 100 || face.Weight > 900 || face.Weight%100 != 0 {
			continue
		}

		style := strings.ToLower(strings.TrimSpace(face.Style))
		if style == "" {
			style = design.FontStyleNormal
		}
		if style != design.FontStyleNormal && style != design.FontStyleItalic {
			continue
		}

		fonts.faces[faceKey{family: family, weight: face.Weight, style: style}] = data
	}

	return fonts
}

// resolution menyebut potongan font yang benar-benar akan dipakai menggambar.
type resolution struct {
	// used adalah potongan yang dipakai; ia boleh berbeda dari yang diminta.
	used faceKey
	// data nil untuk keluarga inti, yang metriknya melekat pada spesifikasi PDF
	// dan tidak perlu disematkan ke dalam berkas.
	data []byte
	core bool
}

// resolve memilih potongan font yang akan dipakai, dan TIDAK PERNAH gagal.
//
// Ini pembalikan keputusan yang disengaja. Sebelumnya permintaan yang tidak
// dapat dipenuhi menggagalkan seluruh ekspor, dengan alasan lebih baik gagal
// jelas daripada berhasil dengan huruf yang salah. Yang membuatnya tidak sepadan
// adalah bentuk kegagalannya: `fontFamily` dan `fontWeight` tidak diperiksa di
// mana pun saat menyunting, sehingga nilai yang mustahil dicetak diterima,
// disiarkan, dan tersimpan — lalu meledak berjam-jam kemudian sebagai dokumen
// yang tidak dapat dicetak sama sekali, jauh dari orang yang menyebabkannya.
//
// Alasan yang sama sudah dipakai untuk gambar yang asetnya hilang: dilewati,
// bukan menggagalkan seluruh ekspor. Lihat output.RenderDocument.
//
// Tiga langkah, dari yang paling setia:
//
//  1. Potongan yang persis diminta.
//  2. Ketebalan terdekat pada keluarga dan gaya yang sama. Menjaga rupa huruf
//     tetap benar; yang bergeser hanya tebalnya.
//  3. Keluarga inti, dengan ketebalan dibulatkan ke 400 atau 700.
//
// Yang dipakai dikembalikan apa adanya lewat resolution.used, supaya pemanggil
// dapat mencatat setiap penggantian. Penggantian yang senyap adalah persis yang
// dikhawatirkan keputusan lama, dan catatan itu jawabannya.
func (f *Fonts) resolve(family string, weight int, style string) resolution {
	if family != CoreFamily {
		if used, ok := f.pick(family, weight, style); ok {
			return resolution{used: used, data: f.faces[used]}
		}

		// Style DILONGGARKAN sebelum keluarga ditinggalkan, dan hanya ke satu
		// arah: italic boleh memakai muka tegak keluarga yang sama, tidak
		// sebaliknya.
		//
		// Sebabnya lebar maju, bukan kerapian. Ketika sebuah keluarga tidak punya
		// muka italic, peramban MENSINTESIS miring dengan memiringkan muka
		// tegaknya — dan oblique sintetis mempertahankan lebar maju muka tegak
		// itu. Memakai muka tegak keluarga yang sama karenanya memecah baris di
		// tempat yang persis sama dengan layar; berpindah ke Helvetica italic
		// mengganti seluruh metriknya, dan kotak yang pas di layar terpotong di
		// cetakan tanpa satu pun galat.
		//
		// Satu arah saja karena peramban juga begitu: tidak ada peramban yang
		// menggambar teks tegak memakai muka italic hanya karena itu satu-satunya
		// yang tersedia.
		if style == design.FontStyleItalic {
			if used, ok := f.pick(family, weight, design.FontStyleNormal); ok {
				return resolution{used: used, data: f.faces[used]}
			}
		}
	}

	core := faceKey{family: CoreFamily, weight: snapCoreWeight(weight), style: style}

	return resolution{used: core, core: true}
}

// pick memilih muka terbaik untuk satu keluarga pada satu style.
//
// Cocok persis lebih dulu, lalu bobot terdekat. Dipisahkan menjadi fungsi
// tersendiri karena resolve memanggilnya dua kali — sekali untuk style yang
// diminta, sekali untuk style yang dilonggarkan — dan dua salinan urutan yang
// sama adalah dua kesempatan untuk berbeda.
func (f *Fonts) pick(family string, weight int, style string) (faceKey, bool) {
	asked := faceKey{family: family, weight: weight, style: style}
	if _, ok := f.faces[asked]; ok {
		return asked, true
	}

	return f.nearestWeight(family, weight, style)
}

// nearestWeight mencari ketebalan terdekat yang benar-benar terdaftar pada satu
// keluarga dan gaya.
//
// Seri diputus ke arah yang lebih ringan, sekadar supaya hasilnya pasti dan tidak
// bergantung pada urutan penelusuran map — yang di Go memang diacak.
func (f *Fonts) nearestWeight(family string, weight int, style string) (faceKey, bool) {
	var (
		best  faceKey
		found bool
	)

	for key := range f.faces {
		if key.family != family || key.style != style {
			continue
		}
		if !found || jarak(key.weight, weight) < jarak(best.weight, weight) ||
			(jarak(key.weight, weight) == jarak(best.weight, weight) && key.weight < best.weight) {
			best, found = key, true
		}
	}

	return best, found
}

// snapCoreWeight membulatkan ke satu-satunya dua ketebalan yang dimiliki font
// inti PDF. Terdekat, dengan seri ke arah yang lebih ringan.
func snapCoreWeight(weight int) int {
	if jarak(weight, 700) < jarak(weight, 400) {
		return 700
	}

	return 400
}

func jarak(a, b int) int {
	if a > b {
		return a - b
	}

	return b - a
}
