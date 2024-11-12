package sessions

import (
	"checkYoutube/testing_utils"
	"encoding/gob"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	gob.Register(&oauth2.Token{})
	os.Exit(m.Run())
}

func TestGetValueFromSession(t *testing.T) {
	// mocks
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	const (
		sessionValue    = "testvalue"
		successCase     = "success case"
		invalidTypeCase = "error case - invalid session value type"
	)
	sessionStore := sessions.NewCookieStore([]byte(("test")))

	type args struct {
		sessionStore *sessions.CookieStore
		r            *http.Request
		sessionName  string
		key          string
	}
	type testCase[T any] struct {
		name    string
		args    args
		want    T
		wantErr bool
	}
	tests := []testCase[string]{
		{
			name: successCase,
			args: args{
				sessionStore: sessionStore,
				r:            req,
				sessionName:  Oauth2SessionName,
				key:          VerifierKey,
			},
			want:    sessionValue,
			wantErr: false,
		},
		{
			name: "error case - invalid session name",
			args: args{
				sessionStore: sessionStore,
				r:            req,
				sessionName:  "",
				key:          VerifierKey,
			},
			want:    "",
			wantErr: true,
		},
		{
			name: invalidTypeCase,
			args: args{
				sessionStore: sessionStore,
				r:            req,
				sessionName:  Oauth2SessionName,
				key:          VerifierKey,
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testing_utils.DeleteOauth2SessionValue(t, sessionStore, req, Oauth2SessionName, tt.args.key)
			if tt.name == successCase {
				// set a session value
				testing_utils.SetOauth2SessionValue(t, sessionStore, req, Oauth2SessionName, tt.args.key, sessionValue)
			}
			if tt.name == invalidTypeCase {
				// set a session value of the wrong type
				testing_utils.SetOauth2SessionValue(t, sessionStore, req, Oauth2SessionName, tt.args.key, 0)
			}
			got, err := GetValueFromSession[string](tt.args.sessionStore, tt.args.r, tt.args.sessionName, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetValueFromSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetValueFromSession() got = %v, want %v", got, tt.want)
			}
		})
	}
}
