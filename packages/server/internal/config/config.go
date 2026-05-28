package config

import (
	"os"
)

type Config struct {
	Port   string
	JWT    JWTConfig
	DB     DBConfig
}

type JWTConfig struct {
	Secret string
}

type DBConfig struct {
	Type string // sqlite | mysql | postgres | mongo
	DSN  string // optional full DSN; otherwise built from individual env vars
}

func Load() Config {
	return Config{
		Port: getEnv("PORT", "8080"),
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "change-me-in-production"),
		},
		DB: DBConfig{
			Type: getEnv("DB_TYPE", "sqlite"),
			DSN:  os.Getenv("DB_DSN"),
		},
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
