package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Client adalah satu-satunya jalan MCP menyentuh API.
//
// Ia memegang sesi agent dan menyegarkannya sendiri. Yang di luar berkas ini
// cukup memanggil Do dan tidak pernah memikirkan token.
type Client struct {
	baseURL  string
	email    string
	password string
	http     *http.Client
	logger   *slog.Logger

	// mu menjaga token dan tenggatnya. Tool MCP dilayani bersamaan, dan tanpa
	// ini dua pemanggilan yang tiba saat token kedaluwarsa akan login dua kali
	// lalu saling menimpa hasilnya.
	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// New menerima nilai yang ia pakai, BUKAN struct Config milik paket induk.
//
// Dua sebab. Pertama arah impor: kalau ia menerima mcp.Config, paket ini
// menunjuk balik ke induknya dan tool tidak akan pernah bisa memakainya tanpa
// siklus. Kedua kejujuran: klien API tidak berkepentingan pada port MCP maupun
// token penjaganya, dan parameter yang menyebutkan apa yang benar-benar dipakai
// membuat itu terbaca dari tanda tangannya.
func New(baseURL, email, password string, timeout time.Duration, logger *slog.Logger) *Client {
	return &Client{
		baseURL:  baseURL,
		email:    email,
		password: password,
		http:     &http.Client{Timeout: timeout},
		logger:   logger,
	}
}

// tokenLeeway adalah jarak aman sebelum tenggat token.
//
// Token yang tinggal beberapa detik akan kedaluwarsa DI TENGAH permintaan yang
// memakainya, dan kegagalannya muncul sebagai 401 yang membingungkan alih-alih
// sebagai perpanjangan yang wajar.
const tokenLeeway = 60 * time.Second

// accessToken mengembalikan token yang masih berlaku, login bila perlu.
//
// Login DITUNDA sampai dibutuhkan, bukan dilakukan saat start. MCP dan API kerap
// dinyalakan bersamaan, dan server yang menolak berdiri hanya karena API-nya
// belum siap akan gagal justru pada susunan yang paling umum.
func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" && time.Now().Add(tokenLeeway).Before(c.expiresAt) {
		return c.token, nil
	}

	return c.loginLocked(ctx)
}

