package usecase

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/domain/design"
)

// FontScope adalah prefix objek untuk seluruh berkas font di object storage.
const FontScope = "fonts"

// FontObjectName menyusun nama objek dari tiga sifat yang dibawa elemen.
//
// Ini inti pilihan rancangan: nama objeknya FUNGSI MURNI dari family, bobot, dan
// style — bukan nama acak yang harus dicari di tabel. Elemen dokumen tidak
// membawa token font, hanya ketiga sifat itu, sehingga jembatan dari elemen ke
// berkas harus dapat dihitung. Aset gambar bisa memakai nama acak justru karena
// elemennya membawa assetToken; font tidak punya padanan itu.
//
//	fonts/barlow/400-normal.ttf
//	fonts/barlow-condensed/700-normal.ttf
//	fonts/ibm-plex-mono/500-normal.ttf
//
// Akibat yang disengaja: mendaftarkan ulang muka huruf yang sama MENIMPA objek
// yang sama. Tidak ada baris kembar yang perlu didamaikan, dan "perbarui font
// ini" tidak butuh operasi tersendiri.
func FontObjectName(family string, weight int, style string) string {
	return fmt.Sprintf("%s/%s/%d-%s.ttf", FontScope, FontFamilySlug(family), weight, normalizeFontStyle(style))
}

// FontFamilySlug mengubah nama keluarga menjadi satu segmen path yang aman.
//
// Spasi menjadi tanda hubung, dan segalanya di luar huruf-angka-hubung dibuang.
// "Barlow Condensed" dan "barlow condensed" karenanya menunjuk objek yang sama —
// sifat yang wajib, karena elemen mengirimkan nama yang sudah dihuruf-kecilkan
// sementara pendaftaran datang dari manusia yang mengetik apa adanya.
func FontFamilySlug(family string) string {
	var builder strings.Builder
	builder.Grow(len(family))

	for _, symbol := range strings.ToLower(strings.TrimSpace(family)) {
		switch {
		case symbol >= 'a' && symbol <= 'z', symbol >= '0' && symbol <= '9':
			builder.WriteRune(symbol)
		case symbol == ' ', symbol == '-', symbol == '_':
			// Pemisah apa pun menjadi satu tanda hubung, dan yang berturut-turut
			// tidak menghasilkan hubung ganda — supaya "IBM  Plex_Mono" dan
			// "ibm plex mono" tetap menunjuk objek yang sama.
			if builder.Len() > 0 && !strings.HasSuffix(builder.String(), "-") {
				builder.WriteByte('-')
			}
		}
	}

	return strings.Trim(builder.String(), "-")
}

func normalizeFontStyle(style string) string {
	if strings.EqualFold(strings.TrimSpace(style), design.FontStyleItalic) {
		return design.FontStyleItalic
	}

	return design.FontStyleNormal
}

// ParseFontObjectName membalik FontObjectName.
//
// Balikan yang TIDAK sempurna, dan itu disengaja: slug memetakan spasi maupun
// tanda hubung ke satu bentuk, sehingga "Barlow Condensed" dan "barlow-condensed"
// menghasilkan objek yang sama dan tidak dapat dibedakan lagi sesudahnya.
//
// Karena itu yang dikembalikan SLUG-nya, bukan tebakan nama aslinya. Slug itulah
// nama kanonik: apa pun yang dikirim elemen akan di-slug dengan aturan yang sama
// sebelum dicari, jadi menyebutkan slug kepada frontend membuat perjalanan
// bolak-baliknya tepat. Menebak "Barlow Condensed" dari "barlow-condensed" justru
// akan salah pada keluarga yang namanya memang bertanda hubung.
func ParseFontObjectName(objectName string) (family string, weight int, style string, ok bool) {
	sisa, cocok := strings.CutPrefix(objectName, FontScope+"/")
	if !cocok {
		return "", 0, "", false
	}

	family, berkas, cocok := strings.Cut(sisa, "/")
	if !cocok || family == "" {
		return "", 0, "", false
	}

	nama, cocok := strings.CutSuffix(berkas, ".ttf")
	if !cocok {
		return "", 0, "", false
	}

	angka, style, cocok := strings.Cut(nama, "-")
	if !cocok {
		return "", 0, "", false
	}

	weight, err := strconv.Atoi(angka)
	if err != nil || weight < 100 || weight > 900 || weight%100 != 0 {
		return "", 0, "", false
	}
	if style != design.FontStyleNormal && style != design.FontStyleItalic {
		return "", 0, "", false
	}

	return family, weight, style, true
}
