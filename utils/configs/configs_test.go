package configs

import (
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
