package entity

import "time"

type Project struct {
	ID        int64
	Name      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Keduanya hanya terisi saat mengambil SATU proyek, bukan pada daftar.
	// Daftar dipakai pemilih di layar, dan menyertakan peserta beserta lampiran
	// di sana berarti dua kueri tambahan per baris untuk data yang tidak dilihat
	// siapa pun sampai satu proyek benar-benar dibuka.
	Companies   []ProjectCompany
	Attachments []ProjectAttachment
}

// ProjectCompany adalah keterlibatan satu perusahaan dalam satu proyek.
//
// Satu perusahaan boleh memegang DUA peran — pelanggan yang sekaligus memasok
// sebagian barangnya bukan hal aneh — tetapi tidak boleh didaftarkan dua kali
// pada peran yang sama.
type ProjectCompany struct {
	ID        string
	ProjectID int64
	CompanyID string
	Role      string
	Note      string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Company dibawa serta saat dibaca supaya pemanggil tidak perlu mengambilnya
	// satu per satu; ia tidak pernah ikut saat menulis.
	Company *Company
}

// ProjectAttachment adalah berkas yang ditautkan ke sebuah proyek.
//
// Berkasnya sendiri tinggal di object storage sebagai aset biasa — tabel ini
// hanya menautkannya, beserta keterangan yang tidak dimiliki aset: jenisnya,
// milik siapa, dan catatannya.
type ProjectAttachment struct {
	ID        string
	ProjectID int64
	AssetID   int64
	Kind      string
	CompanyID *string
	Note      string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Asset dan Company dibawa serta saat dibaca. AssetToken yang dipakai
	// pemanggil untuk membuka berkasnya — id numeriknya tidak pernah keluar.
	Asset   *Asset
	Company *Company
}
