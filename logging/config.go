package logging

import (
	"log/slog"
	"os"
	"path"
	"strings"
)

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

// FuncNameAttr returns a slog.Attr to add the function name to the log
func FuncNameAttr(funcName string) slog.Attr {
	return slog.String("function", funcName)
}

// UserAttr returns a slog.Attr to add the logged user's username to the log
func UserAttr(username string) slog.Attr {
	return slog.String("user", username)
}
