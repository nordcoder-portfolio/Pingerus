package ping_worker_config

import (
	"time"

	pginfra "github.com/NordCoder/Pingerus/internal/repository/postgres"
)

type KafkaIn struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
	GroupID string   `mapstructure:"group_id"`
}

type KafkaOut struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
}

type HTTPPing struct {
	Timeout         time.Duration `mapstructure:"timeout"`
	UserAgent       string        `mapstructure:"user_agent"`
	MaxRedirects    int           `mapstructure:"max_redirects"`
	FollowRedirects bool          `mapstructure:"follow_redirects"`
	VerifyTLS       bool          `mapstructure:"verify_tls"`
}

type Server struct {
	MetricsAddr string `mapstructure:"metrics_addr"`
}

type Config struct {
	DB       pginfra.Config `mapstructure:"db"`
	In       KafkaIn        `mapstructure:"kafka_in"`
	Out      KafkaOut       `mapstructure:"kafka_out"`
	HTTP     HTTPPing       `mapstructure:"http"`
	Server   Server         `mapstructure:"server"`
	LogLevel string         `mapstructure:"log_level"`
}
