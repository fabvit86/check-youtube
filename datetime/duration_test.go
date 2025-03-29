package datetime

import "testing"

func TestFormatISO8601Duration(t *testing.T) {
	type args struct {
		duration string
		username string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "hours, minutes, seconds",
			args: args{
				duration: "PT12H30M5S",
				username: "testuser",
			},
			want:    "12:30:05",
			wantErr: false,
		},
		{
			name: "hours, minutes",
			args: args{
				duration: "PT12H30M",
				username: "testuser",
			},
			want:    "12:30:00",
			wantErr: false,
		},
		{
			name: "hours, seconds",
			args: args{
				duration: "PT12H5S",
				username: "testuser",
			},
			want:    "12:00:05",
			wantErr: false,
		},
		{
			name: "minutes, seconds",
			args: args{
				duration: "PT30M5S",
				username: "testuser",
			},
			want:    "30:05",
			wantErr: false,
		},
		{
			name: "hours",
			args: args{
				duration: "PT8H",
				username: "testuser",
			},
			want:    "08:00:00",
			wantErr: false,
		},
		{
			name: "minutes",
			args: args{
				duration: "PT7M",
				username: "testuser",
			},
			want:    "07:00",
			wantErr: false,
		},
		{
			name: "seconds",
			args: args{
				duration: "PT5S",
				username: "testuser",
			},
			want:    "00:05",
			wantErr: false,
		},
		{
			name: "error case - invalid duration format",
			args: args{
				duration: "INVALID",
				username: "testuser",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatISO8601Duration(tt.args.duration, tt.args.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("FormatISO8601Duration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FormatISO8601Duration() got = %v, want %v", got, tt.want)
			}
		})
	}
}
