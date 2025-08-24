package scheduler_config

import (
	"time"

	pginfra "github.com/NordCoder/Pingerus/internal/repository/postgres"
)

type KafkaCfg struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
}

type SchedCfg struct {
	Tick        time.Duration `mapstructure:"tick"`
	BatchLimit  int           `mapstructure:"batch_limit"`
	MetricsAddr string        `mapstructure:"metrics_addr"`
}

type OTEL struct {
	Enable       bool    `mapstructure:"enable"`
	OTLPEndpoint string  `mapstructure:"otlp_endpoint"`
	ServiceName  string  `mapstructure:"service_name"`
	SampleRatio  float64 `mapstructure:"sample_ratio"`
}

type Config struct {
	DB       pginfra.Config `mapstructure:"db"`
	Kafka    KafkaCfg       `mapstructure:"kafka"`
	Sched    SchedCfg       `mapstructure:"sched"`
	LogLevel string         `mapstructure:"log_level"`
	OTEL     OTEL           `mapstructure:"otel"`
}
