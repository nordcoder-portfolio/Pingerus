package obs

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type LogConfig struct {
	Level  string
	Pretty bool
	App    string
	Env    string
	Ver    string
}

func NewLogger(c LogConfig) (*zap.Logger, error) {
	var cfg zap.Config
	if c.Pretty {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}
	level := new(zapcore.Level)
	if err := level.Set(c.Level); err != nil {
		*level = zapcore.InfoLevel
	}
	cfg.Level = zap.NewAtomicLevelAt(*level)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	l, err := cfg.Build(
		zap.Fields(
			zap.String("service", c.App),
			zap.String("env", c.Env),
			zap.String("version", c.Ver),
		),
	)
	if err != nil {
		return nil, err
	}
	return l, nil
}