// loginLocked menukar sandi dengan access token.
//
// TIDAK memakai refresh token, dan itu keputusan sadar. Refresh ada supaya
// browser tidak perlu menanyakan sandi lagi kepada manusia; MCP memegang
// sandinya sendiri, jadi ia tidak mendapat apa pun dari sana selain wadah cookie
// dan pembukuan rotasi yang bisa basi. Login ulang lebih murah daripada
// keduanya.
//
// Ini juga yang membuat MCP selamat dari restart API: penyimpanan sesi ada di
// memori proses, sehingga setiap deploy menghapus seluruh sesi. Klien yang hanya
// memegang token akan mati bersamanya; yang memegang sandi berdiri lagi sendiri.
func (c *Client) loginLocked(ctx context.Context) (string, error) {
	payload, err := json.Marshal(map[string]string{
		"email":    c.email,
		"password": c.password,
	})
	if err != nil {
		return "", fmt.Errorf("susun muatan login: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/login", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("susun permintaan login: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("hubungi API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Isi balasan TIDAK ikut dicatat maupun dikembalikan. Ia dapat memuat
		// pantulan muatan yang barusan dikirim, dan muatan itu berisi sandi.
		return "", fmt.Errorf("login ditolak API dengan status %d", resp.StatusCode)
	}

	var body struct {
		Data struct {
			Auth struct {
				AccessToken string `json:"access_token"`
				ExpiredAt   int64  `json:"expired_at"`
			} `json:"auth"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("urai balasan login: %w", err)
	}
	if body.Data.Auth.AccessToken == "" {
		return "", fmt.Errorf("balasan login tidak memuat access_token")
	}

	c.token = body.Data.Auth.AccessToken
	c.expiresAt = time.Unix(body.Data.Auth.ExpiredAt, 0)
	c.logger.Info("mcp masuk sebagai agent",
		"email", c.email,
		"berlaku_sampai", c.expiresAt.Format(time.RFC3339))

	return c.token, nil
}

// invalidate membuang token yang ternyata sudah tidak diterima API.
//
// Dipanggil setelah 401. Tokennya boleh jadi masih jauh dari tenggat menurut
// jam MCP, tetapi sesinya di server sudah lenyap — restart API menghapus seluruh
// sesi, dan itu terjadi setiap deploy.
func (c *Client) invalidate(stale string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Hanya dibuang bila belum ada yang menggantinya. Tanpa perbandingan ini,
	// dua permintaan yang sama-sama kena 401 akan membuang token BARU yang
	// diperoleh salah satunya, dan keduanya berputar login tanpa henti.
	if c.token == stale {
		c.token = ""
		c.expiresAt = time.Time{}
	}
}

// Do mengirim satu permintaan ke API atas nama agent.
//
// Menyisipkan Authorization sendiri, dan MENCOBA ULANG SEKALI pada 401. Percobaan
// ulang itu bukan kemewahan: sesi hidup di memori proses API, jadi tiap restart
// membuat token yang tadinya sah menjadi tidak dikenal. Tanpa ini, setiap deploy
// menuntut MCP ikut dijalankan ulang.
//
// Sekali, bukan berulang. Dua kali 401 berturut-turut berarti sandinya memang
// tidak diterima, dan mengulanginya hanya mengubah kesalahan konfigurasi menjadi
// banjir permintaan login.
func (c *Client) Do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	resp, token, err := c.kirim(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	resp.Body.Close()
	c.invalidate(token)
	c.logger.Warn("sesi agent ditolak API, mencoba masuk ulang", "path", path)

	resp, _, err = c.kirim(ctx, method, path, body)

	return resp, err
}

// kirim mengembalikan token yang dipakai, supaya pemanggil dapat membuang
// TEPAT token itu bila ditolak — bukan token yang mungkin sudah diperbarui
// goroutine lain di antaranya.
func (c *Client) kirim(ctx context.Context, method, path string, body []byte) (*http.Response, string, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, "", err
	}

	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, "", fmt.Errorf("susun permintaan %s %s: %w", method, path, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("hubungi API: %w", err)
	}

	return resp, token, nil
}

// DesignTicket adalah izin sekali pakai untuk membuka socket penyuntingan.
//
// Umurnya PENDEK — tiga puluh detik menurut API — dan hangus setelah dipakai
// sekali. Karena itu ia diterbitkan tepat sebelum menyambung, bukan di awal
// tugas: menerbitkannya lalu menunggu model berpikir akan selalu gagal.
type DesignTicket struct {
	Ticket    string `json:"ticket"`
	ExpiresIn int64  `json:"expires_in"`
}

func (c *Client) IssueDesignTicket(ctx context.Context, documentToken string) (DesignTicket, error) {
	resp, err := c.Do(ctx, http.MethodPost, "/api/document-design-ticket/"+url.PathEscape(documentToken), nil)
	if err != nil {
		return DesignTicket{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return DesignTicket{}, fmt.Errorf("penerbitan tiket menjawab %d", resp.StatusCode)
	}

	var body struct {
		Data struct {
			DesignTicket DesignTicket `json:"design_ticket"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return DesignTicket{}, fmt.Errorf("urai balasan tiket: %w", err)
	}
	if body.Data.DesignTicket.Ticket == "" {
		return DesignTicket{}, fmt.Errorf("balasan tiket tidak memuat ticket")
	}

	return body.Data.DesignTicket, nil
}

// SocketURL menyusun alamat WebSocket dari APIBaseURL yang sama.
//
// Diturunkan, bukan disetel terpisah: HTTP dan WebSocket dilayani proses serta
// port yang sama, dan dua variabel yang wajib menunjuk server yang sama adalah
// dua kesempatan untuk meleset.
func (c *Client) SocketURL(documentToken, ticket string) string {
	skema := "ws"
	if strings.HasPrefix(c.baseURL, "https://") {
		skema = "wss"
	}
	host := strings.TrimPrefix(strings.TrimPrefix(c.baseURL, "https://"), "http://")

	return fmt.Sprintf("%s://%s/document-design/%s?ticket=%s",
		skema, host, url.PathEscape(documentToken), url.QueryEscape(ticket))
}

// Identity adalah jawaban /api/me — dipakai memastikan MCP benar-benar masuk
// sebagai akun yang dimaksud, bukan sekadar berhasil login sebagai siapa saja.
type Identity struct {
	// userID, bukan user_id. Nama field mengikuti apa yang API BENAR-BENAR
	// kirim, bukan yang tampak wajar — salah satu huruf saja menghasilkan
	// string kosong tanpa galat apa pun, dan itu ketahuan hanya karena diuji.
	UserID string `json:"userID"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

// Authenticate memeriksa sandi SEORANG PENGGUNA, bukan sandi agent.
//
// Terpisah dari seluruh jalur di atas dan sengaja tidak menyentuh token agent:
// ia dipakai alur OAuth untuk membuktikan bahwa yang berdiri di depan formulir
// memang pemilik akun. Menumpangkannya pada sesi agent akan menukar sesi yang
// dipakai seluruh tool hanya karena ada orang mencoba login.
//
// Sesi yang lahir dari sini DITINGGALKAN, tidak di-logout. Logout menuntut
// refresh token yang hanya ada di cookie, dan mengejarnya berarti menyusun
// wadah cookie hanya untuk membuang sesi yang toh akan kedaluwarsa sendiri.
// Yang perlu diketahui: setiap login OAuth yang berhasil meninggalkan satu sesi
// menganggur di API sampai tenggatnya lewat.
func (c *Client) Authenticate(ctx context.Context, email, password string) (Identity, error) {
	payload, err := json.Marshal(map[string]string{"email": email, "password": password})
	if err != nil {
		return Identity{}, fmt.Errorf("susun muatan login: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/login", bytes.NewReader(payload))
	if err != nil {
		return Identity{}, fmt.Errorf("susun permintaan login: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Identity{}, fmt.Errorf("hubungi API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Sekali lagi tanpa isi balasan: ia dapat memantulkan muatan yang
		// barusan dikirim, dan muatan itu berisi sandi orang lain.
		return Identity{}, fmt.Errorf("login ditolak API dengan status %d", resp.StatusCode)
	}

	var body struct {
		Data struct {
			Auth struct {
				AccessToken string `json:"access_token"`
			} `json:"auth"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Identity{}, fmt.Errorf("urai balasan login: %w", err)
	}
	if body.Data.Auth.AccessToken == "" {
		return Identity{}, fmt.Errorf("balasan login tidak memuat access_token")
	}

	return c.identitasDengan(ctx, body.Data.Auth.AccessToken)
}

// identitasDengan menanyakan /api/me memakai token tertentu.
func (c *Client) identitasDengan(ctx context.Context, token string) (Identity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/me", nil)
	if err != nil {
		return Identity{}, fmt.Errorf("susun permintaan /api/me: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return Identity{}, fmt.Errorf("hubungi API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Identity{}, fmt.Errorf("/api/me menjawab %d", resp.StatusCode)
	}

	var body struct {
		Data struct {
			User Identity `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Identity{}, fmt.Errorf("urai balasan /api/me: %w", err)
	}

	return body.Data.User, nil
}

func (c *Client) Me(ctx context.Context) (Identity, error) {
	resp, err := c.Do(ctx, http.MethodGet, "/api/me", nil)
	if err != nil {
		return Identity{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Identity{}, fmt.Errorf("/api/me menjawab %d", resp.StatusCode)
	}

	var body struct {
		Data struct {
			User Identity `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Identity{}, fmt.Errorf("urai balasan /api/me: %w", err)
	}

	return body.Data.User, nil
}
