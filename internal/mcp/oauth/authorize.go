package oauth

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// permintaanOtorisasi adalah parameter alur authorization code.
type permintaanOtorisasi struct {
	ClientID     string
	RedirectURI  string
	State        string
	Scope        string
	Resource     string
	Challenge    string
	ChallengeAlg string
	ResponseType string
}

func bacaPermintaan(nilai url.Values) permintaanOtorisasi {
	return permintaanOtorisasi{
		ClientID:     nilai.Get("client_id"),
		RedirectURI:  nilai.Get("redirect_uri"),
		State:        nilai.Get("state"),
		Scope:        nilai.Get("scope"),
		Resource:     nilai.Get("resource"),
		Challenge:    nilai.Get("code_challenge"),
		ChallengeAlg: nilai.Get("code_challenge_method"),
		ResponseType: nilai.Get("response_type"),
	}
}

// periksa mengembalikan kode galat OAuth bila permintaannya tidak layak dilanjutkan.
//
// Dipisah dari penanganannya karena kedua jalur — GET dan POST — harus memakai
// pemeriksaan yang PERSIS sama. Dua salinan pemeriksaan keamanan yang perlahan
// berbeda adalah cara paling umum sebuah lubang muncul tanpa ada yang menulisnya.
func (s *Server) periksa(p permintaanOtorisasi) (kode, keterangan string) {
	if p.ResponseType != "code" {
		return "unsupported_response_type", "hanya response_type=code yang didukung"
	}
	if p.Challenge == "" {
		return "invalid_request", "code_challenge wajib diisi"
	}
	if p.ChallengeAlg != "S256" {
		return "invalid_request", "code_challenge_method harus S256"
	}
	if !s.cocokResource(p.Resource) {
		return "invalid_target", "resource tidak menunjuk server ini"
	}

	return "", ""
}

// authorizeForm menyajikan halaman login.
func (s *Server) authorizeForm(w http.ResponseWriter, r *http.Request) {
	p := bacaPermintaan(r.URL.Query())

	klien, ok := s.store.ambilClient(p.ClientID)
	if !ok {
		// TIDAK dialihkan. Klien yang tidak dikenal berarti alamat baliknya juga
		// tidak dapat dipercaya, dan mengalihkan galat ke alamat yang tidak
		// terdaftar adalah cara sebuah authorization server dijadikan alat
		// pengalih terbuka.
		s.halamanGalat(w, http.StatusBadRequest, "Klien tidak dikenal",
			"Aplikasi yang mengirim Anda ke sini belum terdaftar. Sambungkan ulang konektornya.")

		return
	}

	if !klien.allowsRedirect(p.RedirectURI) {
		s.halamanGalat(w, http.StatusBadRequest, "Alamat balik tidak cocok",
			"Alamat balik yang diminta tidak sama dengan yang didaftarkan aplikasi ini.")

		return
	}

	// Sejak titik ini alamat baliknya SUDAH tepercaya, jadi galat selanjutnya
	// dikembalikan lewat pengalihan — itu yang membuat klien dapat menampilkan
	// sebabnya kepada pengguna alih-alih menggantung.
	if kode, keterangan := s.periksa(p); kode != "" {
		s.alihkanGalat(w, r, p, kode, keterangan)
		return
	}

	s.halamanLogin(w, http.StatusOK, p, klien, "")
}

