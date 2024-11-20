package auth

import (
	"checkYoutube/clients"
	sessionsutils "checkYoutube/sessions"
	"checkYoutube/test"
	"context"
	"encoding/gob"
	"fmt"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

type peopleClientMock struct {
	getLoggedUserinfoStub func() string
}
type peopleClientFactoryMock struct {
	newClientStub func(oauth2.TokenSource) (clients.PeopleClientInterface, error)
}

func (p peopleClientMock) GetLoggedUserinfo() string {
	return p.getLoggedUserinfoStub()
}
func (pf *peopleClientFactoryMock) NewClient(ts oauth2.TokenSource) (clients.PeopleClientInterface, error) {
	return pf.newClientStub(ts)
}

func TestMain(m *testing.M) {
	gob.Register(&TokenInfo{})
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
	oauth2C := Oauth2Config{&test.Oauth2Mock{}}
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
	oauth2C := Oauth2Config{&test.Oauth2Mock{}}
	sessionStore := sessions.NewCookieStore([]byte(("test")))
	const errorCase = "error case - verifier not found"
	pcf := &peopleClientFactoryMock{
		newClientStub: func(ts oauth2.TokenSource) (clients.PeopleClientInterface, error) {
			return &peopleClientMock{getLoggedUserinfoStub: func() string { return "usertest" }}, nil
		},
	}

	type args struct {
		oauth2C        Oauth2Config
		sessionStore   *sessions.CookieStore
		pcf            clients.PeopleClientFactoryInterface
		serverBasepath string
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
				pcf:            pcf,
			},
			want: http.StatusSeeOther,
		},
		{
			name: "error case - error on creating people client",
			args: args{
				serverBasepath: "http://localhost:8900",
				oauth2C:        oauth2C,
				sessionStore:   sessionStore,
				pcf: &peopleClientFactoryMock{
					newClientStub: func(ts oauth2.TokenSource) (clients.PeopleClientInterface, error) {
						return nil, fmt.Errorf("testerror")
					},
				},
			},
			want: http.StatusInternalServerError,
		},
		{
			name: errorCase,
			args: args{
				serverBasepath: "http://localhost:8900",
				oauth2C:        oauth2C,
				sessionStore:   sessionStore,
				pcf:            pcf,
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
			handlerFunction := Oauth2Redirect(oauth2C, tt.args.sessionStore, tt.args.pcf, tt.args.serverBasepath)
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
	oauth2C := Oauth2Config{&test.Oauth2Mock{}}
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
			test.DeleteOauth2SessionValue(t, tt.args.sessionStore, req,
				sessionsutils.Oauth2SessionName, sessionsutils.VerifierKey)
			if tt.name == successCase {
				test.SetOauth2SessionValue(t, tt.args.sessionStore, req,
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

func TestCheckTokenMiddleware(t *testing.T) {
	// mocks
	const wrongSessionValueCase = "error case - wrong session value type"
	req, err := http.NewRequest(http.MethodGet, "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	sessionStore := sessions.NewCookieStore([]byte(("test")))
	oauth2C := Oauth2Config{&test.Oauth2Mock{}}

	type args struct {
		next           http.Handler
		oauth2C        Oauth2Config
		sessionStore   *sessions.CookieStore
		serverBasepath string
		tokenInfo      *TokenInfo
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
				oauth2C:        oauth2C,
				sessionStore:   sessionStore,
				serverBasepath: "http://localhost:8900",
				tokenInfo: &TokenInfo{
					Token: &oauth2.Token{
						AccessToken: "test",
					}},
				sessionName: sessionsutils.Oauth2SessionName,
				recorder:    httptest.NewRecorder(),
			},
			want: http.StatusOK,
		},
		{
			name: "success case - refresh token",
			args: args{
				next:           next,
				oauth2C:        oauth2C,
				sessionStore:   sessionStore,
				serverBasepath: "http://localhost:8900",
				tokenInfo: &TokenInfo{
					Token: &oauth2.Token{
						AccessToken:  "test",
						RefreshToken: "refreshtest",
						Expiry:       time.Now().Add(time.Hour * -24),
					}},
				sessionName: sessionsutils.Oauth2SessionName,
				recorder:    httptest.NewRecorder(),
			},
			want: http.StatusOK,
		},
		{
			name: "error case - invalid token",
			args: args{
				next:           next,
				oauth2C:        oauth2C,
				sessionStore:   sessionStore,
				serverBasepath: "http://localhost:8900",
				tokenInfo:      &TokenInfo{},
				sessionName:    sessionsutils.Oauth2SessionName,
				recorder:       httptest.NewRecorder(),
			},
			want: http.StatusTemporaryRedirect,
		},
		{
			name: wrongSessionValueCase,
			args: args{
				next:           next,
				oauth2C:        oauth2C,
				sessionStore:   sessionStore,
				serverBasepath: "http://localhost:8900",
				tokenInfo: &TokenInfo{
					Token: &oauth2.Token{
						AccessToken:  "test",
						RefreshToken: "refreshtest",
						Expiry:       time.Now().Add(time.Hour * -24),
					}},
				sessionName: sessionsutils.Oauth2SessionName,
				recorder:    httptest.NewRecorder(),
			},
			want: http.StatusTemporaryRedirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == wrongSessionValueCase {
				test.SetOauth2SessionValue[int](t, sessionStore, req, tt.args.sessionName,
					sessionsutils.TokenKey, 0)
			} else {
				test.SetOauth2SessionValue[*TokenInfo](t, sessionStore, req, tt.args.sessionName,
					sessionsutils.TokenKey, tt.args.tokenInfo)
			}
			handlerFunction := CheckTokenMiddleware(tt.args.next, tt.args.oauth2C, tt.args.sessionStore,
				tt.args.serverBasepath)
			handlerFunction(tt.args.recorder, req)
			if tt.args.recorder.Code != tt.want {
				t.Errorf("CheckTokenMiddleware() = %v, want %v", tt.args.recorder.Code, tt.want)
			}
		})
	}
}

func addVerifierToContext(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, verifierCtxKey{}, value)
}
