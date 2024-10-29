package auth

import (
	"checkYoutube/handlers"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/people/v1"
	"google.golang.org/api/youtube/v3"
	"log"
	"log/slog"
	"net/http"
	"os"
	"sync"
)

// oauth2Config embeds the interface that wraps oauth2.Config
type oauth2Config struct {
	Oauth2ConfigProvider
}

type verifierCtxKey struct{}

const (
	oauth2SessionName = "oauth2_session"
	verifierKey       = "verifier"
)

// session storage, used to store the data needed for the oauth2 login flow
var sessionStore = sessions.NewCookieStore([]byte((os.Getenv("SESSION_KEY"))))

// package-shared singleton that specifies oauth2 configuration
var oauth2C *oauth2Config
var once sync.Once

// InitOauth2Config initializes the oauth2 config as singleton
func InitOauth2Config(clientID, clientSecret, redirectURL string) {
	once.Do(func() {
		oauth2C = &oauth2Config{
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
	})
}

// Login oauth2 login
func Login(w http.ResponseWriter, r *http.Request) {
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

// Oauth2Redirect oauth2 redirect landing endpoint
func Oauth2Redirect(serverBasepath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// retrieve verifier from context
		verifier, verifierOk := r.Context().Value(verifierCtxKey{}).(string)
		if !verifierOk {
			http.Error(w, "verifier not found in context", http.StatusInternalServerError)
			return
		}

		// get code from request URL
		code := r.URL.Query().Get("code")
		err := getToken(code, verifier)
		if err != nil {
			encErr := json.NewEncoder(w).Encode(err.Error())
			if encErr != nil {
				log.Fatal(encErr)
			}
		}

		// redirect to YouTube check endpoint
		http.Redirect(w, r, fmt.Sprintf("%s/check-youtube?filtered=true", serverBasepath), http.StatusSeeOther)
	}
}

// get auth token from OAUTH2 code exchange, and init the services
func getToken(code, verifier string) error {
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
func SwitchAccount() http.HandlerFunc {
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
func CheckVerifierMiddleware(next http.Handler, serverBasepath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		verifier, err := getValueFromSession(r, oauth2SessionName, verifierKey)
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
func getValueFromSession(r *http.Request, sessionName, key string) (string, error) {
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
