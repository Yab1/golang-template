package logger

import (
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/Yab1/golang-template/internal/platform/config"
)

func New(cfg config.Log) (*zap.SugaredLogger, error) {
	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(strings.ToLower(cfg.Level))); err != nil {
		level = zapcore.InfoLevel
	}

	var zcfg zap.Config
	switch strings.ToLower(cfg.Format) {
	case "console":
		zcfg = zap.NewDevelopmentConfig()
	default:
		zcfg = zap.NewProductionConfig()
	}
	zcfg.Level = zap.NewAtomicLevelAt(level)

	l, err := zcfg.Build()
	if err != nil {
		return nil, err
	}
	return l.Sugar(), nil
}
