package http

import (
	"net/url"
	"path"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
)

// Satu daftar host yang menjawab satu pertanyaan: browser dari mana yang boleh
// bicara dengan API ini.
//
// Dipakai dua penjaga sekaligus — CORS pada jalur HTTP biasa, dan pemeriksaan
// Origin pada handshake WebSocket. Keduanya dulu punya aturan sendiri: CORS
// menuntut kecocokan penuh termasuk skema, sedangkan handshake hanya
// mencocokkan host. Dua aturan berbeda untuk satu pertanyaan berarti satu
// frontend dapat lolos di satu jalur dan ditolak di jalur lain, dan yang
// ditolak adalah jalur yang paling sulit didiagnosis dari browser.
//
// Yang dicocokkan HOST beserta portnya, bukan origin utuh. Skema sengaja tidak
// ikut diperiksa: di belakang reverse proxy, yang menentukan http atau https
// adalah proxy-nya, bukan konfigurasi ini — dan menuntutnya cocok hanya
// menghasilkan penolakan yang membingungkan tiap kali TLS dipasang atau dilepas.
// Perlindungan terhadap situs asing tetap utuh, karena nama host-lah yang
// membedakan mereka.
//
// Konsekuensi yang mudah mengejutkan: pada path.Match, * mencakup titik. Pola
// *.example.com karena itu juga menerima a.b.example.com. Itu disengaja
// dibiarkan — pustaka WebSocket memakai semantik yang sama, dan seluruhnya tetap
// berada di bawah domain yang memang milik pemasangnya.

// allowedOriginPatterns menyusun daftar pola yang berlaku bagi kedua penjaga.
func allowedOriginPatterns(cfg config.Config) []string {
	patterns := make([]string, 0, len(cfg.AllowedOrigins)+6)
	patterns = append(patterns, cfg.AllowedOrigins...)

	if cfg.IsLocal() {
		patterns = append(patterns, localOriginPatterns()...)
	}

	return patterns
}

// localOriginPatterns adalah host yang ikut diizinkan saat APP_ENV=local.
//
// Di mesin sendiri, port dan subdomain berganti mengikuti cara frontend
// dijalankan — localhost:3000, portal.localhost:3000, dan seterusnya — sehingga
// menyetel ulang ALLOWED_ORIGINS setiap kali hanya menghambat.
//
// Pelonggarannya tetap terkurung pada loopback. Akhiran .localhost adalah TLD
// yang dicadangkan RFC 6761 dan tidak dapat didaftarkan publik, jadi tidak ada
// host luar yang bisa menyamar lewat pola ini.
//
// Pencocokannya memakai path.Match, tempat * mencakup apa saja selain garis
// miring. Karena itu pola seperti "localhost*" sengaja dihindari: ia juga akan
// cocok dengan localhost.evil.com. Setiap pola di bawah menambatkan nama host
// secara utuh, dan hanya bagian port yang dibebaskan.
//
// IPv6 tidak disertakan: path.Match memperlakukan "[" sebagai pembuka kelas
// karakter, sehingga pola untuk [::1] akan diartikan sama sekali lain.
func localOriginPatterns() []string {
	return []string{
		"localhost",
		"localhost:*",
		"*.localhost",
		"*.localhost:*",
		"127.0.0.1",
		"127.0.0.1:*",
	}
}

// originAllowed mencocokkan satu header Origin dengan daftar pola.
//
// Semantiknya dibuat sama persis dengan yang dipakai pustaka WebSocket untuk
// OriginPatterns — host tanpa peduli besar-kecil huruf, dicocokkan path.Match —
// supaya kedua penjaga tidak pernah berbeda pendapat tentang origin yang sama.
//
// Daftar kosong berarti tidak ada yang diizinkan, bukan semua diizinkan. Itu
// arah kegagalan yang benar: lingkungan yang lupa menyetelnya menjadi yang
// paling tertutup, bukan yang paling terbuka.
func originAllowed(patterns []string, origin string) bool {
	if origin == "" || len(patterns) == 0 {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}

	host := strings.ToLower(parsed.Host)
	for _, pattern := range patterns {
		if ok, err := path.Match(strings.ToLower(pattern), host); err == nil && ok {
			return true
		}
	}

	return false
}
