package test

import (
	"context"
	"golang.org/x/oauth2"
	"net/http"
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
func (o *Oauth2Mock) GenerateAuthURL(string, string, bool, bool) string {
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
