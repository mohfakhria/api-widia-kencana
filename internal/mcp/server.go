package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/mcp/apiclient"
	"github.com/mohfakhria/api-widia-kencana/internal/mcp/oauth"
	"github.com/mohfakhria/api-widia-kencana/internal/mcp/session"
	"github.com/mohfakhria/api-widia-kencana/internal/mcp/tool"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server adalah gerbang MCP.
//
// Ia melayani dua permukaan sekaligus, dan itu disengaja:
//
//   - /mcp — protokol MCP yang sebenarnya, dipakai GPT dan Claude
//   - /health dan /whoami — HTTP biasa, untuk pemeriksa kesehatan dan untuk
//     menelusuri rantainya TANPA klien MCP
//
// Yang kedua tidak menjadi mubazir setelah yang pertama ada. Ketika sesuatu
// rusak, membedakan "protokol MCP bermasalah" dari "agent tidak dapat masuk ke
// API" jauh lebih cepat lewat satu curl daripada lewat klien LLM.
type Server struct {
	cfg      Config
	api      *apiclient.Client
	mcp      *mcp.Server
	sessions *session.Manager
	oauth    *oauth.Server
	logger   *slog.Logger
}

// serverName dan serverVersion diumumkan ke klien saat initialize. Klien
// menampilkannya kepada penggunanya, jadi keduanya nama yang dibaca manusia.
const (
	serverName    = "widia-kencana"
	serverVersion = "0.1.0"
)

