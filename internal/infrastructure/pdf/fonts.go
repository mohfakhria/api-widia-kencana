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
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// ManifestName adalah berkas yang mendaftarkan keluarga font beserta berkasnya.
// Pendaftaran dibuat eksplisit, bukan hasil pemindaian direktori, supaya nama
// keluarga yang dipakai frontend tidak bergantung pada nama berkas.
const ManifestName = "fonts.json"

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

// faceKey adalah satu potongan font: satu keluarga, satu ketebalan, satu gaya.
type faceKey struct {
	family string
	weight int
	style  string
}

// Fonts adalah kumpulan font yang tersedia bagi renderer.
//
// Seluruh berkas dimuat ke memori sekali saat aplikasi start, bukan dibaca tiap
// ekspor. Satu berkas font berukuran ratusan kilobita dan jumlahnya sedikit,
// sedangkan membacanya berulang kali akan menambah I/O pada jalur yang justru
// diharapkan cepat.
type Fonts struct {
	faces map[faceKey][]byte
}

type manifest struct {
	Families []manifestFamily `json:"families"`
}

type manifestFamily struct {
	Name  string         `json:"name"`
	Faces []manifestFace `json:"faces"`
}

type manifestFace struct {
	Weight int    `json:"weight"`
	Style  string `json:"style"`
	File   string `json:"file"`
}

// LoadFonts membaca manifes dan seluruh berkas font di dalamnya.
//
// Direktori yang tidak ada bukan error: aplikasi tetap berjalan dengan keluarga
// inti saja. Yang menjadi error adalah manifes yang ada tetapi cacat, atau berkas
// yang disebut manifes tetapi tidak ditemukan — keduanya berarti niat yang tidak
// terpenuhi, dan mendiamkannya akan muncul belakangan sebagai ekspor yang
// hurufnya berbeda dari layar.
func LoadFonts(dir string) (*Fonts, error) {
	fonts := &Fonts{faces: make(map[faceKey][]byte)}

	if dir == "" {
		return fonts, nil
	}

	raw, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if errors.Is(err, os.ErrNotExist) {
		return fonts, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read font manifest: %w", err)
	}

	var parsed manifest
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse font manifest: %w", err)
	}

	for _, family := range parsed.Families {
		name := strings.ToLower(strings.TrimSpace(family.Name))
		if name == "" {
			return nil, errors.New("font manifest has a family without a name")
		}
		if name == CoreFamily {
			return nil, fmt.Errorf("font family %q is reserved for the built-in core font", CoreFamily)
		}

		for _, face := range family.Faces {
			key, err := newFaceKey(name, face)
			if err != nil {
				return nil, err
			}

			data, err := os.ReadFile(filepath.Join(dir, face.File))
			if err != nil {
				return nil, fmt.Errorf("read font file for %s %d %s: %w", name, key.weight, key.style, err)
			}
			fonts.faces[key] = data
		}
	}

	return fonts, nil
}

func newFaceKey(family string, face manifestFace) (faceKey, error) {
	if face.Weight < 100 || face.Weight > 900 || face.Weight%100 != 0 {
		return faceKey{}, fmt.Errorf("font family %q has face weight %d, expected a multiple of 100 between 100 and 900", family, face.Weight)
	}

	style := strings.ToLower(strings.TrimSpace(face.Style))
	if style == "" {
		style = design.FontStyleNormal
	}
	if style != design.FontStyleNormal && style != design.FontStyleItalic {
		return faceKey{}, fmt.Errorf("font family %q has face style %q, expected normal or italic", family, style)
	}
	if strings.TrimSpace(face.File) == "" {
		return faceKey{}, fmt.Errorf("font family %q has a face without a file", family)
	}

	return faceKey{family: family, weight: face.Weight, style: style}, nil
}

// Families menyebut seluruh keluarga yang tersedia, untuk dicatat saat start
// supaya ketiadaan font terlihat di log dan bukan baru terungkap saat ada yang
// mencoba mengekspor.
func (f *Fonts) Families() []string {
	catalog := f.Catalog()
	names := make([]string, 0, len(catalog))
	for _, family := range catalog {
		names = append(names, family.Name)
	}

	return names
}

// coreFaces adalah potongan yang dimiliki keluarga inti PDF. Hanya tegak dan
// tebal; ketebalan lain ditolak saat menggambar alih-alih dibulatkan.
func coreFaces() []design.FontFace {
	return []design.FontFace{
		{Weight: 400, Style: design.FontStyleNormal},
		{Weight: 400, Style: design.FontStyleItalic},
		{Weight: 700, Style: design.FontStyleNormal},
		{Weight: 700, Style: design.FontStyleItalic},
	}
}

// Catalog menyebut setiap keluarga beserta potongan yang benar-benar terdaftar.
//
// Urutannya dibuat pasti — keluarga menurut abjad, potongan menurut ketebalan
// lalu gaya — supaya pilihan font di editor tidak berubah-ubah urutannya tiap
// kali halaman dimuat.
func (f *Fonts) Catalog() []design.FontFamily {
	grouped := map[string][]design.FontFace{CoreFamily: coreFaces()}

	for key := range f.faces {
		grouped[key.family] = append(grouped[key.family], design.FontFace{
			Weight: key.weight,
			Style:  key.style,
		})
	}

	names := slices.Sorted(maps.Keys(grouped))
	catalog := make([]design.FontFamily, 0, len(names))

	for _, name := range names {
		faces := grouped[name]
		slices.SortFunc(faces, func(a, b design.FontFace) int {
			if a.Weight != b.Weight {
				return a.Weight - b.Weight
			}

			return strings.Compare(a.Style, b.Style)
		})
		catalog = append(catalog, design.FontFamily{Name: name, Faces: faces})
	}

	return catalog
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
	asked := faceKey{family: family, weight: weight, style: style}

	if family != CoreFamily {
		if data, ok := f.faces[asked]; ok {
			return resolution{used: asked, data: data}
		}
		if nearest, ok := f.nearestWeight(family, weight, style); ok {
			return resolution{used: nearest, data: f.faces[nearest]}
		}
	}

	core := faceKey{family: CoreFamily, weight: snapCoreWeight(weight), style: style}

	return resolution{used: core, core: true}
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
