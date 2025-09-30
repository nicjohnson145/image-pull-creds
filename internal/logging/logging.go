package logging

import (
	"context"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
)

//go:generate go-enum -f $GOFILE -marshal -names

/*
ENUM(
warn
info
debug
trace
)
*/
type LogLevel string

/*
ENUM(
human
json
)
*/
type LogFormat string

type LoggingConfig struct {
	Level  LogLevel
	Format LogFormat
}

func Init(cfg *LoggingConfig) zerolog.Logger {
	base := zerolog.New(os.Stdout)
	switch cfg.Format {
	case LogFormatHuman:
		base = base.Output(zerolog.ConsoleWriter{Out: os.Stdout})
	}

	logger := base.With().Timestamp().Logger()

	switch cfg.Level {
	case LogLevelWarn:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case LogLevelInfo:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case LogLevelDebug:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case LogLevelTrace:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	return logger
}

func GetContextLogger(ctx context.Context) zerolog.Logger {
	sink := logr.FromContextOrDiscard(ctx).GetSink()
	if sink == nil {
		return zerolog.Nop()
	}

	return *(sink.(*zerologr.LogSink).GetUnderlying())
}
