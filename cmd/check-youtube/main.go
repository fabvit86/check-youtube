package main

import (
	"checkYoutube/auth"
	"checkYoutube/clients"
	"checkYoutube/configs"
	"checkYoutube/handlers"
	"checkYoutube/logging"
	"checkYoutube/web"
	_ "embed"
	"encoding/gob"
	"fmt"
	"github.com/gorilla/sessions"
	"log/slog"
	"net/http"
	"os"
)

func main() {
	const funcName = "main"

	// configure logger
	logging.ConfigureLogger(configs.GetEnvOrFallback("LOG_LEVEL", slog.LevelInfo.String()))

	// get env veriables
	port := configs.GetEnvOrFallback("SERVER_PORT", "8900")
	serverBasepath := fmt.Sprintf("http://localhost:%s", port)
	clientID, err := configs.GetEnvOrErr("CLIENT_ID")
	if err != nil {
		slog.Error(err.Error(), logging.FuncNameAttr(funcName))
		os.Exit(-1)
	}
	clientSecret, err := configs.GetEnvOrErr("CLIENT_SECRET")
	if err != nil {
		slog.Error(err.Error(), logging.FuncNameAttr(funcName))
		os.Exit(-1)
	}
	redirectURL, err := configs.GetEnvOrErr("OAUTH_LANDING_PAGE")
	if err != nil {
		slog.Error(err.Error(), logging.FuncNameAttr(funcName))
		os.Exit(-1)
	}

	// session storage, used to store the data needed for the oauth2 login flow
	sessionStore := sessions.NewCookieStore([]byte((os.Getenv("SESSION_KEY"))))
	gob.Register(&auth.TokenInfo{})

	// create oauth2 config
	oauth2C := auth.CreateOauth2Config(clientID, clientSecret, redirectURL)

	// client services factory
	pcf := &clients.PeopleClientFactory{}
	ytcf := &clients.YoutubeClientFactory{}

	// register handlers
	http.HandleFunc("/login", auth.Login(oauth2C, sessionStore))
	http.HandleFunc("/landing", auth.CheckVerifierMiddleware(
		auth.Oauth2Redirect(oauth2C, sessionStore, pcf, serverBasepath), sessionStore, serverBasepath))
	http.HandleFunc("/check-youtube", auth.CheckTokenMiddleware(
		handlers.GetYoutubeChannelsVideos(oauth2C, ytcf, serverBasepath, string(web.HtmlTemplate)),
		oauth2C, sessionStore, serverBasepath))
	http.HandleFunc("/switch-account", auth.CheckVerifierMiddleware(
		auth.SwitchAccount(oauth2C), sessionStore, serverBasepath))
	http.HandleFunc("/mark-as-viewed", auth.CheckTokenMiddleware(
		handlers.MarkAsViewed(oauth2C, serverBasepath), oauth2C, sessionStore, serverBasepath))
	http.Handle("/static/", http.FileServer(http.FS(web.StaticContent)))

	// start the server
	slog.Info(fmt.Sprintf("listening on port %s...", port), logging.FuncNameAttr(funcName))
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		slog.Error(err.Error(), logging.FuncNameAttr(funcName))
		os.Exit(-1)
	}
}
