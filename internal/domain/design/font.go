package design

// FontFamily adalah satu keluarga font beserta seluruh potongan yang benar-benar
// tersedia.
//
// Ada di paket ini, bukan di sisi renderer, karena daftarnya adalah bagian dari
// kosakata model isi dokumen: nilai `fontFamily`, `fontWeight`, dan `fontStyle`
// pada sebuah elemen hanya sah bila ada potongan yang cocok. Editor memakainya
// untuk mengisi pilihan font, sehingga pengguna tidak dapat memilih sesuatu yang
// akan menggagalkan ekspornya sendiri.
type FontFamily struct {
	Name  string
	Faces []FontFace
}

// FontFace adalah satu potongan font: satu ketebalan, satu gaya.
type FontFace struct {
	Weight int
	Style  string
}
