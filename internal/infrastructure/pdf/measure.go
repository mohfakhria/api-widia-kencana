package pdf

import (
	"context"

	"github.com/go-pdf/fpdf"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// Pengukuran teks tanpa menggambar apa pun.
//
// Ada karena penyusun dokumen yang bukan manusia — server MCP, skrip, atau apa
// pun yang memancarkan elemen — tidak dapat melihat hasilnya. Manusia menaruh
// elemen berikutnya dengan melihat di mana yang sebelumnya berakhir; penyusun
// program harus menghitungnya, dan aturan pemenggalan barisnya ada di dalam
// backend ini.
//
// Yang membuatnya dapat dipercaya: ia memanggil layoutText, jalur yang sama
// persis dengan yang dipakai menggambar. Tingginya karena itu tidak dapat
// berbeda dari yang tercetak.

// measurePageSide adalah ukuran halaman semu untuk dokumen pengukur.
//
// Tidak pernah ada halaman yang ditambahkan dan tidak satu pun perintah gambar
// dikeluarkan, jadi angkanya tidak memengaruhi hasil apa pun. fpdf tetap menuntut
// ukuran yang masuk akal saat dibuat.
const measurePageSide = 1000

// MeasureText menghitung tinggi, lebar, dan jumlah baris tiap elemen teks.
//
// Elemen diukur satu per satu dan tidak saling memengaruhi. Yang bukan teks
// menghasilkan pengukuran nol — penyaringannya urusan pemanggil, dan menolaknya
// di sini akan membuat satu elemen keliru menggagalkan seluruh permintaan.
func (r *Renderer) MeasureText(ctx context.Context, elements []design.Element) ([]design.TextMeasurement, error) {
	// Dokumen fpdf sekali pakai, hanya sebagai tempat font terpasang dan lebar
	// glif dapat ditanyakan. Tidak pernah menghasilkan berkas.
	doc := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "pt",
		Size:           fpdf.SizeType{Wd: measurePageSide, Ht: measurePageSide},
	})

	c := &canvas{
		pdf:              doc,
		fonts:            r.fonts,
		registeredFonts:  make(map[faceKey]selectedFont),
		registeredImages: make(map[string]imageRegistration),
		substitutions:    make(map[substitution]int),
	}

	out := make([]design.TextMeasurement, 0, len(elements))
	for index := range elements {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		element := &elements[index]
		if element.Type != design.ElementText {
			out = append(out, design.TextMeasurement{ID: element.ID})
			continue
		}

		lines, _, err := c.layoutText(element)
		if err != nil {
			return nil, err
		}

		// Tinggi memakai jarak antar baris, bukan tinggi huruf: itulah tinggi
		// kotak yang benar-benar dibutuhkan teks ini, dan itulah yang hendak
		// dipasang pemanggil ke field h. Sama dengan tinggi blok pada CSS.
		step := element.ResolvedFontSize() * element.ResolvedLineHeight()

		widest := 0.0
		for _, line := range lines {
			if width := c.measure(line, element.LetterSpacing); width > widest {
				widest = width
			}
		}

		out = append(out, design.TextMeasurement{
			ID:     element.ID,
			Lines:  len(lines),
			Height: float64(len(lines)) * step,
			Width:  widest,
		})
	}

	// Penggantian font SENGAJA tidak dilaporkan di sini. Pengukuran adalah
	// pertanyaan yang boleh diajukan ratusan kali saat menyusun satu dokumen, dan
	// memperingatkan pada tiap panggilan akan menenggelamkan log. Penggantian yang
	// sama tetap tercatat saat dokumennya benar-benar dicetak, yaitu saat ia
	// berakibat pada sesuatu.

	return out, nil
}