// authorizeSubmit memeriksa sandi lalu menerbitkan kode otorisasi.
func (s *Server) authorizeSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.halamanGalat(w, http.StatusBadRequest, "Formulir tidak terbaca",
			"Muat ulang halaman lalu coba lagi.")

		return
	}

	p := bacaPermintaan(r.PostForm)

	klien, ok := s.store.ambilClient(p.ClientID)
	if !ok || !klien.allowsRedirect(p.RedirectURI) {
		// Diperiksa ULANG, bukan dipercaya dari formulir. Seluruh isi formulir
		// datang dari peramban dan dapat disunting siapa pun sebelum dikirim.
		s.halamanGalat(w, http.StatusBadRequest, "Permintaan tidak sah",
			"Sambungkan ulang konektornya lalu ulangi dari awal.")

		return
	}

	if kode, keterangan := s.periksa(p); kode != "" {
		s.alihkanGalat(w, r, p, kode, keterangan)
		return
	}

	alamat := alamatSaja(r.RemoteAddr)
	if s.store.tercekal(alamat) {
		s.logger.Warn("percobaan login oauth dihentikan", "alamat", alamat)
		s.halamanLogin(w, http.StatusTooManyRequests, p, klien,
			"Terlalu banyak percobaan gagal. Coba lagi beberapa menit lagi.")

		return
	}

	email := strings.TrimSpace(r.PostFormValue("email"))
	sandi := r.PostFormValue("password")

	subject, err := s.auth(r.Context(), email, sandi)
	if err != nil {
		s.store.catatGagal(alamat)
		// Sebabnya TIDAK dibedakan bagi pengguna: "email tidak ada" dan "sandi
		// salah" yang terpisah memberi tahu penebak bahwa sebuah alamat email
		// terdaftar di sini.
		s.logger.Warn("login oauth ditolak", "alamat", alamat, "error", err)
		s.halamanLogin(w, http.StatusUnauthorized, p, klien, "Email atau kata sandi salah.")

		return
	}

	s.store.catatBerhasil(alamat)

	nilai, err := acak()
	if err != nil {
		s.logger.Error("gagal membuat kode otorisasi", "error", err)
		s.alihkanGalat(w, r, p, "server_error", "tidak dapat menerbitkan kode")

		return
	}

	s.store.simpanCode(Code{
		Value:       nilai,
		ClientID:    p.ClientID,
		RedirectURI: p.RedirectURI,
		Challenge:   p.Challenge,
		Scope:       p.Scope,
		Resource:    p.Resource,
		Subject:     subject,
		ExpiresAt:   time.Now().Add(codeTTL),
	})

	s.logger.Info("kode otorisasi terbit",
		"client_id", p.ClientID, "subject", subject.Email)

	balik, err := url.Parse(p.RedirectURI)
	if err != nil {
		s.halamanGalat(w, http.StatusBadRequest, "Alamat balik tidak sah", err.Error())
		return
	}

	q := balik.Query()
	q.Set("code", nilai)
	if p.State != "" {
		q.Set("state", p.State)
	}
	balik.RawQuery = q.Encode()

	http.Redirect(w, r, balik.String(), http.StatusFound)
}

// alihkanGalat mengembalikan galat ke klien lewat alamat baliknya.
func (s *Server) alihkanGalat(w http.ResponseWriter, r *http.Request, p permintaanOtorisasi, kode, keterangan string) {
	balik, err := url.Parse(p.RedirectURI)
	if err != nil {
		s.halamanGalat(w, http.StatusBadRequest, "Alamat balik tidak sah", keterangan)
		return
	}

	q := balik.Query()
	q.Set("error", kode)
	q.Set("error_description", keterangan)
	if p.State != "" {
		q.Set("state", p.State)
	}
	balik.RawQuery = q.Encode()

	http.Redirect(w, r, balik.String(), http.StatusFound)
}

