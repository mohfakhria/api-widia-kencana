package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv       string
	AppPort      string
	AppBaseURL   string
	FrontendURL  string
	PGHost       string
	PGPort       string
	PGUser       string
	PGPassword   string
	PGDB         string
	RedisEnabled bool
	RedisHost    string
	RedisPort    string
	JWTSecret    string
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Println("warning: .env file not found, using system env")
	}

	return Config{
		AppEnv:       getEnv("APP_ENV", "local"),
		AppPort:      getEnv("APP_PORT", "8080"),
		AppBaseURL:   getEnv("APP_BASEURL", "http://localhost:8080"),
		FrontendURL:  getEnv("FRONTEND_URL", "http://localhost:3000"),
		PGHost:       getEnv("PG_HOST", "localhost"),
		PGPort:       getEnv("PG_PORT", "5432"),
		PGUser:       getEnv("PG_USER", "postgres"),
		PGPassword:   getEnv("PG_PASSWORD", "postgres"),
		PGDB:         getEnv("PG_DB", "postgres"),
		RedisEnabled: isRedisEnabled(),
		RedisHost:    getEnv("REDIS_HOST", "localhost"),
		RedisPort:    getEnv("REDIS_PORT", "6379"),
		JWTSecret:    getEnv("JWT_SECRET", "change-this-in-env"),
	}
}

func (c Config) Address() string {
	return ":" + c.AppPort
}

func (c Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

func (c Config) CookieSecure() bool {
	return c.IsProduction()
}

func (c Config) CookieDomain() string {
	return "localhost"
}

func (c Config) RedisAddr() string {
	return c.RedisHost + ":" + c.RedisPort
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

func isRedisEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv("REDIS_ENABLED")))
	return value != "false" && value != "0" && value != "no"
}
