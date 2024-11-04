package auth

import (
	"checkYoutube/logging"
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"log/slog"
	"net/http"
)

type Oauth2ConfigProvider interface {
	generateVerifier() string
	generateAuthURL(state, verifier string, promptAccountSelect bool) string
	exchangeCodeWithTokenSource(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (oauth2.TokenSource, error)
	createHTTPClient(ctx context.Context, token *oauth2.Token) *http.Client
}

type oauth2ConfigInstance struct {
	oauth2Config oauth2.Config
}

func (o *oauth2ConfigInstance) generateVerifier() string {
	return oauth2.GenerateVerifier()
}

func (o *oauth2ConfigInstance) generateAuthURL(state, verifier string, promptAccountSelect bool) string {
	opts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier),
	}
	if promptAccountSelect {
		opts = append(opts, oauth2.SetAuthURLParam("prompt", "select_account"))
	}

	return o.oauth2Config.AuthCodeURL(state, opts...)
}

func (o *oauth2ConfigInstance) exchangeCodeWithTokenSource(ctx context.Context, code string,
	opts ...oauth2.AuthCodeOption) (oauth2.TokenSource, error) {
	const funcName = "exchangeCodeWithTokenSource"

	token, err := o.oauth2Config.Exchange(ctx, code, opts...)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to retrieve auth token, error: %s", err.Error()),
			logging.FuncNameAttr(funcName))
		return nil, err
	}

	return o.oauth2Config.TokenSource(ctx, token), nil
}

func (o *oauth2ConfigInstance) createHTTPClient(ctx context.Context, token *oauth2.Token) *http.Client {
	return o.oauth2Config.Client(ctx, token)
}
