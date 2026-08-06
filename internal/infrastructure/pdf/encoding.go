package pdf

import "strings"

// Berkas ini mengubah teks UTF-8 menjadi pengkodean yang dipahami font.
//
// Font yang disematkan dari berkas TrueType memakai Unicode dan tidak
// membutuhkan apa pun di sini. Keluarga inti PDF berbeda: ia memakai Windows-1252,
// satu byte per huruf. Menyerahkan byte UTF-8 apa adanya ke sana membuat setiap
// huruf di luar ASCII pecah menjadi beberapa huruf sampah — tanda pisah "—"
// muncul sebagai "â€", dan tidak ada satu pun error yang memberi tahu.

// cp1252High memetakan rentang 0x80–0x9F pada Windows-1252, satu-satunya bagian
// yang menyimpang dari Latin-1. Sisanya lurus: 0x00–0x7F sama dengan ASCII, dan
// 0xA0–0xFF sama dengan Latin-1.
var cp1252High = map[rune]byte{
	'€': 0x80, '‚': 0x82, 'ƒ': 0x83, '„': 0x84,
	'…': 0x85, '†': 0x86, '‡': 0x87, 'ˆ': 0x88,
	'‰': 0x89, 'Š': 0x8A, '‹': 0x8B, 'Œ': 0x8C,
	'Ž': 0x8E, '‘': 0x91, '’': 0x92, '“': 0x93,
	'”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97,
	'˜': 0x98, '™': 0x99, 'š': 0x9A, '›': 0x9B,
	'œ': 0x9C, 'ž': 0x9E, 'Ÿ': 0x9F,
}

// encodeCP1252 mengubah teks UTF-8 menjadi Windows-1252.
//
// Huruf yang tidak ada dalam Windows-1252 — aksara Tionghoa, Arab, dan sebagian
// besar tanda baca tipografis di luar tabel di atas — menjadi tanda tanya. Itu
// batas nyata keluarga inti, dan alasan lain untuk mendaftarkan berkas font
// sendiri bila dokumen memuat huruf di luar Eropa Barat.
//
// Setiap huruf selalu menjadi tepat satu byte, termasuk yang diganti. Sifat itu
// dipegang oleh pengukuran teks: jumlah huruf pada teks asli sama dengan jumlah
// byte pada hasilnya, sehingga jarak antar huruf tetap dihitung sebanyak yang
// semestinya.
func encodeCP1252(text string) string {
	var builder strings.Builder
	builder.Grow(len(text))

	for _, symbol := range text {
		switch {
		case symbol < 0x80, symbol >= 0xA0 && symbol <= 0xFF:
			builder.WriteByte(byte(symbol))
		default:
			if encoded, ok := cp1252High[symbol]; ok {
				builder.WriteByte(encoded)
				continue
			}
			builder.WriteByte('?')
		}
	}

	return builder.String()
}
