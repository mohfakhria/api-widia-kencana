package entity

// Peran yang dikenal. Nilainya sama persis dengan yang tersimpan di kolom
// users.role, yang diisi manual lewat SQL — tidak ada satu pun jalur di
// aplikasi yang menulis ke tabel itu.
//
// Karena diisi tangan, daftar ini bukan penegak melainkan penyebut: database
// tetap menerima nilai apa pun. Yang menjaga adalah pemakainya, dan pemakainya
// memakai daftar IZIN — peran yang tidak disebutkan ditolak, termasuk salah
// ketik. Itu arah kegagalan yang benar untuk pemeriksaan wewenang.
const (
	// RoleSuperadmin adalah manusia yang memakai aplikasi ini. Seluruh baris yang
	// ada saat ini berperan itu.
	//
	// Perhatikan bahwa kolomnya berbawaan 'user' — nilai yang TIDAK dikenal di
	// sini dan tidak dimiliki satu baris pun. Baris yang disisipkan tanpa
	// menyebut perannya karena itu lahir tanpa wewenang apa pun, dan ditolak
	// setiap penjaga peran. Arahnya benar, tetapi bawaannya menyesatkan.
	RoleSuperadmin = "superadmin"

	// RoleAIAgent adalah agent yang menyunting lewat kode, bukan lewat layar.
	//
	// Ia user sungguhan dengan barisnya sendiri di users, dan di dalam document
	// design ia penyunting penuh — termasuk undo dan redo, karena undo di sini
	// memang bersifat dokumen dan bukan per orang. Yang dibatasi hanya rute HTTP
	// di luar penyuntingan: ia boleh mengubah ISI dokumen, tidak boleh
	// menentukan dokumen mana yang ada, namanya apa, atau milik proyek mana.
	RoleAIAgent = "ai-agent"
)

type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
	Role         string
}
