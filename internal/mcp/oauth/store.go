package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/netip"
	"sync"
	"time"
)

// Umur tiap benda yang disimpan di sini.
//
// Kode otorisasi hidup SANGAT pendek dan sekali pakai: ia melintas lewat
// pengalihan browser, sehingga ia singgah di riwayat, di log proxy, dan di
// referer. Umur panjang pada benda yang tercecer di banyak tempat adalah
// gabungan yang paling mudah disalahgunakan.
const (
	codeTTL    = 60 * time.Second
	accessTTL  = 12 * time.Hour
	refreshTTL = 30 * 24 * time.Hour
)

// Client adalah satu klien yang mendaftar sendiri lewat DCR.
//
// Tidak ada rahasia klien. Konektor GPT maupun Claude berjalan di tempat yang
// tidak dapat menyimpan rahasia dengan aman, dan OAuth 2.1 memang menuntut PKCE
// alih-alih rahasia untuk klien semacam itu — rahasia yang tidak dapat
// dirahasiakan hanya menambah pembukuan tanpa menambah jaminan apa pun.
type Client struct {
	ID           string    `json:"client_id"`
	Name         string    `json:"client_name"`
	RedirectURIs []string  `json:"redirect_uris"`
	CreatedAt    time.Time `json:"created_at"`
}

// allowsRedirect memeriksa alamat balik dengan pembandingan PERSIS.
//
// Bukan awalan, bukan pola. Pencocokan longgar pada alamat balik adalah cara
// klasik mencuri kode otorisasi: klien jahat mendaftarkan alamat yang
// "berawalan sama" lalu menerima kode milik orang lain.
func (c Client) allowsRedirect(uri string) bool {
	for _, terdaftar := range c.RedirectURIs {
		if terdaftar == uri {
			return true
		}
	}

	return false
}

// Code adalah kode otorisasi yang menunggu ditukar.
//
// Ia mengikat SELURUH keadaan permintaan aslinya — klien, alamat balik, tantangan
// PKCE, dan siapa yang login. Penukaran yang tidak cocok pada salah satunya
// ditolak, dan itulah yang membuat kode yang bocor tidak berguna bagi penemunya.
type Code struct {
	Value       string
	ClientID    string
	RedirectURI string
	Challenge   string
	Scope       string
	Resource    string
	Subject     Subject
	ExpiresAt   time.Time
}

// Subject adalah manusia yang menyetujui pemberian akses.
//
// DISIMPAN walau untuk saat ini tidak menentukan apa pun: seluruh penyuntingan
// tetap berjalan sebagai akun agent. Ia dicatat supaya dua hal kelak murah —
// mengetahui siapa yang meminta apa dari catatan, dan berpindah ke penyuntingan
// atas nama pengguna tanpa perlu mengubah bentuk token yang sudah beredar.
type Subject struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

// Token adalah access token beserta pasangannya.
//
// Buram, bukan JWT. Penerbit dan pemeriksanya adalah proses yang SAMA, jadi
// tanda tangan tidak membeli apa pun — sedangkan token buram yang dicari di
// penyimpanan dapat dicabut seketika, dan JWT yang sudah terbit tidak.
type Token struct {
	Access    string
	Refresh   string
	ClientID  string
	Scope     string
	Subject   Subject
	ExpiresAt time.Time
	RefreshAt time.Time
}

// store memegang seluruh keadaan OAuth, di MEMORI.
//
// Akibatnya harus disebut terang-terangan: setiap restart menghapus pendaftaran
// klien beserta seluruh token, sehingga konektor yang sudah tersambung WAJIB
// disambungkan ulang setiap kali MCP di-deploy. Itu ongkos yang diterima untuk
// tahap pertama; ketika deploy menjadi sering, penyimpanan yang bertahan adalah
// perbaikan berikutnya, dan bentuk paket ini sengaja tidak menghalanginya.
type store struct {
	mu       sync.Mutex
	clients  map[string]Client
	codes    map[string]Code
	tokens   map[string]Token
	refresh  map[string]string // refresh token -> access token
	attempts map[string]percobaan
}

// percobaan mencatat kegagalan login per alamat.
type percobaan struct {
	gagal  int
	sampai time.Time
}

func newStore() *store {
	return &store{
		clients:  make(map[string]Client),
		codes:    make(map[string]Code),
		tokens:   make(map[string]Token),
		refresh:  make(map[string]string),
		attempts: make(map[string]percobaan),
	}
}

