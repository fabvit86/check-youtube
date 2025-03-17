package auth

import (
	"checkYoutube/logging"
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"log/slog"
	"net/http"
	"strings"
)

type Oauth2ConfigProvider interface {
	GenerateVerifier() string
	GenerateAuthURL(state, verifier string, promptAccountSelect, promptConsent bool) string
	ExchangeCodeWithToken(ctx context.Context, code string, opts ...oauth2.AuthCodeOption) (*oauth2.Token, error)
	CreateHTTPClient(ctx context.Context, token *oauth2.Token) *http.Client
	CreateTokenSource(ctx context.Context, token *oauth2.Token) oauth2.TokenSource
}

type oauth2ConfigInstance struct {
	oauth2Config oauth2.Config
}

func (o *oauth2ConfigInstance) GenerateVerifier() string {
	return oauth2.GenerateVerifier()
}

func (o *oauth2ConfigInstance) GenerateAuthURL(state, verifier string, promptAccountSelect, promptConsent bool) string {
	opts := []oauth2.AuthCodeOption{
		oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier),
	}
	prompts := make([]string, 0)

	// prompt user to select an account
	if promptAccountSelect {
		prompts = append(prompts, selectAccountPrompt)
	}

	// prompt user for App consent
	if promptConsent {
		prompts = append(prompts, consentPrompt)
	}

	if len(prompts) > 0 {
		opts = append(opts, oauth2.SetAuthURLParam("prompt", strings.Join(prompts, " ")))
	}

	return o.oauth2Config.AuthCodeURL(state, opts...)
}

func (o *oauth2ConfigInstance) ExchangeCodeWithToken(ctx context.Context, code string,
	opts ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	const funcName = "exchangeCodeWithTokenSource"

	token, err := o.oauth2Config.Exchange(ctx, code, opts...)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to retrieve auth token, error: %s", err.Error()),
			logging.FuncNameAttr(funcName))
		return nil, err
	}

	return token, nil
}

func (o *oauth2ConfigInstance) CreateHTTPClient(ctx context.Context, token *oauth2.Token) *http.Client {
	return o.oauth2Config.Client(ctx, token)
}

func (o *oauth2ConfigInstance) CreateTokenSource(ctx context.Context, token *oauth2.Token) oauth2.TokenSource {
	return o.oauth2Config.TokenSource(ctx, token)
}
