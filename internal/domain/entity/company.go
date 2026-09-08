package entity

import "time"

// Company adalah perusahaan: milik sendiri maupun lawan transaksi.
//
// Keduanya duduk di tabel yang sama dan TIDAK ADA kolom yang membedakannya —
// pemilih pelanggan di layar karenanya akan memuat perusahaan sendiri juga, dan
// penyaringnya urusan frontend. Lihat catatan di migration/companies.sql.
type Company struct {
	ID   string
	Code string
	// Name adalah nama pendek untuk daftar dan pencarian; LegalName yang disalin
	// ke dokumen, apa adanya sampai ke tanda bacanya. Keduanya tidak dirakit satu
	// dari yang lain.
	Name            string
	LegalName       string
	CompanyType     string
	Description     string
	EstablishedDate *time.Time
	Address         string
	Email           string
	Phone           string
	Fax             string
	Timezone        string
	Locale          string
	CurrencyCode    string
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// Contacts hanya terisi pada pengambilan satu perusahaan, bukan pada daftar.
	// Daftar perusahaan dipakai pemilih di layar, dan menyertakan seluruh kontak
	// di sana berarti satu kueri tambahan per baris untuk data yang tidak dilihat
	// siapa pun.
	Contacts []CompanyContact
}

// CompanyContact adalah orang di perusahaan itu.
//
// Salutation, Name, dan Position bersama-sama menyusun baris "Attn:" pada
// dokumen — "Bapak Wahyudi — Owner" adalah ketiganya, bukan satu teks.
type CompanyContact struct {
	ID         string
	CompanyID  string
	Salutation string
	Name       string
	Position   string
	Department string
	Email      string
	Phone      string
	Mobile     string
	IsPrimary  bool
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