// NewServer merakit seluruhnya dari satu tempat.
//
// Klien API dan pengelola sesi dibuat DI SINI, bukan diterima dari luar: hanya
// server ini yang membutuhkannya, dan membuatnya di sini membuat cmd/mcp tetap
// sesederhana "baca konfigurasi, jalankan".
func NewServer(cfg Config, logger *slog.Logger) (*Server, error) {
	api := apiclient.New(cfg.APIBaseURL, cfg.AgentEmail, cfg.AgentPassword, cfg.HTTPTimeout, logger)
	sessions := session.NewManager(api, logger)

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    serverName,
		Version: serverVersion,
	}, nil)
	if err := tool.Register(mcpServer, tool.Deps{API: api, Sessions: sessions}); err != nil {
		return nil, fmt.Errorf("daftarkan tool: %w", err)
	}

	// Adaptornya di sini, bukan di paket oauth. Paket itu sengaja tidak mengenal
	// klien API — yang ia butuhkan hanyalah satu pertanyaan, "sandi ini milik
	// siapa", dan menerimanya sebagai fungsi membuatnya dapat diuji serta
	// diganti tanpa menyeret seluruh klien HTTP.
	otentikasi := func(ctx context.Context, email, sandi string) (oauth.Subject, error) {
		identity, err := api.Authenticate(ctx, email, sandi)
		if err != nil {
			return oauth.Subject{}, err
		}

		// Email diambil dari yang DIKETIK, karena /api/me tidak mengembalikannya.
		// Aman: kredensialnya baru saja diterima API, jadi alamat ini memang
		// milik akun yang bersangkutan.
		return oauth.Subject{
			UserID: identity.UserID,
			Email:  email,
			Name:   identity.Name,
			Role:   identity.Role,
		}, nil
	}

	return &Server{
		cfg:      cfg,
		api:      api,
		mcp:      mcpServer,
		sessions: sessions,
		oauth: oauth.NewServer(oauth.Config{
			Issuer:   cfg.PublicURL,
			Resource: cfg.OAuthResource(),
		}, otentikasi, logger),
		logger: logger,
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// health TIDAK dijaga token. Ia dituju pemeriksa kesehatan dan skrip deploy,
	// yang tidak semestinya memegang kredensial — dan ia tidak menyentuh API
	// maupun membocorkan apa pun tentang agent.
	mux.HandleFunc("GET /health", s.health)

	// Titik ujung OAuth: metadata, pendaftaran klien, halaman login, dan
	// penukaran token. Seluruhnya TERBUKA tanpa token — lihat alasannya di
	// oauth.Routes — dan itu justru yang membuat konektor dapat menemukan
	// jalannya sendiri dari sebuah 401.
	s.oauth.Routes(mux)

	// whoami menyentuh API sungguhan atas nama agent, jadi ia DIJAGA — oleh
	// penjaga yang sama dengan /mcp, karena kini hanya ada satu.
	//
	// Dibiarkan terbuka bukan pilihan meski isinya sepele: ia memanggil /api/me
	// pada SETIAP permintaan, sehingga ia sekaligus menyebutkan identitas agent
	// kepada siapa pun dan menjadi cara murah membebani API dari luar.
	mux.Handle("GET /whoami", s.requireOAuth(http.HandlerFunc(s.whoami)))

	// Satu instance Server dipakai bersama seluruh sesi MCP. SDK menyatakan ini
	// sah — getServer boleh mengembalikan server yang sama berulang kali — dan
	// itu yang kita mau: daftar tool sama bagi semua pemanggil, dan tidak ada
	// keadaan per sesi yang perlu dijaga.
	streamable := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s.mcp },
		&mcp.StreamableHTTPOptions{
			// Stateless BUKAN pilihan gaya. SDK hanya menawarkan protokol
			// 2026-07-28 ketika transport disetel begini — lihat
			// StreamableServerTransport.SupportsProtocolVersion — dan konektor
			// ChatGPT menuntut versi itu lewat server/discover. Tanpa baris ini,
			// server menjawab discover dengan benar tetapi mengumumkan daftar
			// versi yang tidak memuat 2026-07-28, dan ChatGPT berhenti di situ
			// tanpa pernah memanggil initialize. Gejalanya: "Error refreshing
			// actions", tanpa satu pun galat di sisi kita.
			//
			// Cocok karena kita memang tidak menyimpan apa pun per sesi MCP:
			// satu mcp.Server dipakai seluruh pemanggil, dan sesi dokumen dikunci
			// oleh TOKEN DOKUMEN, bukan oleh sesi MCP. Yang hilang bersama mode
			// ini — Mcp-Session-Id, GET/DELETE pada /mcp, dan permintaan dari
			// server ke klien — tidak satu pun kita pakai.
			//
			// Bila suatu saat ada tool yang perlu bertanya balik ke model
			// (sampling atau elicitation), keputusan ini harus ditinjau ulang:
			// permintaan server ke klien ditolak seketika dalam mode ini.
			Stateless: true,

			// Perlindungan bawaan SDK menolak permintaan yang DATANG dari
			// localhost tetapi membawa Host bukan-localhost. Ia menjaga server
			// MCP lokal dari DNS rebinding: situs jahat mengarahkan sebuah nama
			// ke 127.0.0.1, lalu browser korban menembak server itu dengan Host
			// milik penyerang.
			//
			// Di susunan kita, heuristik itu SELALU berbunyi untuk lalu lintas
			// yang sah: nginx meneruskan dari 127.0.0.1 sambil membawa Host
			// publik apa adanya. Yang ditolak justru satu-satunya jalur yang
			// benar.
			//
			// Dimatikan karena model ancamannya tidak berlaku di sini, dan
			// ketiganya harus TETAP benar agar keputusan ini tetap benar:
			//
			//   1. ufw menutup 9090; satu-satunya jalan masuk lewat nginx
			//   2. nginx yang mengakhiri TLS
			//   3. /mcp menuntut token OAuth
			//
			// Serangan yang dijaga perlindungan ini menuntut browser korban
			// menjangkau server secara LANGSUNG. Selama nomor 1 berlaku, itu
			// tidak mungkin. Bila suatu saat 9090 dibuka di firewall, baris ini
			// harus ditinjau ulang lebih dulu.
			//
			// Menulis ulang Host di nginx menjadi localhost juga akan lolos, dan
			// sengaja TIDAK dipilih: ia membuat aplikasi tidak mengetahui namanya
			// sendiri, sedangkan OAuth kelak menuntut MCP menyebutkan URL
			// publiknya pada metadata resource.
			DisableLocalhostProtection: true,
		},
	)

	// Penjaga yang SAMA dengan rute lain. Protokol MCP tidak membawa penjaganya
	// sendiri di sini — dan tanpa baris ini, seluruh tool terbuka bagi siapa pun
	// yang menemukan host ini.
	mux.Handle("/mcp", s.requireOAuth(streamable))

	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	tulisJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// whoami membuktikan rantainya utuh: MCP masuk sebagai agent, API mengenalinya,
