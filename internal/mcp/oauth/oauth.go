// Package oauth adalah authorization server milik MCP.
//
// Ia berdiri di proses yang SAMA dengan resource server-nya, dan itu keputusan
// sadar dengan satu akibat yang menyenangkan: token tidak perlu ditandatangani.
// Penerbit dan pemeriksanya satu, jadi token cukup berupa nilai acak yang dicari
// di penyimpanan — dan nilai yang dicari di penyimpanan DAPAT dicabut seketika,
// sedangkan JWT yang sudah terbit tidak dapat disentuh sampai ia kedaluwarsa
// sendiri.
//
// Yang dibangun di sini hanya sisi otorisasi. Kata sandi tetap diperiksa API:
// paket ini tidak mengenal database, tidak mengenal bcrypt, dan tidak menyimpan
// satu pun sandi. Ia menukar sandi dengan pertanyaan "siapa ini?" kepada API,
// lalu melupakan sandinya.
//
// PENTING soal identitas: pengguna yang login di sini TIDAK menjadi penyunting.
// Seluruh penyuntingan tetap berjalan sebagai akun agent, sehingga OAuth di
// tahap ini menjaga PINTU, bukan membagi wewenang. Siapa pun yang lolos dapat
// menjangkau dokumen mana pun. Subject tetap dicatat pada tiap token supaya
// perpindahan ke penyuntingan atas nama pengguna kelak tidak menuntut perubahan
// bentuk token yang sudah beredar.
package oauth

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Authenticate memeriksa sandi lalu menjawab siapa pemiliknya.
//
// Tipe fungsi, bukan antarmuka, karena hanya ada satu perilaku yang dibutuhkan
// dan pemanggilnya sudah punya benda yang melakukannya. Antarmuka di sini hanya
// akan menuntut tipe pembungkus yang tidak menambah keterangan apa pun.
type Authenticate func(ctx context.Context, email, password string) (Subject, error)

// Config adalah alamat-alamat yang harus diketahui server ini tentang dirinya.
type Config struct {
	// Issuer adalah URL publik MCP, tanpa garis miring di ujung.
	//
	// Ia HARUS sama persis dengan yang dilihat klien di bilah alamat. Klien
	// membandingkan issuer pada metadata dengan alamat yang ia minta, dan
	// selisih sekecil garis miring pun membatalkan seluruh alur dengan pesan
	// yang tidak menyebut sebabnya.
	Issuer string

	// Resource adalah pengenal sumber daya yang dijaga — endpoint MCP-nya.
	Resource string
}

// Server melayani seluruh titik ujung OAuth.
type Server struct {
	cfg    Config
	auth   Authenticate
	store  *store
	logger *slog.Logger
}

func NewServer(cfg Config, auth Authenticate, logger *slog.Logger) *Server {
	return &Server{
		cfg:    cfg,
		auth:   auth,
		store:  newStore(),
		logger: logger,
	}
}

// Routes memasang seluruh titik ujung OAuth pada mux yang diberikan.
//
// Menerima mux dari luar, bukan membuat sendiri, supaya seluruh rute MCP tetap
// terbaca dari satu tempat di server.go. Rute OAuth yang bersembunyi di balik
// handler bersarang membuat pertanyaan "apa saja yang terbuka tanpa token"
// tidak lagi punya satu jawaban.
func (s *Server) Routes(mux *http.ServeMux) {
	// Ketiganya TERBUKA tanpa token, dan memang harus. Metadata adalah cara
	// klien mengetahui bahwa tempat ini butuh otorisasi serta ke mana harus
	// meminta izin; menjaganya dengan token berarti menuntut izin untuk
	// mengetahui cara meminta izin.
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", s.metadataAuthServer)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", s.metadataResource)

	// Varian berakhiran path. RFC 9728 menempelkan path sumber daya di belakang
	// nama well-known, dan klien berbeda meminta yang berbeda — Claude meminta
	// yang ini untuk resource yang berakhir /mcp. Keduanya dilayani supaya
	// penemuan tidak bergantung pada klien mana yang kebetulan dipakai.
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/mcp", s.metadataResource)

	mux.HandleFunc("POST /oauth/register", s.register)
	mux.HandleFunc("GET /oauth/authorize", s.authorizeForm)
	mux.HandleFunc("POST /oauth/authorize", s.authorizeSubmit)
	mux.HandleFunc("POST /oauth/token", s.token)
}

// Run menyapu benda kedaluwarsa sampai ctx berakhir.
func (s *Server) Run(ctx context.Context) {
	tick := time.NewTicker(5 * time.Minute)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case sekarang := <-tick.C:
			s.store.sapu(sekarang)
		}
	}
}

// galat menjawab dalam bentuk galat OAuth, bukan bentuk galat kita sendiri.
//
// Klien OAuth membaca field `error` untuk memutuskan langkah berikutnya —
// misalnya mendaftar ulang saat invalid_client. Bentuk lain terbaca olehnya
// sebagai kegagalan yang tidak dapat dipulihkan, dan pengguna melihat "gagal
// menyambung" tanpa keterangan.
func galat(w http.ResponseWriter, status int, kode, keterangan string) {
	w.Header().Set("Content-Type", "application/json")
	// Tanpa ini, jawaban galat dapat tersimpan di perantara dan terus disajikan
	// setelah sebabnya diperbaiki.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             kode,
		"error_description": keterangan,
	})
}

func tulisJSON(w http.ResponseWriter, status int, isi any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(isi)
}

// cocokResource memeriksa penanda sumber daya dari klien.
//
// RFC 8707 dituntut spesifikasi MCP justru untuk menutup satu serangan tertentu:
// tanpa penanda ini, token yang diterbitkan untuk server lain dapat dipakai di
// sini, karena tidak ada yang menyatakan token itu ditujukan ke mana.
//
// Kosong DITERIMA. Sebagian klien belum mengirimnya, dan menolak mereka berarti
// menutup pintu bagi klien yang sah demi serangan yang menuntut penyerang sudah
// memiliki authorization server lain yang kita percayai — dan kita tidak
// mempercayai satu pun.
func (s *Server) cocokResource(diminta string) bool {
	if diminta == "" {
		return true
	}

	diminta = strings.TrimRight(diminta, "/")

	return diminta == strings.TrimRight(s.cfg.Resource, "/") ||
		diminta == strings.TrimRight(s.cfg.Issuer, "/")
}
