package email_notifier_config

import (
	"time"

	pginfra "github.com/NordCoder/Pingerus/internal/repository/postgres"
)

type KafkaIn struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
	GroupID string   `mapstructure:"group_id"`
}

type SMTP struct {
	Addr       string        `mapstructure:"addr"`
	From       string        `mapstructure:"from"`
	User       string        `mapstructure:"user"`
	Password   string        `mapstructure:"password"`
	UseTLS     bool          `mapstructure:"use_tls"`
	Timeout    time.Duration `mapstructure:"timeout"`
	SubjPrefix string        `mapstructure:"subj_prefix"`
}

type Server struct {
	MetricsAddr string `mapstructure:"metrics_addr"`
}

type Config struct {
	DB       pginfra.Config `mapstructure:"db"`
	In       KafkaIn        `mapstructure:"kafka_in"`
	SMTP     SMTP           `mapstructure:"smtp"`
	Server   Server         `mapstructure:"server"`
	LogLevel string         `mapstructure:"log_level"`
}
