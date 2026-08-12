package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"time"
)

// tokenResponse mengikuti RFC 6749 §5.1.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		galat(w, http.StatusBadRequest, "invalid_request", "formulir tidak dapat dibaca")
		return
	}

	switch r.PostFormValue("grant_type") {
	case "authorization_code":
		s.tukarKode(w, r)
	case "refresh_token":
		s.tukarRefresh(w, r)
	default:
		galat(w, http.StatusBadRequest, "unsupported_grant_type",
			"hanya authorization_code dan refresh_token yang didukung")
	}
}

// tukarKode menukar kode otorisasi dengan token.
func (s *Server) tukarKode(w http.ResponseWriter, r *http.Request) {
	// Diambil sekaligus dihapus. Kode yang gagal ditukar TIDAK dikembalikan ke
	// penyimpanan — sekali pakai berarti sekali disentuh, dan kode yang boleh
	// dicoba berulang kali adalah kode yang dapat ditebak PKCE-nya dengan
	// mencoba.
	kode, ok := s.store.pakaiCode(r.PostFormValue("code"))
	if !ok {
		galat(w, http.StatusBadRequest, "invalid_grant", "kode tidak dikenal atau sudah kedaluwarsa")
		return
	}

	// Klien yang menukar HARUS klien yang meminta. Tanpa pemeriksaan ini, klien
	// jahat yang memegang kode curian dapat menukarnya sebagai dirinya sendiri.
	if r.PostFormValue("client_id") != kode.ClientID {
		galat(w, http.StatusBadRequest, "invalid_grant", "kode ini bukan milik client_id tersebut")
		return
	}

	// Alamat balik ikut diperiksa walau kodenya sudah terbit, karena RFC 6749
	// menuntutnya: ia mengikat penukaran pada permintaan yang sama persis.
	if r.PostFormValue("redirect_uri") != kode.RedirectURI {
		galat(w, http.StatusBadRequest, "invalid_grant", "redirect_uri berbeda dari permintaan aslinya")
		return
	}

	if !cocokPKCE(kode.Challenge, r.PostFormValue("code_verifier")) {
		galat(w, http.StatusBadRequest, "invalid_grant", "code_verifier tidak cocok")
		return
	}

	if !s.cocokResource(r.PostFormValue("resource")) {
		galat(w, http.StatusBadRequest, "invalid_target", "resource tidak menunjuk server ini")
		return
	}

	s.terbitkan(w, kode.ClientID, kode.Scope, kode.Subject)
}

// tukarRefresh memperpanjang akses tanpa menanyakan sandi lagi.
func (s *Server) tukarRefresh(w http.ResponseWriter, r *http.Request) {
	lama, ok := s.store.pakaiRefresh(r.PostFormValue("refresh_token"))
	if !ok {
		galat(w, http.StatusBadRequest, "invalid_grant", "refresh token tidak dikenal atau sudah dipakai")
		return
	}

	if diminta := r.PostFormValue("client_id"); diminta != "" && diminta != lama.ClientID {
		galat(w, http.StatusBadRequest, "invalid_grant", "refresh token ini bukan milik client_id tersebut")
		return
	}

	s.terbitkan(w, lama.ClientID, lama.Scope, lama.Subject)
}

// terbitkan membuat sepasang token baru dan menjawabkannya.
func (s *Server) terbitkan(w http.ResponseWriter, clientID, scope string, subject Subject) {
	access, err := acak()
	if err != nil {
		s.logger.Error("gagal membuat access token", "error", err)
		galat(w, http.StatusInternalServerError, "server_error", "tidak dapat menerbitkan token")

		return
	}

	refresh, err := acak()
	if err != nil {
		s.logger.Error("gagal membuat refresh token", "error", err)
		galat(w, http.StatusInternalServerError, "server_error", "tidak dapat menerbitkan token")

		return
	}

	sekarang := time.Now()
	s.store.simpanToken(Token{
		Access:    access,
		Refresh:   refresh,
		ClientID:  clientID,
		Scope:     scope,
		Subject:   subject,
		ExpiresAt: sekarang.Add(accessTTL),
		RefreshAt: sekarang.Add(refreshTTL),
	})

	s.logger.Info("token oauth terbit", "client_id", clientID, "subject", subject.Email)

	tulisJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int(accessTTL.Seconds()),
		RefreshToken: refresh,
		Scope:        scope,
	})
}

// cocokPKCE memeriksa verifier terhadap tantangan yang disimpan.
//
// S256: tantangan adalah base64url tanpa padding dari SHA-256 verifier.
// Pembandingnya waktu-tetap — tantangannya memang bukan rahasia, tetapi
// perbandingan biasa pada nilai turunan rahasia adalah kebiasaan yang tidak
// layak dipelihara di berkas seperti ini.
func cocokPKCE(tantangan, verifier string) bool {
	if tantangan == "" || verifier == "" {
		return false
	}

	sum := sha256.Sum256([]byte(verifier))
	dihitung := base64.RawURLEncoding.EncodeToString(sum[:])

	return subtle.ConstantTimeCompare([]byte(dihitung), []byte(tantangan)) == 1
}
