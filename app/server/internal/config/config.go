package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"feedchamber/internal/pkg/logger"
	"feedchamber/internal/storage"

	log "github.com/sirupsen/logrus"
)

type Config struct {
	Port string
	DB   DBConfig
	// Storage 存储配置：启动时由 pkg.BuildStorageConfig 以 settings 表为准解析后
	// 写入（缺省回退环境变量 STORAGE_*/S3_*），不在 Load() 中直接读取环境变量，
	// 以保证 S3/存储凭证只有「settings 表 → 环境变量」一条可信链路。
	Storage        storage.Config
	KV             KVConfig
	Log            logger.Config // LOG_LEVEL / LOG_FORMAT / LOG_OUTPUT
	MetricsEnabled bool          // METRICS_ENABLED, default true
	MetricsPort    int           // METRICS_PORT, default 9091（内部指标端口，不经 nginx 暴露）
	TracingEnabled bool          // TRACING_ENABLED, default true
}

type KVConfig struct {
	NATSURL    string // 环境变量 NATS_URL（必填）：外部 NATS，需启用 JetStream
	NATSBucket string // 环境变量 NATS_BUCKET，默认 "feedchamber_kv"
}

type DBConfig struct {
	Type            string // sqlite | mysql | postgres
	DSN             string // optional full DSN; otherwise built from individual env vars
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int // seconds
	ConnMaxIdleTime int // seconds
}

func Load() Config {
	return Config{
		Port: getEnv("PORT", "8080"),
		DB: DBConfig{
			Type:            getEnv("DB_TYPE", "sqlite"),
			DSN:             os.Getenv("DB_DSN"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: getEnvInt("DB_CONN_MAX_LIFETIME", 300),
			ConnMaxIdleTime: getEnvInt("DB_CONN_MAX_IDLE_TIME", 60),
		},
		// Storage 留空：启动时由 pkg.BuildStorageConfig 从 settings 表（回退环境变量）解析。
		KV: KVConfig{
			NATSURL:    os.Getenv("NATS_URL"),
			NATSBucket: getEnv("NATS_BUCKET", "feedchamber_kv"),
		},
		Log: logger.Config{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
			Output: getEnv("LOG_OUTPUT", "stderr"),
		},
		MetricsEnabled: getEnvBool("METRICS_ENABLED", true),
		MetricsPort:    getEnvInt("METRICS_PORT", 9091),
		TracingEnabled: getEnvBool("TRACING_ENABLED", true),
	}
}

func (c Config) Validate() error {
	if err := c.DB.Validate(); err != nil {
		return err
	}
	// NATS 是外部依赖（内嵌实现已剥离），空值必须在此拦下：
	// 否则会拖到 kvstore.New 才失败，日志里只剩连接错误、看不出是漏配。
	if strings.TrimSpace(c.KV.NATSURL) == "" {
		return fmt.Errorf("NATS_URL is required: KV Store and EventBus need an external NATS with JetStream enabled")
	}
	if c.MetricsEnabled && (c.MetricsPort <= 0 || c.MetricsPort > 65535) {
		return fmt.Errorf("METRICS_PORT must be between 1 and 65535 when METRICS_ENABLED=true")
	}
	return nil
}

func (c DBConfig) Validate() error {
	dbType := strings.TrimSpace(c.Type)
	if dbType == "" {
		return fmt.Errorf("DB_TYPE is required")
	}

	switch dbType {
	case "sqlite":
		return nil
	case "mysql", "postgres":
		if strings.TrimSpace(c.DSN) == "" {
			return fmt.Errorf("DB_DSN is required when DB_TYPE=%s", dbType)
		}
		return nil
	default:
		return fmt.Errorf("unsupported DB_TYPE %q", dbType)
	}
}

func getEnv(key, def string) string {
	// Empty string is treated as unset here so defaults remain usable when a
	// variable is present but blank in .env or container configuration.
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		log.Warnf("config: invalid int for %s=%q, using default %d", key, v, def)
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
		log.Warnf("config: invalid bool for %s=%q, using default %v", key, v, def)
	}
	return def
}
