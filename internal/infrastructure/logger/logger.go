package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New membuat logger aplikasi pada level yang diminta.
//
// Level dapat diatur karena sebagian catatan hanya berguna saat menelusuri
// masalah dan justru menenggelamkan log bila selalu menyala — jejak pesan
// WebSocket per klien, misalnya.
func New(level string) *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(level),
	}))
}

// parseLevel jatuh ke Info untuk nilai yang tidak dikenal. Salah ketik pada
// konfigurasi tidak seharusnya membungkam aplikasi.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
