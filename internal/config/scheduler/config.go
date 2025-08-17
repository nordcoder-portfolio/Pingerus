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

type Config struct {
	DB       pginfra.Config `mapstructure:"db"`
	Kafka    KafkaCfg       `mapstructure:"kafka"`
	Sched    SchedCfg       `mapstructure:"sched"`
	LogLevel string         `mapstructure:"log_level"`
}
