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
	AppEnv              string
	AppPort             string
	AppBaseURL          string
	FrontendURL         string
	PGHost              string
	PGPort              string
	PGUser              string
	PGPassword          string
	PGDB                string
	CookieDomain        string
	CookieSecure        bool
	JWTSecret           string
	JWTSubEncryptionKey string
	MinIOEndpoint       string
	MinIORootUser       string
	MinIORootPassword   string
	MinIOBucket         string
	MinIOUseSSL         bool
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("warning: .env file not found, using system env")
	}

	appBaseURL := getEnv("APP_BASEURL", "http://localhost:8080")

	return Config{
		AppEnv:      getEnv("APP_ENV", "local"),
		AppPort:     getEnv("APP_PORT", "8080"),
		AppBaseURL:  appBaseURL,
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
		PGHost:      getEnv("PG_HOST", "localhost"),
		PGPort:      getEnv("PG_PORT", "5432"),
		PGUser:      getEnv("PG_USER", "postgres"),
		PGPassword:  getEnv("PG_PASSWORD", "postgres"),
		PGDB:        getEnv("PG_DB", "postgres"),

		// Domain kosong membuat cookie host-only: terikat persis ke host yang
		// men-set-nya, dan benar tanpa dikonfigurasi baik di localhost maupun
		// production. Isi COOKIE_DOMAIN hanya bila cookie memang perlu dibagi
		// ke beberapa subdomain.
		CookieDomain: getEnv("COOKIE_DOMAIN", ""),

		// Cookie hanya boleh melintas HTTPS saat API dilayani lewat HTTPS.
		// COOKIE_SECURE dipakai bila TLS diterminasi di proxy dan APP_BASEURL
		// terlanjur menunjuk ke alamat internal http://.
		CookieSecure: getBoolEnv("COOKIE_SECURE", isHTTPSURL(appBaseURL)),

		JWTSecret:           getEnv("JWT_SECRET", "change-this-in-env"),
		JWTSubEncryptionKey: getEnv("JWT_SUB_ENCRYPTION_KEY", ""),
		MinIOEndpoint:       getEnv("MINIO_ENDPOINT", "localhost:9002"),
		MinIORootUser:       getEnv("MINIO_ROOT_USER", ""),
		MinIORootPassword:   getEnv("MINIO_ROOT_PASSWORD", ""),
		MinIOBucket:         getEnv("MINIO_BUCKET", "widia-assets"),
		MinIOUseSSL:         getBoolEnv("MINIO_USE_SSL", false),
	}
}

func (c Config) Address() string {
	return ":" + c.AppPort
}

func (c Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

// Validate menolak kombinasi konfigurasi yang diam-diam melemahkan keamanan.
func (c Config) Validate() error {
	if c.IsProduction() && !c.CookieSecure {
		return errors.New("APP_ENV=production membutuhkan refresh cookie yang Secure: arahkan APP_BASEURL ke URL https://, atau set COOKIE_SECURE=true bila TLS diterminasi di proxy")
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

func isHTTPSURL(raw string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "https://")
}

func getBoolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value != "false" && value != "0" && value != "no"
}
