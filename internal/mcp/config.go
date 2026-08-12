package mcp

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config adalah seluruh yang dibutuhkan MCP untuk berdiri.
//
// MCP adalah KLIEN API, bukan bagiannya. Ia tidak mengenal database, MinIO,
// maupun kunci JWT — satu-satunya yang ia tahu adalah alamat API dan cara masuk
// sebagai agent. Itu batas yang dijaga dengan sengaja: begitu MCP memegang
// koneksi database, godaan memotong lewat usecase muncul, dan pintasan itu
// menghapus tiket, sesi, presence, dan kursor — yaitu seluruh alasan
// penyuntingan agent dilewatkan socket supaya dapat ditonton.
type Config struct {
	Port string

	// APIBaseURL menunjuk satu alamat untuk HTTP maupun WebSocket, karena
	// keduanya memang dilayani proses dan port yang sama. Dua variabel yang wajib
	// menunjuk server yang sama adalah dua kesempatan untuk meleset.
	APIBaseURL string

	AgentEmail    string
	AgentPassword string

	// AuthToken menjaga MCP itu sendiri.
	//
	// Server ini memegang sandi agent, sehingga siapa pun yang menjangkaunya
	// ADALAH agent — dan agent boleh menghapus dokumen serta aset. Tanpa penjaga
	// di sini, wewenang itu menjadi publik begitu mcp.widiakencana.com terbit.
	AuthToken string

	HTTPTimeout time.Duration
}

// LoadConfig membaca env, lalu MENOLAK berdiri bila ada yang kurang.
//
// Tidak ada nilai bawaan untuk kredensial maupun token penjaga. Bawaan pada hal
// semacam itu hanya menghasilkan server yang menyala dan gagal belakangan
// dengan pesan yang tidak menyebut sebabnya — atau lebih buruk, menyala tanpa
// penjaga sama sekali.
func LoadConfig() (Config, error) {
	cfg := Config{
		Port:          env("MCP_PORT", "9090"),
		APIBaseURL:    strings.TrimRight(env("API_BASE_URL", "http://127.0.0.1:8080"), "/"),
		AgentEmail:    env("AGENT_EMAIL", ""),
		AgentPassword: env("AGENT_PASSWORD", ""),
		AuthToken:     env("MCP_AUTH_TOKEN", ""),
		HTTPTimeout:   15 * time.Second,
	}

	var kurang []string
	if cfg.AgentEmail == "" {
		kurang = append(kurang, "AGENT_EMAIL")
	}
	if cfg.AgentPassword == "" {
		kurang = append(kurang, "AGENT_PASSWORD")
	}
	if cfg.AuthToken == "" {
		kurang = append(kurang, "MCP_AUTH_TOKEN")
	}
	if len(kurang) > 0 {
		return Config{}, fmt.Errorf("konfigurasi wajib belum diisi: %s", strings.Join(kurang, ", "))
	}

	if !strings.HasPrefix(cfg.APIBaseURL, "http://") && !strings.HasPrefix(cfg.APIBaseURL, "https://") {
		return Config{}, errors.New("API_BASE_URL harus berawalan http:// atau https://")
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}
