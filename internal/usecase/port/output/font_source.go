package output

import "context"

// FontSource mengambil berkas font dari luar, misalnya repositori Google Fonts.
//
// Antarmuka tersendiri, bukan panggilan HTTP langsung di usecase, karena inilah
// satu-satunya tempat sistem ini menghubungi alamat di internet atas perintah
// pengguna. Memisahkannya membuat batas itu terlihat dan dapat diganti — dan
// membuat usecase-nya dapat ditelusuri tanpa jaringan.
type FontSource interface {
	// Fetch mengembalikan berkas TTF untuk satu muka huruf.
	//
	// Family diberikan seperti yang ditulis manusia; implementasinya yang tahu
	// bagaimana memetakannya ke alamat sumber.
	Fetch(ctx context.Context, family string, weight int, style string) ([]byte, error)
}

// FontValidator memeriksa bahwa byte yang diambil benar-benar dapat disematkan
// ke PDF.
//
// Dipisahkan ke port karena yang tahu jawabannya adalah pustaka PDF, dan usecase
// tidak boleh mengimpornya. Pemeriksaan ini WAJIB dilakukan saat mendaftar,
// bukan saat mengekspor: berkas rusak yang lolos akan merusak SETIAP ekspor
// sesudahnya, dan gejalanya muncul jauh dari sebabnya.
type FontValidator interface {
	Validate(data []byte) error
}
