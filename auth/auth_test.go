package auth

import (
	"checkYoutube/testing_utils"
	"context"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mocks
type oauth2Mock struct{}

func (o *oauth2Mock) generateVerifier() string {
	return "mockVerifier"
}
func (o *oauth2Mock) generateAuthURL(string, string, bool) string {
	return "mockURL"
}
func (o *oauth2Mock) exchangeCodeWithTokenSource(context.Context, string, ...oauth2.AuthCodeOption) (oauth2.TokenSource, error) {
	return &testing_utils.TokenSourceMock{}, nil
}
func (o *oauth2Mock) createHTTPClient(context.Context, *oauth2.Token) *http.Client {
	return &http.Client{}
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
	oauth2C := Oauth2Config{&oauth2Mock{}}
	sessionStore := sessions.NewCookieStore([]byte(("test")))

	type args struct {
		oauth2C      Oauth2Config
		sessionStore *sessions.CookieStore
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "success case",
			args: args{
				oauth2C:      oauth2C,
				sessionStore: sessionStore,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			handlerFunction := Login(tt.args.oauth2C, tt.args.sessionStore)
			handlerFunction(recorder, req)
			if recorder.Code != http.StatusTemporaryRedirect {
				t.Errorf("Login() = %v, want %v", recorder.Code, http.StatusTemporaryRedirect)
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
	oauth2C := Oauth2Config{&oauth2Mock{}}
	const errorCase = "error case - verifier not found"

	type args struct {
		serverBasepath string
		oauth2C        Oauth2Config
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
			},
			want: http.StatusSeeOther,
		},
		{
			name: errorCase,
			args: args{
				serverBasepath: "http://localhost:8900",
				oauth2C:        oauth2C,
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
			handlerFunction := Oauth2Redirect(oauth2C, tt.args.serverBasepath)
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
	oauth2C := Oauth2Config{&oauth2Mock{}}
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

func Test_getToken(t *testing.T) {
	// mocks
	oauth2C := Oauth2Config{&oauth2Mock{}}

	type args struct {
		oauth2C  Oauth2Config
		code     string
		verifier string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "success case",
			args:    args{oauth2C: oauth2C, code: "test", verifier: "verifier"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := getToken(tt.args.oauth2C, tt.args.code, tt.args.verifier); (err != nil) != tt.wantErr {
				t.Errorf("getToken() error = %v, wantErr %v", err, tt.wantErr)
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
			deleteOauth2SessionValue(t, sessionStore, req, verifierKey)
			if tt.name == successCase {
				setOauth2SessionValue(t, sessionStore, req, verifierKey, "verifier")
			}
			handlerFunction := CheckVerifierMiddleware(tt.args.next, tt.args.sessionStore, tt.args.serverBasepath)
			handlerFunction(recorder, req)
			if recorder.Code != tt.want {
				t.Errorf("CheckVerifierMiddleware() = %v, want %v", recorder.Code, tt.want)
			}
		})
	}
}

func Test_getValueFromSession(t *testing.T) {
	// mocks
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	const (
		sessionValue = "testvalue"
		successCase  = "success case"
	)
	sessionStore := sessions.NewCookieStore([]byte(("test")))

	type args struct {
		sessionStore *sessions.CookieStore
		r            *http.Request
		sessionName  string
		key          string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: successCase,
			args: args{
				sessionStore: sessionStore,
				r:            req,
				sessionName:  oauth2SessionName,
				key:          verifierKey,
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
				key:          verifierKey,
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteOauth2SessionValue(t, sessionStore, req, tt.args.key)
			if tt.name == successCase {
				// set a session value
				setOauth2SessionValue(t, sessionStore, req, tt.args.key, sessionValue)
			}
			got, err := getValueFromSession(tt.args.sessionStore, tt.args.r, tt.args.sessionName, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("getValueFromSession() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getValueFromSession() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func addVerifierToContext(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, verifierCtxKey{}, value)
}

func setOauth2SessionValue(t *testing.T, sessionStore *sessions.CookieStore, req *http.Request, key string, value string) {
	session, err := sessionStore.Get(req, oauth2SessionName)
	if err != nil {
		t.Fatal(err)
	}
	session.Values[key] = value
}

func deleteOauth2SessionValue(t *testing.T, sessionStore *sessions.CookieStore, req *http.Request, key string) {
	session, err := sessionStore.Get(req, oauth2SessionName)
	if err != nil {
		t.Fatal(err)
	}
	delete(session.Values, key)
}
