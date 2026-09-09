package entity

import "time"

type Project struct {
	ID        int64
	Name      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Variables adalah kantong nilai tingkat proyek yang diisi orang, dan
	// project_value tinggal di dalamnya. TIDAK diturunkan dari dokumen — lihat
	// alasan lengkapnya di migration/projects.sql.
	Variables map[string]any

	// Keempatnya hanya terisi saat mengambil SATU proyek, bukan pada daftar.
	// Daftar dipakai pemilih di layar, dan menyertakan peserta beserta lampiran
	// di sana berarti dua kueri tambahan per baris untuk data yang tidak dilihat
	// siapa pun sampai satu proyek benar-benar dibuka.
	Companies   []ProjectCompany
	Attachments []ProjectAttachment
	Documents   []ProjectDocument
	Milestones  []ProjectMilestone
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

// ProjectDocument adalah dokumen yang dibuat di editor dan dikaitkan ke sebuah
// proyek.
//
// BEDA TAJAM dari ProjectAttachment, dan perbedaannya menentukan cara
// menghapusnya. Lampiran adalah berkas yang DITERIMA — scan PO, foto lapangan —
// yang tidak punya hidup di luar proyeknya, sehingga melepasnya ikut membuang
// berkasnya. Dokumen DIBUAT sendiri, punya riwayat, induk, dan isinya sendiri;
// melepasnya dari proyek hanya memutus kaitannya.
type ProjectDocument struct {
	ID         string
	ProjectID  int64
	DocumentID int64
	Note       string
	CreatedAt  time.Time
	UpdatedAt  time.Time

	// Document dibawa serta saat dibaca. Token yang dipakai pemanggil untuk
	// membukanya — id numeriknya tidak pernah keluar.
	Document *Document
}

// ProjectMilestone adalah satu tonggak yang SUDAH tercapai, beserta tanggalnya.
//
// Barisnya hanya ada ketika tercapai — ketiadaannya itulah "belum". Tidak ada
// kolom sudah/belum, dan tidak ada penyemaian saat proyek dibuat.
//
// Milestone adalah TEKS BEBAS yang dinormalkan, bukan kosakata tertutup seperti
// role dan kind. Akibatnya progress bukan angka persen melainkan linimasa —
// "4 dari 6" hanya berarti bila himpunannya tetap. Lihat catatan lengkapnya di
// migration/project_milestones.sql.
type ProjectMilestone struct {
	ID        string
	ProjectID int64
	Milestone string
	ReachedAt time.Time
	Note      string
	CreatedAt time.Time
	UpdatedAt time.Time
}