// dan perannya memang ai-agent.
//
// Ini yang membedakannya dari sekadar "login berhasil". Login yang berhasil
// sebagai akun yang KELIRU akan terlihat sama persis sampai jauh kemudian,
// ketika sesuatu ditolak barier peran tanpa menyebut sebabnya.
func (s *Server) whoami(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.HTTPTimeout)
	defer cancel()

	identity, err := s.api.Me(ctx)
	if err != nil {
		s.logger.Error("gagal mengambil identitas agent", "error", err)
		tulisJSON(w, http.StatusBadGateway, map[string]any{
			"status":  "error",
			"message": "tidak dapat menghubungi API sebagai agent",
		})
		return
	}

	tulisJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"agent":  identity,
	})
}

// requireOAuth adalah SATU-SATUNYA penjaga di server ini.
//
// Sebelumnya ada dua: OAuth, dan MCP_AUTH_TOKEN — rahasia bersama peninggalan
// masa sebelum OAuth. Yang kedua dicabut seluruhnya, bukan sekadar dipersempit,
// karena ia lemah pada tiga hal sekaligus: ia melewati seluruh alur
// persetujuan, tidak dapat dicabut per klien — mencabutnya berarti restart,
// yang memutus semua konektor OAuth sekaligus — dan tidak menyebutkan siapa
// pemakainya di catatan.
//
// Yang menghapus alasan terakhirnya untuk hidup: konektor GPT maupun Claude
// TIDAK PERNAH memakainya. Tidak ada tempat menempelkan token statis di alur
// mereka; yang diberikan hanya URL, dan sisanya mereka temukan sendiri dari
// 401. Token itu karenanya hanya melayani kenyamanan kita di baris perintah,
// dan itu tidak sebanding dengan rahasia tetap yang harus dipelihara.
//
// Akibatnya satu dan perlu diketahui: tidak ada lagi jalur darurat tanpa
// peramban. Bila yang rusak justru alur otorisasinya sendiri, yang tersisa
// adalah /health, log systemd, dan restart.
func (s *Server) requireOAuth(next http.Handler) http.Handler {
	// Disusun SEKALI, di luar handler. Menyusunnya per permintaan berarti
	// membangun ulang rantai middleware pada setiap panggilan tool.
	return auth.RequireBearerToken(s.oauth.Verifier(), &auth.RequireBearerTokenOptions{
		// Inilah yang membuat 401 dapat ditindaklanjuti sendiri oleh klien: ia
		// menunjuk ke metadata yang menyebutkan authorization server-nya, dan
		// dari situ konektor memulai alur tanpa seorang pun menyetel apa pun.
		ResourceMetadataURL: s.oauth.ResourceMetadataURL(),
	})(next)
}

// Run melayani sampai ctx berakhir, lalu berhenti dengan tertib.
//
// Konteksnya datang dari sinyal, sama seperti API. Berhenti mendadak akan
// memutus permintaan yang sedang dilayani di tengah jalan, dan pada tahap
// berikutnya — ketika MCP memegang socket penyuntingan — itu berarti suntingan
// yang menggantung.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              ":" + s.cfg.Port,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Penyapu sesi ikut hidup selama server, dan ikut mati bersamanya. Ia yang
	// menutup socket yang menganggur — dan, saat shutdown, seluruhnya sekaligus,
	// supaya agent tidak tertinggal sebagai penghuni hantu di kanvas orang.
	go s.sessions.Run(ctx)

	// Penyapu OAuth membuang kode, token, dan catatan percobaan yang sudah
	// lewat tenggat. Tanpa ia, peta di dalamnya hanya tumbuh selama proses hidup.
	go s.oauth.Run(ctx)

	berhenti := make(chan error, 1)
	go func() {
		s.logger.Info("mcp mendengarkan",
			"port", s.cfg.Port,
			"api", s.cfg.APIBaseURL,
			"agent", s.cfg.AgentEmail)
		berhenti <- srv.ListenAndServe()
	}()

	select {
	case err := <-berhenti:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		matiCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()

		return srv.Shutdown(matiCtx)
	}
}

func tulisJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
