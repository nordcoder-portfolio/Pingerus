package ping_worker_config

import (
	"github.com/NordCoder/Pingerus/internal/obs"
	"github.com/NordCoder/Pingerus/internal/repository/kafka"
	"time"

	pginfra "github.com/NordCoder/Pingerus/internal/repository/postgres"
)

type KafkaIn struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
	GroupID string   `mapstructure:"group_id"`
}

func (kic *KafkaIn) AsConsumerConfig() *kafka.ConsumerConfig {
	return &kafka.ConsumerConfig{
		Brokers:       kic.Brokers,
		GroupID:       kic.GroupID,
		Topic:         kic.Topic,
		FromBeginning: true, // todo add in config
		Logger:        nil,
	}
}

type KafkaOut struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
}

type HTTPPing struct {
	Timeout         time.Duration `mapstructure:"timeout"`
	UserAgent       string        `mapstructure:"user_agent"`
	FollowRedirects bool          `mapstructure:"follow_redirects"`
	VerifyTLS       bool          `mapstructure:"verify_tls"`
}

type Server struct {
	MetricsAddr string `mapstructure:"metrics_addr"`
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
		App:    "pingerus/ping-worker",
		Env:    "",
		Ver:    "",
	}
}

type Config struct {
	DB     pginfra.Config `mapstructure:"db"`
	In     KafkaIn        `mapstructure:"kafka_in"`
	Out    KafkaOut       `mapstructure:"kafka_out"`
	HTTP   HTTPPing       `mapstructure:"http"`
	Server Server         `mapstructure:"server"`
	Log    Log            `mapstructure:"log"`
	OTEL   OTEL           `mapstructure:"otel"`
}
