package mailer

import (
	"context"

	"go.uber.org/zap"
)

type Log struct {
	logger *zap.SugaredLogger
}

func NewLog(logger *zap.SugaredLogger) *Log {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &Log{logger: logger}
}

func (l *Log) Send(_ context.Context, msg Message) error {
	l.logger.Infow("mail",
		"to", msg.To,
		"subject", msg.Subject,
		"text", msg.Text,
	)
	return nil
}

func (l *Log) Driver() string {
	return "log"
}
