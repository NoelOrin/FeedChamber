package config

import (
	"os"

	"orinforum/internal/storage"
)

type Config struct {
	Port    string
	JWT     JWTConfig
	DB      DBConfig
	Storage storage.Config
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
		Storage: storage.Config{
			Provider:        getEnv("STORAGE_PROVIDER", "local"),
			LocalDir:        getEnv("STORAGE_LOCAL_DIR", "uploads"),
			Endpoint:        os.Getenv("S3_ENDPOINT"),
			Region:          getEnv("S3_REGION", "us-east-1"),
			Bucket:          os.Getenv("S3_BUCKET"),
			AccessKeyID:     os.Getenv("S3_ACCESS_KEY_ID"),
			AccessKeySecret: os.Getenv("S3_ACCESS_KEY_SECRET"),
			UseSSL:          getEnv("S3_USE_SSL", "true") == "true",
			CustomDomain:    os.Getenv("S3_CUSTOM_DOMAIN"),
		},
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
