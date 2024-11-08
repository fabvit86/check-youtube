package sessions

import (
	"checkYoutube/logging"
	"fmt"
	"github.com/gorilla/sessions"
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
		slog.Warn(err.Error(), logging.FuncNameAttr(funcName))
		return value, err
	}

	// check if session is expired
	if session.IsNew {
		slog.Warn("session is expired, created new session", logging.FuncNameAttr(funcName))
	}

	return value, nil
}
