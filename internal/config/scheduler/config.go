package scheduler_config

import (
	"github.com/NordCoder/Pingerus/internal/obs"
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

func (oc *OTEL) AsOTELConfig() *obs.OTELConfig {
	return &obs.OTELConfig{
		Enable:      oc.Enable,
		Endpoint:    oc.OTLPEndpoint,
		ServiceName: oc.ServiceName,
		SampleRatio: oc.SampleRatio,
	}
}

type Log struct { // todo add to config
	Level  string `mapstructure:"level"`
	Pretty bool   `mapstructure:"pretty"`
}

func (lc *Log) AsLoggerConfig() *obs.LogConfig {
	return &obs.LogConfig{
		Level:  lc.Level,
		Pretty: lc.Pretty,
		App:    "pingerus/scheduler",
		Env:    "",
		Ver:    "",
	}
}

type Config struct {
	DB    pginfra.Config `mapstructure:"db"`
	Kafka KafkaCfg       `mapstructure:"kafka"`
	Sched SchedCfg       `mapstructure:"sched"`
	Log   Log            `mapstructure:"log"`
	OTEL  OTEL           `mapstructure:"otel"`
}
