package utils

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
)

// GetEnvOrFallback returns the env variable value or the fallback value if the env var is not set
func GetEnvOrFallback(varName, fallback string) string {
	if varValue, ok := os.LookupEnv(varName); ok {
		return varValue
	}

	slog.Warn(fmt.Sprintf("env variable %s not set, fallback to %s", varName, fallback))
	return fallback
}

// GetEnvOrErr returns the env variable value or an error if the env var is not set
func GetEnvOrErr(varName string) (string, error) {
	if varValue, ok := os.LookupEnv(varName); ok && varValue != "" {
		return varValue, nil
	}

	err := fmt.Errorf("required env variable %s not set or empty", varName)
	slog.Error(err.Error())
	return "", err
}

// ConfigureLogger configure and set the slog logger. logLevel is case insensitive
func ConfigureLogger(logLevel string) {
	loggerOpts := slog.HandlerOptions{
		AddSource: true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// log only the file name
			if a.Key == slog.SourceKey {
				s := a.Value.Any().(*slog.Source)
				s.File = path.Base(s.File)
			}
			return a
		},
	}

	// set log level
	switch strings.ToLower(logLevel) {
	case "error":
		loggerOpts.Level = slog.LevelError
	case "warn", "warning":
		loggerOpts.Level = slog.LevelWarn
	case "debug":
		loggerOpts.Level = slog.LevelDebug
	default:
		loggerOpts.Level = slog.LevelInfo
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &loggerOpts)))
}
