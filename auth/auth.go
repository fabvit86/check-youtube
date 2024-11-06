package auth

import (
	"checkYoutube/logging"
	sessionsutils "checkYoutube/utils/sessions"
	"context"
	"fmt"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/people/v1"
	"google.golang.org/api/youtube/v3"
	"log/slog"
	"net/http"
)

// Oauth2Config embeds the interface that wraps an oauth2.Config
type Oauth2Config struct {
	Oauth2ConfigProvider
}

type verifierCtxKey struct{}

// CreateOauth2Config creates a new Oauth2Config instance
func CreateOauth2Config(clientID, clientSecret, redirectURL string) Oauth2Config {
	return Oauth2Config{
		&oauth2ConfigInstance{
			oauth2Config: oauth2.Config{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Endpoint:     google.Endpoint,
				RedirectURL:  redirectURL,
				Scopes:       []string{youtube.YoutubeScope, people.UserinfoProfileScope},
			},
		},
	}
}

// Login oauth2 login
func Login(oauth2C Oauth2Config, sessionStore *sessions.CookieStore) http.HandlerFunc {
	const funcName = "Login"
	return func(w http.ResponseWriter, r *http.Request) {
		// add and retrieve session
		session, err := sessionStore.Get(r, sessionsutils.Oauth2SessionName)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to get session: %s", err.Error()), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// generate and store oauth code verifier
		verifier := oauth2C.GenerateVerifier()
		session.Values[sessionsutils.VerifierKey] = verifier

		// set session cookie in the response
		session.Options.HttpOnly = true
		err = session.Save(r, w)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to save session: %s", err.Error()), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// redirect to the Google's auth url
		url := oauth2C.GenerateAuthURL("state", verifier, true)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	}
}

// Oauth2Redirect oauth2 redirect landing endpoint
func Oauth2Redirect(oauth2C Oauth2Config, sessionStore *sessions.CookieStore, serverBasepath string) http.HandlerFunc {
	const funcName = "Oauth2Redirect"
	return func(w http.ResponseWriter, r *http.Request) {
		// retrieve verifier from context
		verifier, verifierOk := r.Context().Value(verifierCtxKey{}).(string)
		if !verifierOk {
			err := fmt.Errorf("verifier not found in context")
			slog.Error(err.Error(), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// get code from request URL
		code := r.URL.Query().Get("code")

		// get session
		session, err := sessionStore.Get(r, sessionsutils.Oauth2SessionName)
		if err != nil {
			slog.Error(err.Error(), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// exchange code with token, store token in session
		err = getAndStoreToken(oauth2C, session, code, verifier)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to retrieve session: %s", err.Error()), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// save session
		err = session.Save(r, w)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to save session: %s", err.Error()), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// redirect to YouTube check endpoint
		http.Redirect(w, r, fmt.Sprintf("%s/check-youtube?filtered=true", serverBasepath), http.StatusSeeOther)
	}
}

// get auth token from OAUTH2 code exchange and store it in session
func getAndStoreToken(oauth2C Oauth2Config, session *sessions.Session, code, verifier string) error {
	const funcName = "getToken"
	ctx := context.Background()

	// get the token from token exchange
	token, err := oauth2C.ExchangeCodeWithToken(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		slog.Error(fmt.Sprintf("failed to retrieve auth token, error: %s", err.Error()),
			logging.FuncNameAttr(funcName))
		return err
	}

	// store token in session
	session.Values[sessionsutils.TokenKey] = token

	slog.Info("user successfully authenticated", logging.FuncNameAttr(funcName))
	return nil
}

// SwitchAccount redirect the user to select an account
func SwitchAccount(oauth2C Oauth2Config) http.HandlerFunc {
	const funcName = "SwitchAccount"
	return func(w http.ResponseWriter, r *http.Request) {
		// retrieve verifier from context
		verifier, verifierOk := r.Context().Value(verifierCtxKey{}).(string)
		if !verifierOk {
			errMsg := "verifier not found in context"
			slog.Error(errMsg, logging.FuncNameAttr(funcName))
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}

		// redirect to the Google's auth url
		url := oauth2C.GenerateAuthURL("state", verifier, true)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	}
}

// CheckVerifierMiddleware redirects the user if the oauth2 verifier is not found in the session
func CheckVerifierMiddleware(next http.Handler, sessionStore *sessions.CookieStore, serverBasepath string) http.HandlerFunc {
	const funcName = "CheckVerifierMiddleware"
	return func(w http.ResponseWriter, r *http.Request) {
		// get verifier from session
		verifier, err := sessionsutils.GetValueFromSession[string](sessionStore, r,
			sessionsutils.Oauth2SessionName, sessionsutils.VerifierKey)
		if err != nil {
			// redirect to login page
			slog.Warn(fmt.Sprintf("failed to retrieve verifier from session, redirect to login: %s",
				err.Error()), logging.FuncNameAttr(funcName))
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// add verifier to context
		ctx := r.Context()
		ctx = context.WithValue(ctx, verifierCtxKey{}, verifier)
		r = r.WithContext(ctx)

		// serve next handler in the chain
		next.ServeHTTP(w, r)
	}
}
