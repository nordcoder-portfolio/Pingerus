package ping_worker_config

import (
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

	v.SetDefault("db.dsn", "postgres://postgres:secret@localhost:5432/pingerus?sslmode=disable")
	v.SetDefault("db.max_conns", 10)
	v.SetDefault("db.min_conns", 2)
	v.SetDefault("db.max_conn_lifetime", "30m")
	v.SetDefault("db.max_conn_idle_time", "10m")
	v.SetDefault("db.health_check_period", "30s")
	v.SetDefault("db.query_timeout", "2s")

	v.SetDefault("kafka_in.brokers", []string{"localhost:9094"})
	v.SetDefault("kafka_in.topic", "pingerus.checks.request")
	v.SetDefault("kafka_in.group_id", "ping-worker")

	v.SetDefault("kafka_out.brokers", []string{"localhost:9094"})
	v.SetDefault("kafka_out.topic", "pingerus.status.changed")

	v.SetDefault("http.timeout", "5s")
	v.SetDefault("http.user_agent", "Pingerus/1.0")
	v.SetDefault("http.follow_redirects", true)
	v.SetDefault("http.verify_tls", true)

	v.SetDefault("otel.enable", false)
	v.SetDefault("otel.service_name", "ping-worker")
	v.SetDefault("otel.sample_ratio", 1.0)
	v.SetDefault("otel.otlp_endpoint", "localhost:4317")

	v.SetDefault("server.metrics_addr", ":8083")
	v.SetDefault("log_level", "info")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
