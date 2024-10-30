package auth

import (
	"checkYoutube/handlers"
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

const (
	oauth2SessionName = "oauth2_session"
	verifierKey       = "verifier"
)

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
	return func(w http.ResponseWriter, r *http.Request) {
		// add and retrieve session
		session, err := sessionStore.Get(r, oauth2SessionName)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to get session: %s", err.Error()))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// generate and store oauth code verifier
		verifier := oauth2C.generateVerifier()
		session.Values[verifierKey] = verifier

		// set session cookie in the response
		err = session.Save(r, w)
		if err != nil {
			slog.Error(fmt.Sprintf("failed to save session: %s", err.Error()))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// redirect to the Google's auth url
		url := oauth2C.generateAuthURL("state", verifier, true)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	}
}

// Oauth2Redirect oauth2 redirect landing endpoint
func Oauth2Redirect(oauth2C Oauth2Config, serverBasepath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// retrieve verifier from context
		verifier, verifierOk := r.Context().Value(verifierCtxKey{}).(string)
		if !verifierOk {
			err := fmt.Errorf("verifier not found in context")
			slog.Error(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// get code from request URL
		code := r.URL.Query().Get("code")
		err := getToken(oauth2C, code, verifier)
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// redirect to YouTube check endpoint
		http.Redirect(w, r, fmt.Sprintf("%s/check-youtube?filtered=true", serverBasepath), http.StatusSeeOther)
	}
}

// get auth token from OAUTH2 code exchange, and init the services
func getToken(oauth2C Oauth2Config, code, verifier string) error {
	ctx := context.Background()

	// get the token source from token exchange
	ts, err := oauth2C.exchangeCodeWithTokenSource(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		slog.Error(fmt.Sprintf("failed to exchange auth code with token, error: %s", err.Error()))
		return err
	}

	// init the http client using the token
	token, err := ts.Token()
	if err != nil {
		slog.Error(fmt.Sprintf("failed to get token from token source, error: %s", err.Error()))
		return err
	}

	// init services
	err = handlers.InitServices(ts, oauth2C.createHTTPClient(ctx, token))
	if err != nil {
		slog.Error(fmt.Sprintf("failed to init services, error: %s", err.Error()))
		return err
	}

	slog.Info("user successfully authenticated")
	return nil
}

// SwitchAccount redirect the user to select an account
func SwitchAccount(oauth2C Oauth2Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// retrieve verifier from context
		verifier, verifierOk := r.Context().Value(verifierCtxKey{}).(string)
		if !verifierOk {
			errMsg := "verifier not found in context"
			slog.Error(errMsg)
			http.Error(w, errMsg, http.StatusInternalServerError)
			return
		}

		// redirect to the Google's auth url
		url := oauth2C.generateAuthURL("state", verifier, true)
		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	}
}

// CheckVerifierMiddleware redirects the user if the oauth2 verifier is not found in the session
func CheckVerifierMiddleware(next http.Handler, sessionStore *sessions.CookieStore, serverBasepath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		verifier, err := getValueFromSession(sessionStore, r, oauth2SessionName, verifierKey)
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if verifier == "" {
			// redirect to login page
			slog.Warn("verifier not found in session, redirect to login")
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

// getValueFromSession returns the data having the given key from the session store
func getValueFromSession(sessionStore *sessions.CookieStore, r *http.Request, sessionName, key string) (string, error) {
	var value string

	// retrieve session from cookie
	session, err := sessionStore.Get(r, sessionName)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to get session: %s", err.Error()))
		return value, err
	}

	// retrieve value from session
	value, verifierOk := session.Values[key].(string)
	if session.IsNew || !verifierOk {
		slog.Warn(fmt.Sprintf("session is expired or value with key '%s' is invalid", key))
	}

	return value, nil
}
