package testing_utils

import "golang.org/x/oauth2"

// TokenSourceMock mocks an oauth2 token source implementation
type TokenSourceMock struct{}

func (t *TokenSourceMock) Token() (*oauth2.Token, error) {
	return &oauth2.Token{}, nil
}