// Halaman disusun html/template, bukan fmt.Sprintf.
//
// Seluruh nilai yang masuk ke sini berasal dari parameter URL yang dikendalikan
// pihak lain — nama klien datang dari pendaftaran terbuka, state dari klien.
// Penggabungan string akan menjadikan halaman ini titik XSS yang sempurna:
// halaman sandi, di domain kita, yang isinya ditentukan penyerang.
var (
	tplLogin = template.Must(template.New("login").Parse(`
<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Masuk — Widia Kencana</title>
<style>
 :root { color-scheme: light dark; }
 body { font-family: system-ui, -apple-system, "Segoe UI", sans-serif; margin: 0;
        display: grid; place-items: center; min-height: 100vh; background: #f5f5f7; }
 @media (prefers-color-scheme: dark) { body { background: #16161a; } }
 .kartu { width: min(360px, 92vw); background: Canvas; color: CanvasText;
          padding: 28px; border-radius: 14px; box-shadow: 0 8px 30px rgba(0,0,0,.12); }
 h1 { font-size: 18px; margin: 0 0 4px; }
 p.sub { font-size: 13px; color: #6b6b70; margin: 0 0 20px; }
 label { display: block; font-size: 13px; margin: 14px 0 6px; }
 input { width: 100%; box-sizing: border-box; padding: 10px 12px; font-size: 14px;
         border: 1px solid #d0d0d5; border-radius: 8px; background: Canvas; color: CanvasText; }
 button { width: 100%; margin-top: 20px; padding: 11px; font-size: 14px; font-weight: 600;
          border: 0; border-radius: 8px; background: #2f6feb; color: #fff; cursor: pointer; }
 .galat { margin-top: 16px; padding: 10px 12px; font-size: 13px; border-radius: 8px;
          background: #fdeaea; color: #a3252c; }
 @media (prefers-color-scheme: dark) { .galat { background: #3a1d1f; color: #ffb3b8; } }
</style>
<div class="kartu">
  <h1>Masuk ke Widia Kencana</h1>
  <p class="sub">{{if .Nama}}<strong>{{.Nama}}</strong> meminta akses ke dokumen desain Anda.{{else}}Sebuah aplikasi meminta akses ke dokumen desain Anda.{{end}}</p>
  {{if .Pesan}}<div class="galat">{{.Pesan}}</div>{{end}}
  <form method="post" action="/oauth/authorize">
    <label for="email">Email</label>
    <input id="email" name="email" type="email" autocomplete="username" required autofocus>
    <label for="password">Kata sandi</label>
    <input id="password" name="password" type="password" autocomplete="current-password" required>
    <input type="hidden" name="response_type" value="{{.P.ResponseType}}">
    <input type="hidden" name="client_id" value="{{.P.ClientID}}">
    <input type="hidden" name="redirect_uri" value="{{.P.RedirectURI}}">
    <input type="hidden" name="state" value="{{.P.State}}">
    <input type="hidden" name="scope" value="{{.P.Scope}}">
    <input type="hidden" name="resource" value="{{.P.Resource}}">
    <input type="hidden" name="code_challenge" value="{{.P.Challenge}}">
    <input type="hidden" name="code_challenge_method" value="{{.P.ChallengeAlg}}">
    <button type="submit">Masuk dan izinkan</button>
  </form>
</div>
`))

	tplGalat = template.Must(template.New("galat").Parse(`
<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Judul}} — Widia Kencana</title>
<style>
 :root { color-scheme: light dark; }
 body { font-family: system-ui, -apple-system, "Segoe UI", sans-serif; margin: 0;
        display: grid; place-items: center; min-height: 100vh; background: #f5f5f7; }
 @media (prefers-color-scheme: dark) { body { background: #16161a; } }
 .kartu { width: min(400px, 92vw); background: Canvas; color: CanvasText;
          padding: 28px; border-radius: 14px; box-shadow: 0 8px 30px rgba(0,0,0,.12); }
 h1 { font-size: 17px; margin: 0 0 8px; }
 p { font-size: 14px; line-height: 1.5; color: #6b6b70; margin: 0; }
</style>
<div class="kartu"><h1>{{.Judul}}</h1><p>{{.Pesan}}</p></div>
`))
)

func (s *Server) halamanLogin(w http.ResponseWriter, status int, p permintaanOtorisasi, klien Client, pesan string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Halaman ini memuat parameter permintaan dan tidak boleh tersimpan di
	// peramban maupun perantara — riwayat yang menyimpannya membuat kode
	// otorisasi dapat diminta ulang oleh siapa pun yang memegang mesin itu.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	_ = tplLogin.Execute(w, map[string]any{
		"P":     p,
		"Nama":  klien.Name,
		"Pesan": pesan,
	})
}

func (s *Server) halamanGalat(w http.ResponseWriter, status int, judul, pesan string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	_ = tplGalat.Execute(w, map[string]any{"Judul": judul, "Pesan": pesan})
}
