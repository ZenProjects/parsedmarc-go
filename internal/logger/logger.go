package logger

import (
	"go.uber.org/zap"
	"parsedmarc-go/internal/config"
)

// New creates a new zap logger based on configuration
func New(cfg config.LoggingConfig) (*zap.Logger, error) {
	var zapConfig zap.Config

	switch cfg.Level {
	case "debug":
		zapConfig = zap.NewDevelopmentConfig()
	default:
		zapConfig = zap.NewProductionConfig()
	}

	// Set log level
	level, err := zap.ParseAtomicLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	zapConfig.Level = level

	// Set encoding format
	switch cfg.Format {
	case "console":
		zapConfig.Encoding = "console"
	default:
		zapConfig.Encoding = "json"
	}

	// Set output paths
	if cfg.OutputPath == "stdout" || cfg.OutputPath == "" {
		zapConfig.OutputPaths = []string{"stdout"}
	} else {
		zapConfig.OutputPaths = []string{cfg.OutputPath}
	}

	// Error output
	zapConfig.ErrorOutputPaths = []string{"stderr"}

	return zapConfig.Build()
}

// NewDefault creates a default logger for cases where config is not available
func NewDefault() *zap.Logger {
	logger, err := zap.NewProduction()
	if err != nil {
		// Fallback to a basic logger if production logger fails
		logger = zap.NewNop()
	}
	return logger
}
