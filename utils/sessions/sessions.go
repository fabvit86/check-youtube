package sessions

import (
	"checkYoutube/logging"
	"context"
	"fmt"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"log/slog"
	"net/http"
)

const (
	Oauth2SessionName = "oauth2_session"
	VerifierKey       = "verifier"
	TokenKey          = "token"
)

type TokenCtxKey struct{}

// GetValueFromSession returns the data having the given key from the session store
func GetValueFromSession[T any](sessionStore *sessions.CookieStore, r *http.Request, sessionName, key string) (T, error) {
	const funcName = "GetValueFromSession"
	var value T

	// retrieve session from cookie
	session, err := sessionStore.Get(r, sessionName)
	if err != nil {
		slog.Error(fmt.Sprintf("failed to get session: %s", err.Error()), logging.FuncNameAttr(funcName))
		return value, err
	}

	// retrieve value from session
	value, verifierOk := session.Values[key].(T)
	if !verifierOk {
		err = fmt.Errorf("session value with key '%s' is nil or of the wrong type", key)
		slog.Error(err.Error(), logging.FuncNameAttr(funcName))
		return value, err
	}

	// check if session is expired
	if session.IsNew {
		slog.Warn("session is expired, created new session", logging.FuncNameAttr(funcName))
	}

	return value, nil
}

// CheckTokenMiddleware retrieves the token from the session, validates it and stores it in the context
func CheckTokenMiddleware(next http.Handler, sessionStore *sessions.CookieStore, serverBasepath string) http.HandlerFunc {
	const funcName = "CheckTokenMiddleware"
	return func(w http.ResponseWriter, r *http.Request) {
		// get token from session
		token, err := GetValueFromSession[*oauth2.Token](sessionStore, r, Oauth2SessionName, TokenKey)
		if err != nil {
			slog.Warn(fmt.Sprintf("session value with key '%s' is invalid, redirect to login: %s",
				TokenKey, err.Error()), logging.FuncNameAttr(funcName))
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// check if token is valid
		if !token.Valid() {
			err = fmt.Errorf("token is nil or expired, redirect to login")
			slog.Error(err.Error(), logging.FuncNameAttr(funcName))
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// add token to context
		ctx := r.Context()
		ctx = context.WithValue(ctx, TokenCtxKey{}, token)
		r = r.WithContext(ctx)

		// serve next handler in the chain
		next.ServeHTTP(w, r)
	}
}
