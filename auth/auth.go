package auth

import (
	"checkYoutube/clients"
	"checkYoutube/database"
	"checkYoutube/errors"
	"checkYoutube/logging"
	sessionsutils "checkYoutube/sessions"
	"context"
	errors2 "errors"
	"fmt"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/people/v1"
	"google.golang.org/api/youtube/v3"
	"log/slog"
	"net/http"
	"net/url"
)

// Oauth2Config embeds the interface that wraps an oauth2.Config
type Oauth2Config struct {
	Oauth2ConfigProvider
}

// TokenInfo contains the oauth2 token and additional user information
type TokenInfo struct {
	Token    *oauth2.Token
	Username string
	UserId   string
}

type verifierCtxKey struct{}

func (verifierCtxKey) String() string { return "verifier" }

type TokenCtxKey struct{}

func (TokenCtxKey) String() string { return "tokenInfo" }

const (
	selectAccountPrompt = "select_account"
	consentPrompt       = "consent"
	trueStr             = "true"
	falseStr            = "false"
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
	const funcName = "Login"
	return func(w http.ResponseWriter, r *http.Request) {
		selectAccountParam := r.URL.Query().Get(selectAccountPrompt)
		promptAccountSelect := selectAccountParam == trueStr || selectAccountParam == ""
		promptConsent := r.URL.Query().Get(consentPrompt) == trueStr

		// add and retrieve session
		session, err := sessionStore.Get(r, sessionsutils.Oauth2SessionName)
		if err != nil {
			err = errors.GetSessionErr{Err: err}
			slog.Error(err.Error(), logging.FuncNameAttr(funcName))
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
			err = errors.SaveSessionErr{Err: err}
			slog.Error(err.Error(), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// redirect to the Google's auth url
		authUrl := oauth2C.GenerateAuthURL("state", verifier, promptAccountSelect, promptConsent)
		http.Redirect(w, r, authUrl, http.StatusTemporaryRedirect)
	}
}

// Oauth2Redirect oauth2 redirect landing endpoint
func Oauth2Redirect(oauth2C Oauth2Config, sessionStore *sessions.CookieStore, storage database.StorageInterface,
	pcf clients.PeopleClientFactoryInterface, serverBasepath string) http.HandlerFunc {
	const funcName = "Oauth2Redirect"
	return func(w http.ResponseWriter, r *http.Request) {
		// retrieve verifier from context
		verifier, verifierOk := r.Context().Value(verifierCtxKey{}).(string)
		if !verifierOk {
			err := errors.ValueNotFoundInCtx{Key: verifierCtxKey{}}
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

		// exchange code with token
		token, err := oauth2C.ExchangeCodeWithToken(r.Context(), code, oauth2.VerifierOption(verifier))
		if err != nil {
			slog.Error(fmt.Sprintf("failed to retrieve auth token, error: %s", err.Error()),
				logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// use people service to get username of the logged user
		peopleSvc, err := pcf.NewClient(oauth2C.CreateTokenSource(r.Context(), token))
		if err != nil {
			slog.Error(fmt.Sprintf("unable to create people service: %s",
				err.Error()), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		userinfo := peopleSvc.GetLoggedUserinfo()
		userId := userinfo.Id
		username := userinfo.DisplayName

		// store refresh token into the database
		if token.RefreshToken != "" {
			err = storage.UpsertRefreshToken(userId, token.RefreshToken)
			if err != nil {
				slog.Error(fmt.Sprintf("failed to store refresh token into db: %s", err.Error()),
					logging.FuncNameAttr(funcName), logging.UserAttr(username))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			slog.Info("user's refresh token updated", logging.FuncNameAttr(funcName), logging.UserAttr(username))
		}

		// store token and user info in session
		session.Values[sessionsutils.TokenKey] = &TokenInfo{
			Token:    token,
			Username: username,
			UserId:   userId,
		}

		// save session
		err = session.Save(r, w)
		if err != nil {
			err = errors.SaveSessionErr{Err: err}
			slog.Error(err.Error(), logging.FuncNameAttr(funcName), logging.UserAttr(username))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// redirect to YouTube check endpoint
		slog.Info("user successfully authenticated", logging.FuncNameAttr(funcName), logging.UserAttr(username))
		http.Redirect(w, r, fmt.Sprintf("%s/check-youtube?filtered=true", serverBasepath), http.StatusSeeOther)
	}
}

// SwitchAccount redirect the user to select an account
func SwitchAccount(oauth2C Oauth2Config) http.HandlerFunc {
	const funcName = "SwitchAccount"
	return func(w http.ResponseWriter, r *http.Request) {
		// retrieve verifier from context
		verifier, verifierOk := r.Context().Value(verifierCtxKey{}).(string)
		if !verifierOk {
			err := errors.ValueNotFoundInCtx{Key: verifierCtxKey{}}
			slog.Error(err.Error(), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// redirect to the Google's auth url
		authUrl := oauth2C.GenerateAuthURL("state", verifier, true, false)
		http.Redirect(w, r, authUrl, http.StatusTemporaryRedirect)
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
			if errors2.As(err, &errors.GetSessionErr{}) {
				slog.Error(err.Error(), logging.FuncNameAttr(funcName))
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				// redirect to login
				slog.Warn(fmt.Sprintf("redirect to login: %s", err.Error()), logging.FuncNameAttr(funcName))
				http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			}
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

// CheckTokenMiddleware retrieves the token from the session, validates it, refreshes it and stores it in the context
func CheckTokenMiddleware(next http.Handler, oauth2C Oauth2Config, storage database.StorageInterface,
	sessionStore *sessions.CookieStore, serverBasepath string) http.HandlerFunc {
	const funcName = "CheckTokenMiddleware"
	return func(w http.ResponseWriter, r *http.Request) {
		// get token from session
		tokenInfo, err := sessionsutils.GetValueFromSession[*TokenInfo](sessionStore, r,
			sessionsutils.Oauth2SessionName, sessionsutils.TokenKey)
		if err != nil {
			if errors2.As(err, &errors.GetSessionErr{}) {
				slog.Error(err.Error(), logging.FuncNameAttr(funcName))
				http.Error(w, err.Error(), http.StatusInternalServerError)
			} else {
				// redirect to login
				slog.Warn(fmt.Sprintf("redirect to login: %s", err.Error()), logging.FuncNameAttr(funcName))
				http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			}
			return
		}

		// check if token is valid, if not try to refresh it
		if !tokenInfo.Token.Valid() {
			var err error
			var refreshToken string
			if tokenInfo.Token != nil {
				slog.Warn("token is expired, trying to refresh it",
					logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))

				// retrieve refresh token from db for session user
				slog.Info("trying to retrieve refresh token from db",
					logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
				refreshToken, err = storage.GetRefreshTokenByUserId(tokenInfo.UserId)
				if err != nil {
					slog.Error(err.Error(), logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				// refresh the token
				if refreshToken != "" {
					slog.Warn("token is nil or expired, trying to refresh it",
						logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
					tokenInfo.Token.RefreshToken = refreshToken
					tokenInfo.Token, err = oauth2C.CreateTokenSource(r.Context(), tokenInfo.Token).Token()
				} else {
					slog.Warn("refresh token not found in db",
						logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
					err = fmt.Errorf("refresh token not found in db")
				}
			} else {
				err = fmt.Errorf("token not found in session")
			}
			if err != nil {
				slog.Error(fmt.Sprintf("unable to refresh token, redirect to login: %s", err.Error()),
					logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))

				// redirect to login, prompting the user for consent in order to get back a refresh token
				loginUrl, parseErr := url.Parse(fmt.Sprintf("%s/login", serverBasepath))
				if parseErr != nil {
					slog.Error(fmt.Sprintf("failed to parse login url: %s", parseErr.Error()),
						logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
					http.Error(w, parseErr.Error(), http.StatusInternalServerError)
					return
				}
				queryParams := url.Values{}
				queryParams.Add(selectAccountPrompt, falseStr)
				queryParams.Add(consentPrompt, trueStr)
				loginUrl.RawQuery = queryParams.Encode()
				http.Redirect(w, r, loginUrl.String(), http.StatusTemporaryRedirect)
				return
			}
			slog.Info("token has been refreshed", logging.FuncNameAttr(funcName),
				logging.UserAttr(tokenInfo.Username))

			// update refresh token into db
			err = storage.UpsertRefreshToken(tokenInfo.UserId, tokenInfo.Token.RefreshToken)
			if err != nil {
				slog.Error(err.Error(), logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			// update tokenInfo in session with refreshed token
			session, err := sessionStore.Get(r, sessionsutils.Oauth2SessionName)
			if err != nil {
				err = errors.GetSessionErr{Err: err}
				slog.Error(err.Error(), logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			session.Values[sessionsutils.TokenKey] = tokenInfo
			err = session.Save(r, w)
			if err != nil {
				err = errors.SaveSessionErr{Err: err}
				slog.Error(err.Error(), logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		// add token to context
		ctx := r.Context()
		ctx = context.WithValue(ctx, TokenCtxKey{}, tokenInfo)
		r = r.WithContext(ctx)

		// serve next handler in the chain
		next.ServeHTTP(w, r)
	}
}
