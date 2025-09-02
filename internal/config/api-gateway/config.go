package api_gateway_config

import (
	"github.com/NordCoder/Pingerus/internal/obs"
	"time"

	pg "github.com/NordCoder/Pingerus/internal/repository/postgres"
)

type App struct {
	Name    string `mapstructure:"name"`
	Env     string `mapstructure:"env"`
	Version string `mapstructure:"version"`
}

type Server struct {
	HTTPAddr        string        `mapstructure:"http_addr"`
	GRPCAddr        string        `mapstructure:"grpc_addr"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	GracefulTimeout time.Duration `mapstructure:"graceful_timeout"`
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

type Log struct {
	Level  string `mapstructure:"level"`
	Pretty bool   `mapstructure:"pretty"`
}

func (lc *Log) AsLoggerConfig() *obs.LogConfig {
	return &obs.LogConfig{
		Level:  lc.Level,
		Pretty: lc.Pretty,
		App:    "pingerus/api-gateway",
		Env:    "",
		Ver:    "",
	}
}

type Auth struct {
	Enable       bool          `mapstructure:"enable"`
	JWTSecret    string        `mapstructure:"jwt_secret"`
	AccessTTL    time.Duration `mapstructure:"access_ttl"`
	RefreshTTL   time.Duration `mapstructure:"refresh_ttl"`
	CookieName   string        `mapstructure:"cookie_name"`
	CookieDomain string        `mapstructure:"cookie_domain"`
	CookiePath   string        `mapstructure:"cookie_path"`
	CookieSecure bool          `mapstructure:"cookie_secure"`
}

type Config struct {
	App    App       `mapstructure:"app"`
	Server Server    `mapstructure:"server"`
	DB     pg.Config `mapstructure:"db"`
	OTEL   OTEL      `mapstructure:"otel"`
	Log    Log       `mapstructure:"log"`
	Auth   Auth      `mapstructure:"auth"`
}

type ErrConfig string

func (e ErrConfig) Error() string { return string(e) }
