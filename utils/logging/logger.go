package logging

import (
	"context"
	"log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger zap.Logger
var cfg zap.Config

func init() {
	cfg = zap.Config{
		Encoding:         "json",
		Level:            zap.NewAtomicLevelAt(zapcore.Level(0)),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig: zapcore.EncoderConfig{
			MessageKey: "message",

			LevelKey:    "level",
			EncodeLevel: zapcore.CapitalLevelEncoder,

			TimeKey:    "time",
			EncodeTime: zapcore.ISO8601TimeEncoder,

			CallerKey:    "caller",
			EncodeCaller: zapcore.ShortCallerEncoder,
		},
	}

	aLogger, err := cfg.Build()

	if err != nil {
		log.Fatalf("FATAL ERROR: Failed to build zap logger: %s", err.Error())
	}

	logger = *aLogger
}

// Logger returns a zap logger with all available context
func Logger(ctx context.Context) zap.Logger {
	newLogger := logger
	newLogger = *newLogger.With(GetValuesSlice(ctx)...)
	return newLogger
}

// SetLevel func
func SetLevel(level int) {
	cfg.Level.SetLevel(zapcore.Level(level))
}
