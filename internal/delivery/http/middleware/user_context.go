package middleware

import "github.com/gin-gonic/gin"

// userContextKey adalah SATU kunci untuk seluruh identitas pemanggil.
//
// Sebelumnya kelimanya ditaruh sebagai kunci terpisah, dan itu punya dua cacat
// yang keduanya diam. Pertama, tiap pembaca mengetik ulang nama kuncinya:
// "userRole" yang salah ketik menjadi "userole" tidak menghasilkan galat apa
// pun, hanya string kosong — dan pada pemeriksaan wewenang, string kosong yang
// diam adalah kegagalan yang paling mahal. Kedua, tidak ada yang menjamin
// kelimanya berasal dari sesi yang sama; empat dapat terisi sementara satu
// tertinggal, dan tidak ada satu tempat pun yang dapat memeriksanya.
//
// Satu struct menutup keduanya: ia terisi sekali, seluruhnya atau tidak sama
// sekali, dan salah ketik pada nama field menjadi galat kompilasi.
const userContextKey = "userContext"

// UserContext adalah identitas pemanggil untuk satu permintaan.
//
// Seluruhnya datang dari SESI, bukan dari token maupun database — token hanya
// menunjuk sesi, dan sesi menyimpan salinan yang disegarkan tiap refresh. Tidak
// ada satu field pun di sini yang menuntut kueri.
//
// UserID bertipe int64, sama dengan kolomnya di database dan sama dengan
// entity.User.ID — tidak ada satu lapisan pun di antaranya yang mengarang
// bentuk sendiri.
//
// Bentuk kawat memang string: presence, cursor, dan selection mengirim id
// pengguna sebagai string JSON, dan frontend menurunkan warna avatar darinya.
// Itu keputusan SERIALISASI, dan tempatnya di lapisan DTO — bukan alasan bagi
// tipe internal untuk ikut berubah. Konversinya karena itu terkumpul di batas
// penyandian, tempat satu-satunya ia memang dibutuhkan.
type UserContext struct {
	SessionID string
	UserID    int64
	Name      string
	Email     string
	Role      string
}

// CurrentUser mengembalikan identitas pemanggil beserta penanda ada-tidaknya.
//
// false berarti permintaan ini tidak melewati AuthRequired. Pemanggil WAJIB
// memeriksanya alih-alih memakai nilai nolnya: UserContext kosong punya Role
// kosong, dan peran kosong yang lolos diam-diam adalah lubang wewenang.
func CurrentUser(c *gin.Context) (UserContext, bool) {
	value, exists := c.Get(userContextKey)
	if !exists {
		return UserContext{}, false
	}

	user, ok := value.(UserContext)

	return user, ok
}

// setCurrentUser hanya dipanggil AuthRequired. Tidak diekspor supaya identitas
// tidak dapat dipalsukan dari lapisan mana pun di belakangnya.
func setCurrentUser(c *gin.Context, user UserContext) {
	c.Set(userContextKey, user)
}
