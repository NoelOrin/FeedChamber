package config

import "testing"

func TestDBConfigValidateAllowsSQLiteDefaultDSN(t *testing.T) {
	cfg := DBConfig{Type: "sqlite"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDBConfigValidateRequiresDSNForExternalBackends(t *testing.T) {
	for _, dbType := range []string{"mysql", "postgres"} {
		t.Run(dbType, func(t *testing.T) {
			cfg := DBConfig{Type: dbType}

			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want DB_DSN error")
			}
		})
	}
}

func TestDBConfigValidateRejectsUnknownBackend(t *testing.T) {
	cfg := DBConfig{Type: "mysqlite"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want unsupported backend error")
	}
}

func TestConfigValidateRequiresNATSURL(t *testing.T) {
	// 内嵌 NATS 已剥离：NATS_URL 为空时必须在启动前就失败，
	// 而不是等到 kvstore.New 才报连接错误（那时日志里看不出是配置问题）。
	cfg := Config{DB: DBConfig{Type: "sqlite"}, KV: KVConfig{NATSURL: ""}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want NATS_URL required error")
	}
}

func TestConfigValidateAcceptsNATSURL(t *testing.T) {
	cfg := Config{DB: DBConfig{Type: "sqlite"}, KV: KVConfig{NATSURL: "nats://127.0.0.1:4222"}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}
