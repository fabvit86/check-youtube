package utils

import (
	"context"
	"log/slog"
	"testing"
)

func TestGetEnvOrErr(t *testing.T) {
	t.Setenv("TEST_VAR", "testvalue")

	type args struct {
		varName string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "success case",
			args: args{
				varName: "TEST_VAR",
			},
			want:    "testvalue",
			wantErr: false,
		},
		{
			name: "error case - var found found",
			args: args{
				varName: "MISSING_VAR",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetEnvOrErr(tt.args.varName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetEnvOrErr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetEnvOrErr() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetEnvOrFallback(t *testing.T) {
	t.Setenv("TEST_VAR", "testvalue")

	type args struct {
		varName  string
		fallback string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "success case",
			args: args{
				varName:  "TEST_VAR",
				fallback: "fallbackvalue",
			},
			want: "testvalue",
		},
		{
			name: "fallback case",
			args: args{
				varName:  "MISSING_VAR",
				fallback: "fallbackvalue",
			},
			want: "fallbackvalue",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetEnvOrFallback(tt.args.varName, tt.args.fallback); got != tt.want {
				t.Errorf("GetEnvOrFallback() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
