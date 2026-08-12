package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// RequireRole hanya meneruskan permintaan dari peran yang disebutkan.
//
// WAJIB dipasang SESUDAH AuthRequired. Ia membaca userID yang ditaruh middleware
// itu, dan urutan yang terbalik membuatnya menolak semua permintaan — keras,
// tetapi ke arah yang benar: salah urut menjadi terlihat pada permintaan
// pertama, bukan menjadi lubang yang diam.
//
// Perannya dibaca dari SESI, bukan dari database maupun dari token. AuthRequired
// sudah menaruhnya di context, dan sesi itu sendiri disegarkan tiap refresh.
// Middleware ini karena itu tidak menyentuh database sama sekali.
//
// Batas yang menyertainya: peran yang diubah lewat SQL baru berlaku pada refresh
// berikutnya — paling lama 24 jam. Menghentikan sesi yang liar tidak menunggu
// itu, karena membunuh sesinya berlaku seketika.
//
// Daftar IZIN, bukan larangan. Peran yang tidak disebutkan ditolak, sehingga
// peran baru — dan salah ketik pada kolom yang diisi manual — tertutup sampai
// ada yang memutuskan sebaliknya.
func RequireRole(allowed ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Daftar kosong berarti tidak ada yang boleh, bukan semua boleh. Itu
		// selalu kekeliruan pemasangan, dan menolak semuanya membuatnya ketahuan
		// segera alih-alih meloloskan siapa saja diam-diam.
		if len(allowed) == 0 {
			abortRole(c, http.StatusForbidden, "Access is not permitted for this role")
			return
		}

		// Tidak ada identitas berarti AuthRequired belum berjalan; peran kosong
		// berarti sesinya tidak membawanya. Keduanya berarti wewenangnya tidak
		// dapat dipastikan, dan meneruskan permintaan yang wewenangnya tidak
		// pasti adalah hal yang justru middleware ini adakan untuk dicegah.
		user, ok := CurrentUser(c)
		if !ok || user.Role == "" {
			abortRole(c, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		if !roleAllowed(user.Role, allowed) {
			abortRole(c, http.StatusForbidden, "Access is not permitted for this role")
			return
		}

		c.Next()
	}
}

// roleAllowed membandingkan tanpa peduli besar-kecil huruf.
//
// Kolomnya diisi manual lewat SQL, sehingga "AI-Agent" adalah salah ketik yang
// wajar. Besar-kecil huruf di sini bukan batas keamanan — tidak ada satu jalur
// pun yang membiarkan seseorang menyetel perannya sendiri — jadi memaafkannya
// menutup satu kelas kekeliruan operator tanpa membuka apa pun.
func roleAllowed(role string, allowed []string) bool {
	for _, candidate := range allowed {
		if strings.EqualFold(role, candidate) {
			return true
		}
	}

	return false
}

// abortRole memakai amplop yang sama dengan AuthRequired, bukan dto.Error.
// Paket middleware sengaja tidak bergantung pada paket dto — lihat auth.go, yang
// menyusun amplopnya dengan cara yang sama.
func abortRole(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"status":  "error",
		"message": message,
	})
}
