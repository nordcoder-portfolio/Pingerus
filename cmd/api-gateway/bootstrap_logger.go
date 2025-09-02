package main

import (
	config "github.com/NordCoder/Pingerus/internal/config/api-gateway"
	"github.com/NordCoder/Pingerus/internal/obs"
	"go.uber.org/zap"
)

func initLogger(cfg *config.Config) (*zap.Logger, error) {
	return obs.NewLogger(obs.LogConfig{
		Level:  cfg.Log.Level,
		Pretty: cfg.Log.Pretty,
		App:    cfg.App.Name,
		Env:    cfg.App.Env,
		Ver:    cfg.App.Version,
	})
}
