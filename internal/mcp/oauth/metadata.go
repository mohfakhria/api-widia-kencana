package oauth

import "net/http"

// scopeDesign adalah satu-satunya scope yang dikenal.
//
// Satu, bukan sederet. Scope yang tidak dipakai untuk memutuskan apa pun hanya
// menghasilkan layar persetujuan yang memperagakan pembatasan yang sebenarnya
// tidak ada — dan itu lebih menyesatkan daripada tidak punya scope sama sekali.
// Pembagian wewenang yang sungguhan menunggu penyuntingan atas nama pengguna.
const scopeDesign = "design"

// authServerMetadata adalah struct kita sendiri, bukan oauthex.AuthServerMeta.
//
// Sebabnya satu field: JWKSURI di sana tanpa omitempty, sehingga metadata kita
// akan mengumumkan `"jwks_uri": ""`. Kita tidak memakai JWT dan tidak punya
// kunci publik untuk diumumkan; field kosong itu mengundang klien mengambil
// alamat kosong, dan kegagalannya muncul jauh dari sebabnya.
type authServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	ScopesSupported                   []string `json:"scopes_supported"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
}

func (s *Server) metadataAuthServer(w http.ResponseWriter, _ *http.Request) {
	tulisJSON(w, http.StatusOK, authServerMetadata{
		Issuer:                 s.cfg.Issuer,
		AuthorizationEndpoint:  s.cfg.Issuer + "/oauth/authorize",
		TokenEndpoint:          s.cfg.Issuer + "/oauth/token",
		RegistrationEndpoint:   s.cfg.Issuer + "/oauth/register",
		ScopesSupported:        []string{scopeDesign},
		ResponseTypesSupported: []string{"code"},
		GrantTypesSupported:    []string{"authorization_code", "refresh_token"},

		// none, dan hanya none. Konektor GPT maupun Claude adalah klien PUBLIK —
		// mereka berjalan di tempat yang tidak dapat menyimpan rahasia — dan
		// PKCE-lah yang menggantikan rahasia klien di OAuth 2.1. Mengumumkan
		// metode berbasis rahasia hanya mengundang klien memilih jalur yang
		// jaminannya semu.
		TokenEndpointAuthMethodsSupported: []string{"none"},

		// S256 saja. "plain" masih sah menurut RFC 7636 tetapi tidak memberi
		// perlindungan apa pun bila tantangannya ikut terbaca bersama kodenya.
		CodeChallengeMethodsSupported: []string{"S256"},
	})
}

// resourceMetadata memberi tahu klien ke mana harus meminta izin.
//
// Inilah yang membuat 401 dari /mcp dapat ditindaklanjuti: klien membaca
// WWW-Authenticate, mengambil dokumen ini, menemukan authorization server-nya,
// lalu memulai alur — seluruhnya tanpa seorang pun menyetel apa pun secara
// manual.
type resourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	ScopesSupported        []string `json:"scopes_supported"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	ResourceName           string   `json:"resource_name"`
}

func (s *Server) metadataResource(w http.ResponseWriter, _ *http.Request) {
	tulisJSON(w, http.StatusOK, resourceMetadata{
		Resource:               s.cfg.Resource,
		AuthorizationServers:   []string{s.cfg.Issuer},
		ScopesSupported:        []string{scopeDesign},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "Widia Kencana Document Design",
	})
}

// ResourceMetadataURL dipakai middleware resource server untuk menyusun
// WWW-Authenticate. Diambil dari sini, bukan disusun ulang di server.go, supaya
// alamat yang diumumkan dan alamat yang dilayani tidak dapat berbeda.
func (s *Server) ResourceMetadataURL() string {
	return s.cfg.Issuer + "/.well-known/oauth-protected-resource"
}
