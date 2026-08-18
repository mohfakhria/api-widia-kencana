package usecase

import (
	"fmt"
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
