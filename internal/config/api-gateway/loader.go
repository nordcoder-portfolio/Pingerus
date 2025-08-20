package api_gateway_config

import (
	"errors"
	"github.com/spf13/viper"
	"strings"
)

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	if path != "" {
		v.SetConfigFile(path)
		_ = v.ReadInConfig()
	}

	v.SetDefault("app.name", "api-gateway")
	v.SetDefault("app.env", "dev")
	v.SetDefault("server.http_addr", ":8080")
	v.SetDefault("server.grpc_addr", ":9090")
	v.SetDefault("server.read_timeout", "5s")
	v.SetDefault("server.write_timeout", "5s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.graceful_timeout", "15s")

	v.SetDefault("db.dsn", "postgres://postgres:secret@localhost:5432/pingerus?sslmode=disable")
	v.SetDefault("db.max_conns", 20)
	v.SetDefault("db.min_conns", 5)
	v.SetDefault("db.max_conn_lifetime", "30m")
	v.SetDefault("db.max_conn_idle_time", "10m")
	v.SetDefault("db.health_check_period", "30s")
	v.SetDefault("db.query_timeout", "2s")

	v.SetDefault("otel.enable", false)
	v.SetDefault("otel.service_name", "api-gateway")
	v.SetDefault("otel.sample_ratio", 1.0)
	v.SetDefault("otel.otlp_endpoint", "localhost:4317")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.pretty", false)

	v.SetDefault("auth.enable", true)
	v.SetDefault("auth.access_ttl", "15m")
	v.SetDefault("auth.refresh_ttl", "720h")
	v.SetDefault("auth.cookie_name", "refresh_token")
	v.SetDefault("auth.cookie_path", "/")
	v.SetDefault("auth.cookie_secure", false)

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if cfg.DB.DSN == "" {
		return nil, errors.New("no pg")
	}
	return &cfg, nil
}
