package scheduler_config

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

	v.SetDefault("kafka.brokers", []string{"localhost:9094"})
	v.SetDefault("kafka.topic", "pingerus.checks.request")

	v.SetDefault("sched.tick", "1s")
	v.SetDefault("sched.batch_limit", 100)
	v.SetDefault("sched.metrics_addr", ":8082")

	v.SetDefault("log_level", "info")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
