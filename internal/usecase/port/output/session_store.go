package output

import (
	"context"
	"time"
)

// SessionStore memegang sesi yang sedang hidup.
//
// Bukan hanya urusan refresh, dan namanya pernah menyebut begitu: sejak
// identitas dicabut dari token, SETIAP permintaan terautentikasi bertanya ke
// sini untuk tahu siapa pemanggilnya. Ia karenanya menjadi satu-satunya tempat
// yang menentukan sebuah sesi masih berlaku atau tidak.
type SessionStore interface {
	Set(ctx context.Context, sessionID string, session Session, ttl time.Duration) error
	Get(ctx context.Context, sessionID string) (*Session, error)
	Delete(ctx context.Context, sessionID string) error
	DeleteAll(ctx context.Context, userID int64) error
}

// Session adalah pemilik sesi beserta sidik jari refresh token terakhirnya.
//
// TokenHash hanya dipakai jalur refresh; validasi access token cukup membaca
// sisanya. Disimpan sebagai SHA-256, bukan tokennya, supaya isi penyimpanan ini
// tidak dapat dipakai sebagai kredensial oleh siapa pun yang membacanya.
//
// Name, Email, dan Role IKUT DISALIN ke sini, dan itu memang salinan — bukan sumber.
// Aslinya di tabel users, dan keduanya diambil pada langkah yang MEMANG sudah
// menyentuh database: login dan refresh. Tanpa salinan ini, satu permintaan
// yang perlu keduanya menembak database dua kali untuk baris yang sama.
//
// Harga salinan adalah basi, dan batasnya perlu diketahui: peran yang diubah
// lewat SQL baru berlaku pada refresh berikutnya — paling lama 24 jam, karena
// itu umur access token. Yang membedakannya dari menaruh peran di dalam token:
// sesi DAPAT dibatalkan seketika lewat Delete atau DeleteAll, sedangkan token
// yang sudah terbit tidak dapat disentuh sama sekali. Arah yang berbahaya —
// menghentikan sesi yang liar — karenanya tetap tertutup.
type Session struct {
	UserID    int64  `json:"user_id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	TokenHash string `json:"token_hash"`
}
