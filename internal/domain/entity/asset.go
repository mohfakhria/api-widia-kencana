package entity

import (
	"path"
	"time"
)

type Asset struct {
	ID         int64
	Token      string
	Bucket     string
	ObjectName string
	// Key adalah nama slot yang isinya dapat diganti. Nil untuk aset biasa.
	Key                *string
	OriginalFilename   string
	StoredFilename     string
	MimeType           string
	Extension          string
	Size               int64
	ETag               string
	ChecksumSHA256     *string
	Status             string
	UploadMethod       string
	IsPrivate          bool
	UploadedBy         *int64
	PresignedExpiresAt *time.Time
	UploadedAt         *time.Time
	FailedAt           *time.Time
	FailureCode        *string
	FailureMessage     *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Group mengembalikan kelompok aset dari nama objeknya.
//
// Diturunkan, bukan disimpan: kelompoknya ADALAH folder tempat objeknya
// mendarat, dan kolom kedua yang menyebut hal sama pasti berselisih suatu hari.
// Pola yang sama dipakai font, yang bahkan tidak punya tabel sama sekali.
//
// Seluruh segmen sebelum nama berkas, sehingga kelompok bertingkat dua seperti
// images/brand terbaca utuh — dan yang kelak lebih dalam ikut terbaca tanpa
// perubahan di sini.
func (a Asset) Group() string {
	group := path.Dir(a.ObjectName)
	if group == "." || group == "/" {
		return ""
	}

	return group
}
