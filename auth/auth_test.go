package auth

import (
	"checkYoutube/testing_utils"
	"context"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestMain(m *testing.M) {
	sessionStore = sessions.NewCookieStore([]byte(("test")))
	os.Exit(m.Run())
}

func TestInitOauth2Config(t *testing.T) {
	const (
		successInitNew      = "success case - init new"
		successKeepExisting = "success case - keep existing instance"
	)
	type args struct {
		clientID     string
		clientSecret string
		redirectURL  string
	}
	testArgs := args{
		clientID:     "test",
		clientSecret: "test",
		redirectURL:  "test",
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: successInitNew,
			args: testArgs,
		},
		{
			name: successKeepExisting,
			args: testArgs,
		},
	}
	for _, tt := range tests {
		InitOauth2Config(tt.args.clientID, tt.args.clientSecret, tt.args.redirectURL)
		t.Run(tt.name, func(t *testing.T) {
			if oauth2C == nil {
				t.Errorf("InitOauth2Config() - oauth2C is nil, want not nil")
			}
			if oauth2C.Oauth2ConfigProvider == nil {
				t.Errorf("InitOauth2Config() - oauth2C.provider is nil, want not nil")
			}
			pointerCopy := oauth2C
			if tt.name == successKeepExisting {
				InitOauth2Config(tt.args.clientID, tt.args.clientSecret, tt.args.redirectURL)
				if oauth2C != pointerCopy {
					t.Errorf("InitOauth2Config() - oauth2C = %v, copy = %v, want same memory address",
						oauth2C, pointerCopy)
				}
			}
		})
	}
}

func TestLogin(t *testing.T) {
	// mocks
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	oauth2C = &oauth2Config{
		&oauth2Mock{},
	}

	type args struct {
		w http.ResponseWriter
		r *http.Request
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "success case",
			args: args{
				w: recorder,
				r: req,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			Login(tt.args.w, tt.args.r)
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
	oauth2C = &oauth2Config{
		&oauth2Mock{},
	}
	const errorCase = "error case - verifier not found"

	type args struct {
		serverBasepath string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "success case",
			args: args{serverBasepath: "http://localhost:8900"},
			want: http.StatusSeeOther,
		},
		{
			name: errorCase,
			args: args{serverBasepath: "http://localhost:8900"},
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
			handlerFunction := Oauth2Redirect(tt.args.serverBasepath)
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
	oauth2C = &oauth2Config{
		&oauth2Mock{},
	}
	const errorCase = "error case - verifier not found"

	tests := []struct {
		name string
		want int
	}{
		{
			name: "success case",
			want: http.StatusTemporaryRedirect,
		},
		{
			name: errorCase,
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
			handlerFunction := SwitchAccount()
			handlerFunction(recorder, req)
			if recorder.Code != tt.want {
				t.Errorf("SwitchAccount() = %v, want %v", recorder.Code, tt.want)
			}
		})
	}
}

func Test_getToken(t *testing.T) {
	// mocks
	oauth2C = &oauth2Config{
		&oauth2Mock{},
	}

	type args struct {
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
			args:    args{code: "test", verifier: "verifier"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := getToken(tt.args.code, tt.args.verifier); (err != nil) != tt.wantErr {
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
	const successCase = "success case"

	type args struct {
		next           http.Handler
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
				serverBasepath: "http://localhost:8900",
			},
			want: http.StatusOK,
		},
		{
			name: "redirect case - verifier not found",
			args: args{
				next:           next,
				serverBasepath: "http://localhost:8900",
			},
			want: http.StatusTemporaryRedirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteOauth2SessionValue(t, req, verifierKey)
			if tt.name == successCase {
				setOauth2SessionValue(t, req, verifierKey, "verifier")
			}
			handlerFunction := CheckVerifierMiddleware(tt.args.next, tt.args.serverBasepath)
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

	type args struct {
		r           *http.Request
		sessionName string
		key         string
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
				r:           req,
				sessionName: oauth2SessionName,
				key:         verifierKey,
			},
			want:    sessionValue,
			wantErr: false,
		},
		{
			name: "error case - invalid session name",
			args: args{
				r:           req,
				sessionName: "",
				key:         verifierKey,
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteOauth2SessionValue(t, req, tt.args.key)
			if tt.name == successCase {
				// set a session value
				setOauth2SessionValue(t, req, tt.args.key, sessionValue)
			}
			got, err := getValueFromSession(tt.args.r, tt.args.sessionName, tt.args.key)
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

func setOauth2SessionValue(t *testing.T, req *http.Request, key string, value string) {
	session, err := sessionStore.Get(req, oauth2SessionName)
	if err != nil {
		t.Fatal(err)
	}
	session.Values[key] = value
}

func deleteOauth2SessionValue(t *testing.T, req *http.Request, key string) {
	session, err := sessionStore.Get(req, oauth2SessionName)
	if err != nil {
		t.Fatal(err)
	}
	delete(session.Values, key)
}
