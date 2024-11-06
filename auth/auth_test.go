package auth

import (
	"checkYoutube/testing_utils"
	sessionsutils "checkYoutube/utils/sessions"
	"context"
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

func TestCreateOauth2Config(t *testing.T) {
	type args struct {
		clientID     string
		clientSecret string
		redirectURL  string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "success case",
			args: args{
				clientID:     "clientIDTest",
				clientSecret: "clientSecretTest",
				redirectURL:  "redirectURLTest",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CreateOauth2Config(tt.args.clientID, tt.args.clientSecret, tt.args.redirectURL)
			if got.Oauth2ConfigProvider == nil {
				t.Errorf("CreateOauth2Config() - config is nil")
			}
		})
	}
}

func TestLogin(t *testing.T) {
	// mocks
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	oauth2C := Oauth2Config{&testing_utils.Oauth2Mock{}}
	sessionStore := sessions.NewCookieStore([]byte(("test")))

	type args struct {
		oauth2C      Oauth2Config
		sessionStore *sessions.CookieStore
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "success case",
			args: args{
				oauth2C:      oauth2C,
				sessionStore: sessionStore,
			},
			want: http.StatusTemporaryRedirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handlerFunction := Login(tt.args.oauth2C, tt.args.sessionStore)
			handlerFunction(recorder, req)
			if recorder.Code != tt.want {
				t.Errorf("Login() = %v, want %v", recorder.Code, tt.want)
			}
		})
	}
}

func TestOauth2Redirect(t *testing.T) {
	// mocks
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	oauth2C := Oauth2Config{&testing_utils.Oauth2Mock{}}
	sessionStore := sessions.NewCookieStore([]byte(("test")))
	const errorCase = "error case - verifier not found"

	type args struct {
		serverBasepath string
		oauth2C        Oauth2Config
		sessionStore   *sessions.CookieStore
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "success case",
			args: args{
				serverBasepath: "http://localhost:8900",
				oauth2C:        oauth2C,
				sessionStore:   sessionStore,
			},
			want: http.StatusSeeOther,
		},
		{
			name: errorCase,
			args: args{
				serverBasepath: "http://localhost:8900",
				oauth2C:        oauth2C,
				sessionStore:   sessionStore,
			},
			want: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == errorCase {
				req = req.WithContext(context.Background())
			} else {
				req = req.WithContext(addVerifierToContext(req.Context(), "verifier"))
			}
			recorder := httptest.NewRecorder()
			handlerFunction := Oauth2Redirect(oauth2C, tt.args.sessionStore, tt.args.serverBasepath)
			handlerFunction(recorder, req)
			if recorder.Code != tt.want {
				t.Errorf("Oauth2Redirect() = %v, want %v", recorder.Code, tt.want)
			}
		})
	}
}

func TestSwitchAccount(t *testing.T) { // mocks
	// mocks
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	oauth2C := Oauth2Config{&testing_utils.Oauth2Mock{}}
	const errorCase = "error case - verifier not found"

	type args struct {
		oauth2C Oauth2Config
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "success case",
			args: args{oauth2C: oauth2C},
			want: http.StatusTemporaryRedirect,
		},
		{
			name: errorCase,
			args: args{oauth2C: oauth2C},
			want: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == errorCase {
				req = req.WithContext(context.Background())
			} else {
				req = req.WithContext(addVerifierToContext(req.Context(), "verifier"))
			}
			recorder := httptest.NewRecorder()
			handlerFunction := SwitchAccount(tt.args.oauth2C)
			handlerFunction(recorder, req)
			if recorder.Code != tt.want {
				t.Errorf("SwitchAccount() = %v, want %v", recorder.Code, tt.want)
			}
		})
	}
}

func Test_getAndStoreToken(t *testing.T) {
	// mocks
	oauth2C := Oauth2Config{&testing_utils.Oauth2Mock{}}
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	sessionStore := sessions.NewCookieStore([]byte(("test")))
	session, err := sessionStore.Get(req, sessionsutils.Oauth2SessionName)
	if err != nil {
		t.Fatal(err)
	}

	type args struct {
		oauth2C  Oauth2Config
		code     string
		verifier string
		session  *sessions.Session
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "success case",
			args:    args{oauth2C: oauth2C, session: session, code: "test", verifier: "verifier"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := getAndStoreToken(tt.args.oauth2C, tt.args.session, tt.args.code,
				tt.args.verifier); (err != nil) != tt.wantErr {
				t.Errorf("getAndStoreToken() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckVerifierMiddleware(t *testing.T) {
	// mocks
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	sessionStore := sessions.NewCookieStore([]byte(("test")))
	const successCase = "success case"

	type args struct {
		next           http.Handler
		sessionStore   *sessions.CookieStore
		serverBasepath string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: successCase,
			args: args{
				next:           next,
				sessionStore:   sessionStore,
				serverBasepath: "http://localhost:8900",
			},
			want: http.StatusOK,
		},
		{
			name: "redirect case - verifier not found",
			args: args{
				next:           next,
				sessionStore:   sessionStore,
				serverBasepath: "http://localhost:8900",
			},
			want: http.StatusTemporaryRedirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testing_utils.DeleteOauth2SessionValue(t, tt.args.sessionStore, req,
				sessionsutils.Oauth2SessionName, sessionsutils.VerifierKey)
			if tt.name == successCase {
				testing_utils.SetOauth2SessionValue(t, tt.args.sessionStore, req,
					sessionsutils.Oauth2SessionName, sessionsutils.VerifierKey, "verifier")
			}
			handlerFunction := CheckVerifierMiddleware(tt.args.next, tt.args.sessionStore, tt.args.serverBasepath)
			handlerFunction(recorder, req)
			if recorder.Code != tt.want {
				t.Errorf("CheckVerifierMiddleware() = %v, want %v", recorder.Code, tt.want)
			}
		})
	}
}

func addVerifierToContext(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, verifierCtxKey{}, value)
}
