package middleware

import (
	"net/http"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/gin-gonic/gin"
)

// AuthRequired memastikan permintaan datang dari sesi yang masih hidup.
//
// Dua langkah, dan keduanya wajib. Tanda tangan token membuktikan ia terbit
// dari server ini dan belum kedaluwarsa; PENYIMPANAN SESI membuktikan sesinya
// masih berlaku sekarang. Yang pertama saja tidak cukup — token yang sah
// tetaplah token yang sah walau pemiliknya sudah logout.
//
// Inilah yang membuat logout berarti. Selama identitas dititipkan di dalam
// token, tidak ada satu cara pun mencabut access token yang sudah beredar; ia
// berlaku sampai jam terakhirnya habis. Sejak identitas dijawab penyimpanan,
// menghapus sesi langsung mematikannya pada permintaan berikutnya.
//
// Ongkosnya satu lookup per permintaan, dan itu murah SECARA KHUSUS di sini:
// penyimpanannya peta di memori proses yang sama, bukan panggilan jaringan.
// Bila kelak ia dipindahkan ke penyimpanan bersama — yang dibutuhkan begitu API
// berjalan lebih dari satu replica — biayanya berubah dan keputusan ini pantas
// ditinjau ulang.
func AuthRequired(tokenSigner output.TokenSigner, sessions output.SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Missing or invalid Authorization header",
			})
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := tokenSigner.ParseToken(c.Request.Context(), tokenStr)
		if err != nil || claims.TokenType != output.TokenTypeAccess || claims.SessionID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid or expired token",
			})
			return
		}

		// Sesi yang tidak ada, sudah dihapus, atau kedaluwarsa sama-sama berarti
		// token ini tidak lagi mewakili siapa pun. Ketiganya menjawab dengan
		// kalimat yang sama: membedakannya hanya memberi tahu penebak token mana
		// yang pernah ada.
		session, err := sessions.Get(c.Request.Context(), claims.SessionID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid or expired token",
			})
			return
		}

		// Satu nilai, terisi seluruhnya. Handler mana pun setelah ini tidak perlu
		// menyentuh database untuk mengetahui siapa pemanggilnya.
		//
		// Tidak ada penguraian bentuk di mana pun di jalur ini: id pengguna
		// bertipe int64 sejak kolomnya sampai ke sini.
		setCurrentUser(c, UserContext{
			SessionID: claims.SessionID,
			UserID:    session.UserID,
			Name:      session.Name,
			Email:     session.Email,
			Role:      session.Role,
		})
		c.Next()
	}
}
