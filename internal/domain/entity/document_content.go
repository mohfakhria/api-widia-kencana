package entity

import "encoding/json"

// DocumentContent adalah isi kanvas satu dokumen beserta nomor revisinya.
//
// Content sengaja disimpan sebagai JSON mentah. Backend belum perlu memahami
// bentuk halaman maupun elemennya, dan memperlakukannya sebagai blob menjamin
// field apa pun yang dikirim frontend kembali utuh saat dibaca — termasuk
// properti visual baru yang belum dikenal backend.
type DocumentContent struct {
	Token   string
	Content json.RawMessage
	Version int64
	// Paper dibawa bersama isinya karena dokumen yang masih kosong perlu diisi
	// benih yang ukurannya mengikuti kertas.
	Paper DocumentPaperSize
}

// DocumentPaperSize adalah ukuran kertas dokumen, secukupnya untuk menempatkan
// elemen. Sengaja bukan DocumentPaper utuh: yang dibutuhkan hanya tiga field ini.
type DocumentPaperSize struct {
	Width  float64
	Height float64
	Unit   string
}
