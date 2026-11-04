package app

import (
	"os"
	"strconv"
	"strings"
)

// Config is everything the server needs from the environment.
type Config struct {
	Addr         string
	DBPath       string
	WebDir       string
	Origins      []string
	CookieSecure bool
	Issuer       string
	TrustProxy   bool
}

func LoadConfig() Config {
	cfg := Config{
		Addr:         env("ADDR", ":8080"),
		DBPath:       env("DB_PATH", "./data/icinidok.db"),
		WebDir:       env("WEB_DIR", "./web"),
		Issuer:       env("TOTP_ISSUER", "içini dök."),
		CookieSecure: envBool("COOKIE_SECURE", false),
		TrustProxy:   envBool("TRUST_PROXY", false),
	}
	for _, origin := range strings.Split(env("APP_ORIGIN", "http://localhost:8080,http://localhost:3000"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			cfg.Origins = append(cfg.Origins, strings.TrimRight(origin, "/"))
		}
	}
	return cfg
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
