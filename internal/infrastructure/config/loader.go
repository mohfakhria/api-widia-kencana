package config

import (
	"errors"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv            string
	AppPort           string
	LogLevel          string
	AllowedOrigins    []string
	PGHost            string
	PGPort            string
	PGUser            string
	PGPassword        string
	PGDB              string
	CookieDomain      string
	CookieSecure      bool
	JWTSecret         string
	MinIOEndpoint     string
	MinIORootUser     string
	MinIORootPassword string
	MinIOBucket       string
	MinIOUseSSL       bool
	DesignFontDir     string
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("warning: .env file not found, using system env")
	}

	return Config{
		AppEnv:  getEnv("APP_ENV", "local"),
		AppPort: getEnv("APP_PORT", "8080"),
		// debug menyalakan jejak pesan WebSocket per klien. Sengaja tidak menyala
		// secara bawaan: begitu penyuntingan mengalir, satu geseran elemen
		// menghasilkan puluhan pesan per detik.
		LogLevel: getEnv("LOG_LEVEL", "info"),

		// Asal yang boleh bicara dengan API ini, dipisah koma. Menjaga DUA hal
		// sekaligus: CORS pada jalur HTTP biasa, dan pemeriksaan Origin pada
		// handshake WebSocket.
		//
		// Yang dibandingkan HOST beserta portnya, bukan origin utuh. Skema
		// sengaja tidak ikut: di belakang reverse proxy, yang menentukan http
		// atau https adalah proxy-nya. Menulisnya tetap boleh dan diterima —
		// https://app.example.com dan app.example.com sama saja — karena orang
		// akan menulisnya apa pun yang tertulis di sini, dan gagal diam-diam
		// gara-gara skema adalah cara paling membingungkan untuk salah.
		//
		// Pola glob berlaku, dan * mencakup titik: *.example.com juga menerima
		// a.b.example.com. Sama persis dengan pustaka WebSocket, supaya kedua
		// penjaga tidak pernah berbeda pendapat.
		//
		// Kosong berarti TIDAK ADA yang diizinkan, bukan semua. Lingkungan yang
		// lupa menyetelnya menjadi yang paling tertutup. Saat APP_ENV=local,
		// seluruh host loopback ikut diizinkan tanpa perlu menyetel apa pun.
		AllowedOrigins: parseOrigins(getEnv("ALLOWED_ORIGINS", "")),

		PGHost:     getEnv("PG_HOST", "localhost"),
		PGPort:     getEnv("PG_PORT", "5432"),
		PGUser:     getEnv("PG_USER", "postgres"),
		PGPassword: getEnv("PG_PASSWORD", "postgres"),
		PGDB:       getEnv("PG_DB", "postgres"),

		// Domain kosong membuat cookie host-only: terikat persis ke host yang
		// men-set-nya, dan benar tanpa dikonfigurasi baik di localhost maupun
		// production. Isi COOKIE_DOMAIN hanya bila cookie memang perlu dibagi
		// ke beberapa subdomain.
		CookieDomain: getEnv("COOKIE_DOMAIN", ""),

		// Disetel eksplisit, tanpa nilai yang diturunkan dari mana pun. Dulu ia
		// mengikuti skema APP_BASEURL, dan itu menyesatkan justru pada susunan
		// yang paling umum: di belakang reverse proxy, APP_BASEURL menunjuk
		// alamat internal http:// sementara browser bicara https, sehingga
		// tebakannya selalu salah arah.
		//
		// APP_ENV=production menuntutnya true — lihat Validate.
		CookieSecure: getBoolEnv("COOKIE_SECURE", false),

		JWTSecret:         getEnv("JWT_SECRET", "change-this-in-env"),
		MinIOEndpoint:     getEnv("MINIO_ENDPOINT", "localhost:9002"),
		MinIORootUser:     getEnv("MINIO_ROOT_USER", ""),
		MinIORootPassword: getEnv("MINIO_ROOT_PASSWORD", ""),
		MinIOBucket:       getEnv("MINIO_BUCKET", "widia-assets"),
		MinIOUseSSL:       getBoolEnv("MINIO_USE_SSL", false),

		// Direktori berkas font untuk ekspor PDF. Berkas yang sama harus
		// disajikan ke frontend: nama keluarga yang sama tidak cukup, karena
		// Helvetica di macOS dan Arial di Windows punya lebar glif yang berbeda
		// dan perbedaan itu menggeser pemenggalan baris.
		// Kosong berarti ekspor memakai Helvetica inti PDF, dan itu keadaan yang
		// didukung — bukan keadaan darurat. Jalur bawaan yang menunjuk direktori
		// tertentu berarti direktori itu dapat menyalakan pemuatan font tanpa ada
		// yang pernah memintanya, dan sekarang tidak ada satu pun berkas
		// konfigurasi yang menyebutkan namanya.
		//
		// Isi hanya bila mendaftarkan berkas font sendiri.
		DesignFontDir: getEnv("DESIGN_FONT_DIR", ""),
	}
}

func (c Config) Address() string {
	return ":" + c.AppPort
}

func (c Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

// IsLocal menandai pengembangan di mesin sendiri.
//
// Dipakai untuk melonggarkan pemeriksaan yang di lingkungan lain harus ketat.
// Sengaja diperiksa persis "local", bukan "bukan production", supaya staging
// tetap seketat produksi.
func (c Config) IsLocal() bool {
	return strings.EqualFold(c.AppEnv, "local")
}

// Validate menolak kombinasi konfigurasi yang diam-diam melemahkan keamanan.
func (c Config) Validate() error {
	if c.IsProduction() && !c.CookieSecure {
		return errors.New("APP_ENV=production membutuhkan refresh cookie yang Secure: set COOKIE_SECURE=true")
	}

	return nil
}

func (c Config) PostgresDSN() string {
	return "host=" + c.PGHost +
		" port=" + c.PGPort +
		" user=" + c.PGUser +
		" password=" + c.PGPassword +
		" dbname=" + c.PGDB +
		" sslmode=disable"
}

func (c Config) PortNumber() int {
	port, err := strconv.Atoi(c.AppPort)
	if err != nil {
		return 8080
	}
	return port
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

// parseOrigins memecah daftar dipisah koma menjadi pola host.
//
// Skema dibuang bila ditulis, sehingga https://app.example.com dan
// app.example.com menghasilkan pola yang sama. Jalur di belakangnya juga
// dibuang: origin tidak pernah punya jalur, dan yang telanjur menuliskannya
// lebih baik dimaafkan daripada ditolak diam-diam.
func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		pattern := strings.TrimSpace(part)
		if pattern == "" {
			continue
		}
		if index := strings.Index(pattern, "://"); index >= 0 {
			pattern = pattern[index+3:]
		}
		pattern = strings.TrimSuffix(strings.SplitN(pattern, "/", 2)[0], ".")
		if pattern != "" {
			out = append(out, pattern)
		}
	}

	return out
}

func getBoolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value != "false" && value != "0" && value != "no"
}
