package email_notifier_config

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
		App:    "pingerus/email-notifier",
		Env:    "",
		Ver:    "",
	}
}

type Config struct {
	DB     pginfra.Config `mapstructure:"db"`
	In     KafkaIn        `mapstructure:"kafka_in"`
	SMTP   SMTP           `mapstructure:"smtp"`
	Server Server         `mapstructure:"server"`
	Log    Log            `mapstructure:"log"`
	OTEL   OTEL           `mapstructure:"otel"`
}
