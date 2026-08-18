package input

import "context"

type FontUseCase interface {
	// Register mengambil, memvalidasi, lalu menyimpan satu keluarga font.
	//
	// Mengembalikan hasil PER MUKA HURUF, bukan satu galat untuk seluruh
	// permintaan. Satu bobot yang tidak tersedia di sumbernya adalah kejadian
	// biasa — banyak keluarga tidak menyediakan seluruh bobot statik — dan
	// menggagalkan sembilan muka huruf lain karenanya jauh lebih merugikan
	// daripada melaporkan yang mana yang tidak dapat diambil.
	Register(ctx context.Context, cmd RegisterFontCommand) ([]FontFaceResult, error)
}

type RegisterFontCommand struct {
	Family  string
	Weights []int
	// Styles kosong berarti hanya tegak. Italic diminta eksplisit karena
	// sebagian besar keluarga dipakai tanpa italic sama sekali, dan mengambil
	// yang tidak dipakai hanya menambah berkas yang tidak pernah tersentuh.
	Styles []string
}

// FontFaceResult melaporkan nasib satu muka huruf.
type FontFaceResult struct {
	Family     string
	Weight     int
	Style      string
	ObjectName string
	Size       int64
	Stored     bool
	// Reason terisi hanya bila Stored bernilai false, dan berisi kalimat yang
	// aman ditampilkan kepada pengguna.
	Reason string
}
