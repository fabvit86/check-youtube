package logging

import (
	"context"
	"log/slog"
	"testing"
)

func TestConfigureLogger(t *testing.T) {
	type args struct {
		logLevel string
	}
	tests := []struct {
		name string
		args args
		want slog.Level
	}{
		{
			name: "debug enabled",
			args: args{
				logLevel: "debug",
			},
			want: slog.LevelDebug,
		},
		{
			name: "info enabled",
			args: args{
				logLevel: "",
			},
			want: slog.LevelInfo,
		},
		{
			name: "warn enabled",
			args: args{
				logLevel: "warn",
			},
			want: slog.LevelWarn,
		},
		{
			name: "error enabled",
			args: args{
				logLevel: "error",
			},
			want: slog.LevelError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ConfigureLogger(tt.args.logLevel)
			slog.Info("test log")
			if !slog.Default().Enabled(context.Background(), tt.want) {
				t.Errorf("TestConfigureLogger() = %s log level not enabled", tt.want.String())
			}
		})
	}
}
