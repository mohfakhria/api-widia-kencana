package output

import (
	"context"
	"time"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// TokenClaims sengaja TIDAK membawa identitas pengguna.
//
// Token hanya menunjuk sesi; siapa pemilik sesi itu dijawab SessionStore. Dulu
// id pengguna ikut di dalam token dan harus dienkripsi supaya tidak terbaca
// siapa pun yang men-decode baseage64-nya — enkripsi yang ada semata-mata untuk
// menyembunyikan sesuatu yang sebenarnya tidak perlu dibawa.
//
// Akibat yang lebih penting daripada kerapiannya: karena identitas dijawab
// penyimpanan, sesi yang dihapus membuat token yang sudah terlanjur beredar
// berhenti berlaku SEKETIKA. Selama identitas ada di dalam token, logout tidak
// dapat menyentuh access token yang sedang dipegang sampai ia kedaluwarsa
// sendiri.
type TokenClaims struct {
	SessionID string
	TokenType string
}

type TokenSigner interface {
	GenerateAccessToken(ctx context.Context, claims TokenClaims, ttl time.Duration) (string, error)
	GenerateRefreshToken(ctx context.Context, claims TokenClaims, ttl time.Duration) (string, error)
	ParseToken(ctx context.Context, token string) (*TokenClaims, error)
}
