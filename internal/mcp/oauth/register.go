package oauth

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// registrationRequest adalah bentuk permintaan RFC 7591 yang kita pedulikan.
//
// Klien mengirim jauh lebih banyak field daripada ini. Yang tidak dikenal
// DIABAIKAN dengan sengaja — kebalikan dari model isi dokumen, yang tertutup.
// Di sana ketertutupan menjaga kontrak antara dua sistem yang kita miliki
// berdua; di sini pengirimnya adalah klien pihak ketiga yang bebas menambah
// field kapan saja, dan menolak pendaftaran karena field yang tidak kita kenal
// berarti menolak klien yang sebenarnya sah.
type registrationRequest struct {
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

// registrationResponse mengikuti RFC 7591.
//
// TokenEndpointAuthMethod disebut terang-terangan sebagai "none" supaya klien
// tidak menebak bahwa ia perlu rahasia — dan tidak mengirim Authorization pada
// /oauth/token yang justru akan ditolak.
type registrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
}

// register menerima pendaftaran klien, TANPA autentikasi.
//
// Terbuka, dan memang begitulah DCR bekerja untuk MCP: konektor mendaftar
// sendiri sebelum ada seorang pun yang login, jadi tidak ada kredensial yang
// dapat dituntut pada langkah ini. Yang menjaga bukan pendaftarannya melainkan
// langkah sesudahnya — pendaftaran tidak memberi akses apa pun sampai seorang
// manusia memasukkan sandinya di /oauth/authorize.
//
// Yang benar-benar berbahaya di sini adalah alamat balik. Klien yang berhasil
// mendaftarkan alamat balik miliknya lalu membujuk pengguna melewati alur akan
// menerima kode otorisasi milik pengguna itu. Karena itu alamatnya diperiksa
// bentuknya di sini, dan dicocokkan PERSIS saat penukaran.
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var permintaan registrationRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&permintaan); err != nil {
		galat(w, http.StatusBadRequest, "invalid_client_metadata", "muatan pendaftaran tidak dapat dibaca")
		return
	}

	if len(permintaan.RedirectURIs) == 0 {
		galat(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uris wajib diisi")
		return
	}

	for _, uri := range permintaan.RedirectURIs {
		if err := periksaRedirect(uri); err != nil {
			galat(w, http.StatusBadRequest, "invalid_redirect_uri", err.Error())
			return
		}
	}

	id, err := acak()
	if err != nil {
		s.logger.Error("gagal membuat client_id", "error", err)
		galat(w, http.StatusInternalServerError, "server_error", "tidak dapat menerbitkan client_id")

		return
	}

	klien := Client{
		ID:           id,
		Name:         strings.TrimSpace(permintaan.ClientName),
		RedirectURIs: permintaan.RedirectURIs,
		CreatedAt:    time.Now(),
	}
	s.store.simpanClient(klien)

	s.logger.Info("klien oauth mendaftar",
		"client_id", klien.ID, "nama", klien.Name, "redirect", klien.RedirectURIs)

	tulisJSON(w, http.StatusCreated, registrationResponse{
		ClientID:                klien.ID,
		ClientName:              klien.Name,
		RedirectURIs:            klien.RedirectURIs,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
		ClientIDIssuedAt:        klien.CreatedAt.Unix(),
	})
}

// periksaRedirect menolak alamat balik yang tidak layak dipercaya.
//
// Fragmen dilarang RFC 6749 karena ia tidak pernah sampai ke server dan
// membuat penyusunan alamat balik menjadi ambigu. Skema selain https ditolak
// kecuali localhost, yang memang dipakai klien desktop yang mendengarkan di
// mesin penggunanya sendiri.
func periksaRedirect(raw string) error {
	uri, err := url.Parse(raw)
	if err != nil {
		return errRedirect("alamat balik tidak dapat diurai")
	}
	if uri.Fragment != "" || strings.Contains(raw, "#") {
		return errRedirect("alamat balik tidak boleh memuat fragmen")
	}

	switch uri.Scheme {
	case "https":
		return nil
	case "http":
		if host := uri.Hostname(); host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}

		return errRedirect("http hanya diizinkan untuk localhost")
	default:
		// Skema aplikasi seperti claude:// dipakai klien desktop untuk kembali ke
		// dirinya sendiri. Diterima selama ia menyebut skema dan jalur — yang
		// ditolak adalah string yang bukan alamat sama sekali.
		if uri.Scheme != "" && uri.Opaque == "" && uri.Host == "" && uri.Path == "" {
			return errRedirect("alamat balik tidak lengkap")
		}

		return nil
	}
}

type errRedirect string

func (e errRedirect) Error() string { return string(e) }
