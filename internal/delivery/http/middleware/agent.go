package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AgentHeader adalah tempat Widia Agent menaruh kuncinya.
//
// Header tersendiri, bukan Authorization. Kunci ini bukan JWT dan tidak punya
// masa berlaku maupun subjek; menumpangkannya pada Authorization membuat dua
// kredensial yang sifatnya berbeda jauh terlihat sama, dan cepat atau lambat
// seseorang akan menyodorkannya ke pengurai token.
const AgentHeader = "X-Widia-Agent-Key"

// AgentRequired menjaga jalur yang hanya boleh ditempuh Widia Agent.
//
// Menerima kunci apa adanya, bukan Config, supaya ketergantungannya terlihat
// pada tanda tangannya sendiri — dan supaya paket ini tidak perlu mengenal
// seluruh konfigurasi aplikasi hanya untuk membandingkan satu string.
//
// Kunci kosong berarti agent MATI. Ia diperiksa lebih dulu, sebelum
// perbandingan, karena tanpa penjagaan itu permintaan tanpa header akan
// dibandingkan dengan konfigurasi tanpa kunci — dua string kosong yang cocok
// sempurna, dan seluruh jalur agent terbuka pada setiap lingkungan yang lupa
// menyetelnya.
func AgentRequired(agentKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if agentKey == "" {
			// Dibedakan dari kunci yang salah, dan itu disengaja. Bahwa fitur ini
			// ada bukan rahasia — kodenya terbuka — sedangkan orang yang memasang
			// aplikasi ini perlu tahu bedanya "belum dinyalakan" dari "kunci saya
			// keliru", tanpa harus membaca kode server.
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Widia Agent is not enabled on this server",
			})
			return
		}

		// Perbandingan waktu-tetap. Perbandingan biasa berhenti pada byte pertama
		// yang berbeda, sehingga lama jawabannya membocorkan berapa banyak awalan
		// yang sudah benar — dan tidak ada pembatas laju di depan jalur ini yang
		// menghalangi seseorang mencoba berulang kali.
		presented := c.GetHeader(AgentHeader)
		if subtle.ConstantTimeCompare([]byte(presented), []byte(agentKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid agent key",
			})
			return
		}

		c.Next()
	}
}
