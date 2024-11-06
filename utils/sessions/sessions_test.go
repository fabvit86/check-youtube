package sessions

import (
	"checkYoutube/testing_utils"
	"encoding/gob"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	gob.Register(&oauth2.Token{})
	os.Exit(m.Run())
}

func TestCheckTokenMiddleware(t *testing.T) {
	// mocks
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	sessionStore := sessions.NewCookieStore([]byte(("test")))

	type args struct {
		next           http.Handler
		sessionStore   *sessions.CookieStore
		serverBasepath string
		token          *oauth2.Token
		sessionName    string
		recorder       *httptest.ResponseRecorder
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "success case",
			args: args{
				next:           next,
				sessionStore:   sessionStore,
				serverBasepath: "http://localhost:8900",
				token: &oauth2.Token{
					AccessToken: "test",
				},
				sessionName: Oauth2SessionName,
				recorder:    httptest.NewRecorder(),
			},
			want: http.StatusOK,
		},
		{
			name: "error case - invalid token",
			args: args{
				next:           next,
				sessionStore:   sessionStore,
				serverBasepath: "http://localhost:8900",
				token:          &oauth2.Token{},
				sessionName:    Oauth2SessionName,
				recorder:       httptest.NewRecorder(),
			},
			want: http.StatusTemporaryRedirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testing_utils.SetOauth2SessionValue[*oauth2.Token](t, sessionStore, req, tt.args.sessionName,
				TokenKey, tt.args.token)
			handlerFunction := CheckTokenMiddleware(tt.args.next, tt.args.sessionStore, tt.args.serverBasepath)
			handlerFunction(tt.args.recorder, req)
			if tt.args.recorder.Code != tt.want {
				t.Errorf("CheckTokenMiddleware() = %v, want %v", tt.args.recorder.Code, tt.want)
			}
		})
	}
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
