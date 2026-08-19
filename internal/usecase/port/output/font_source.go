package output

// FontIdentity adalah jati diri satu muka huruf, dibaca dari berkasnya sendiri.
//
// Family datang dari tabel nama di dalam font — "Barlow Condensed" lengkap
// dengan spasinya — bukan dari nama berkas. Nama berkas tidak cukup:
// "BarlowCondensed-Bold.ttf" akan menghasilkan slug "barlowcondensed",
// sedangkan elemen dokumen mengirim "barlow condensed" yang menghasilkan
// "barlow-condensed". Keduanya tidak akan pernah bertemu.
type FontIdentity struct {
	Family string
	Weight int
	Style  string
}

// FontInspector membaca jati diri sebuah berkas font SEKALIGUS memastikan ia
// dapat disematkan ke PDF.
//
// Keduanya satu operasi karena keduanya menuntut hal yang sama: berkasnya harus
// benar-benar diurai. Memisahkannya berarti mengurai dua kali, dan membuka
// peluang satu berkas dinyatakan sah oleh yang satu lalu ditolak yang lain.
//
// Ada di port karena yang tahu jawabannya adalah pustaka font dan pustaka PDF,
// dan usecase tidak boleh mengimpor keduanya.
type FontInspector interface {
	Inspect(data []byte) (FontIdentity, error)
}
