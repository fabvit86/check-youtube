package testing_utils

import (
	"context"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"net/http"
	"testing"
)

// TokenSourceMock mocks an oauth2 token source implementation
type TokenSourceMock struct{}

func (t *TokenSourceMock) Token() (*oauth2.Token, error) {
	return &oauth2.Token{}, nil
}

// Oauth2Mock mocks an oauth2 config implementation
type Oauth2Mock struct{}

func (o *Oauth2Mock) GenerateVerifier() string {
	return "mockVerifier"
}
func (o *Oauth2Mock) GenerateAuthURL(string, string, bool) string {
	return "mockURL"
}
func (o *Oauth2Mock) ExchangeCodeWithToken(context.Context, string, ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return &oauth2.Token{}, nil
}
func (o *Oauth2Mock) CreateHTTPClient(context.Context, *oauth2.Token) *http.Client {
	return &http.Client{}
}
func (o *Oauth2Mock) CreateTokenSource(context.Context, *oauth2.Token) oauth2.TokenSource {
	return &TokenSourceMock{}
}

func SetOauth2SessionValue[T any](t *testing.T, sessionStore *sessions.CookieStore,
	req *http.Request, sessionName, key string, value T) {
	session, err := sessionStore.Get(req, sessionName)
	if err != nil {
		t.Fatal(err)
	}
	session.Values[key] = value
}

func DeleteOauth2SessionValue(t *testing.T, sessionStore *sessions.CookieStore,
	req *http.Request, sessionName, key string) {
	session, err := sessionStore.Get(req, sessionName)
	if err != nil {
		t.Fatal(err)
	}
	delete(session.Values, key)
}
