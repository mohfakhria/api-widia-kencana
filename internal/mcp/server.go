package mcp

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

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
	cfg    Config
	api    *APIClient
	mcp    *mcp.Server
	logger *slog.Logger
}

func NewServer(cfg Config, api *APIClient, logger *slog.Logger) *Server {
	return &Server{cfg: cfg, api: api, mcp: newMCPServer(api), logger: logger}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// health TIDAK dijaga token. Ia dituju pemeriksa kesehatan dan skrip deploy,
	// yang tidak semestinya memegang kredensial — dan ia tidak menyentuh API
	// maupun membocorkan apa pun tentang agent.
	mux.HandleFunc("GET /health", s.health)

	// whoami menyentuh API sungguhan atas nama agent, jadi ia DIJAGA.
	mux.Handle("GET /whoami", s.requireToken(http.HandlerFunc(s.whoami)))

	// Satu instance Server dipakai bersama seluruh sesi MCP. SDK menyatakan ini
	// sah — getServer boleh mengembalikan server yang sama berulang kali — dan
	// itu yang kita mau: daftar tool sama bagi semua pemanggil, dan tidak ada
	// keadaan per sesi yang perlu dijaga.
	streamable := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return s.mcp },
		nil,
	)

	// Penjaga yang SAMA dengan rute lain. Protokol MCP tidak membawa penjaganya
	// sendiri di sini — dan tanpa baris ini, seluruh tool terbuka bagi siapa pun
	// yang menemukan host ini.
	mux.Handle("/mcp", s.requireToken(streamable))

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

// requireToken menjaga MCP itu sendiri.
//
// Server ini memegang sandi agent, sehingga siapa pun yang menjangkaunya ADALAH
// agent. Pembandingnya waktu-tetap: perbandingan biasa berhenti pada byte
// pertama yang berbeda, dan selisih waktunya cukup untuk menebak token satu byte
// demi satu byte.
func (s *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		diberikan := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(diberikan), []byte(s.cfg.AuthToken)) != 1 {
			tulisJSON(w, http.StatusUnauthorized, map[string]any{
				"status":  "error",
				"message": "token MCP tidak sah",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
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
