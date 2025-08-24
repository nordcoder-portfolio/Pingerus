package email_notifier_config

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
	v.SetDefault("db.max_conns", 20)
	v.SetDefault("db.min_conns", 5)
	v.SetDefault("db.max_conn_lifetime", "30m")
	v.SetDefault("db.max_conn_idle_time", "10m")
	v.SetDefault("db.health_check_period", "30s")
	v.SetDefault("db.query_timeout", "2s")

	v.SetDefault("kafka_in.brokers", []string{"kafka:9092"})
	v.SetDefault("kafka_in.topic", "pingerus.status.change")
	v.SetDefault("kafka_in.group_id", "email-notifier")

	v.SetDefault("smtp.addr", "localhost:1025")
	v.SetDefault("smtp.from", "noreply@pingerus.dev")
	v.SetDefault("smtp.use_tls", false)
	v.SetDefault("smtp.timeout", "5s")
	v.SetDefault("smtp.subj_prefix", "[Pingerus]")

	v.SetDefault("otel.enable", false)
	v.SetDefault("otel.service_name", "email-notifier")
	v.SetDefault("otel.sample_ratio", 1.0)
	v.SetDefault("otel.otlp_endpoint", "localhost:4317")

	v.SetDefault("server.metrics_addr", ":8084")
	v.SetDefault("log_level", "info")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