// Batas percobaan sandi per alamat.
//
// /oauth/authorize adalah formulir sandi yang terbuka ke internet, dan API di
// belakangnya tidak punya pembatas sendiri. Tanpa ini, seluruh basis pengguna
// dapat ditebak dari satu mesin tanpa hambatan apa pun.
const (
	maxGagal    = 10
	jedaSetelah = 15 * time.Minute
)

// tercekal menjawab apakah alamat ini sedang dihentikan.
func (s *store) tercekal(alamat string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	catatan, ada := s.attempts[alamat]

	return ada && catatan.gagal >= maxGagal && time.Now().Before(catatan.sampai)
}

func (s *store) catatGagal(alamat string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	catatan := s.attempts[alamat]
	if time.Now().After(catatan.sampai) {
		catatan = percobaan{}
	}

	catatan.gagal++
	catatan.sampai = time.Now().Add(jedaSetelah)
	s.attempts[alamat] = catatan
}

func (s *store) catatBerhasil(alamat string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.attempts, alamat)
}

func (s *store) simpanClient(c Client) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clients[c.ID] = c
}

func (s *store) ambilClient(id string) (Client, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ada := s.clients[id]

	return c, ada
}

func (s *store) simpanCode(c Code) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.codes[c.Value] = c
}

// pakaiCode mengambil kode SEKALIGUS menghapusnya.
//
// Satu langkah di bawah satu kunci, bukan ambil-lalu-hapus. Dua penukaran yang
// tiba bersamaan dengan kode yang sama hanya boleh membuat satu di antaranya
// mendapat token; celah di antara dua langkah terpisah adalah persis yang
// dimanfaatkan penyerang yang berhasil membaca kode dari riwayat browser.
func (s *store) pakaiCode(value string) (Code, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ada := s.codes[value]
	if !ada {
		return Code{}, false
	}

	delete(s.codes, value)

	if time.Now().After(c.ExpiresAt) {
		return Code{}, false
	}

	return c, true
}

func (s *store) simpanToken(t Token) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens[t.Access] = t
	if t.Refresh != "" {
		s.refresh[t.Refresh] = t.Access
	}
}

func (s *store) ambilToken(access string) (Token, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ada := s.tokens[access]
	if !ada || time.Now().After(t.ExpiresAt) {
		return Token{}, false
	}

	return t, true
}

// pakaiRefresh menukar refresh token dengan pemiliknya, lalu MENCABUT keduanya.
//
// Rotasi, bukan pemakaian ulang. Refresh token yang tetap berlaku setelah
// dipakai tidak dapat dibedakan dari salinannya yang dicuri; yang berputar
// membuat pencurian ketahuan pada pemakaian kedua, karena salah satu pihak
// akan ditolak.
func (s *store) pakaiRefresh(value string) (Token, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	access, ada := s.refresh[value]
	if !ada {
		return Token{}, false
	}

	t, ada := s.tokens[access]
	delete(s.refresh, value)
	delete(s.tokens, access)

	if !ada || time.Now().After(t.RefreshAt) {
		return Token{}, false
	}

	return t, true
}

// sapu membuang yang sudah lewat tenggat.
//
// Tanpa ini, peta di atas hanya tumbuh: token yang kedaluwarsa tidak pernah
// dibaca lagi, tetapi tetap memakan memori selama proses hidup.
func (s *store) sapu(sekarang time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for nilai, c := range s.codes {
		if sekarang.After(c.ExpiresAt) {
			delete(s.codes, nilai)
		}
	}

	for nilai, t := range s.tokens {
		if sekarang.After(t.RefreshAt) {
			delete(s.tokens, nilai)
			delete(s.refresh, t.Refresh)
		}
	}

	for alamat, catatan := range s.attempts {
		if sekarang.After(catatan.sampai) {
			delete(s.attempts, alamat)
		}
	}
}

// acak menghasilkan nilai buram yang aman dipakai sebagai kredensial.
func acak() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("acak: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// alamatSaja membuang nomor port dari alamat pemanggil.
//
// Pembatas percobaan harus mengikat MESIN, bukan koneksi. Port berubah pada
// setiap sambungan baru, jadi menghitung dengan port di dalamnya sama saja
// dengan tidak menghitung.
func alamatSaja(remote string) string {
	if ap, err := netip.ParseAddrPort(remote); err == nil {
		return ap.Addr().String()
	}

	return remote
}
