package main

import (
	"checkYoutube/auth"
	"checkYoutube/handlers"
	"checkYoutube/utils"
	"embed"
	_ "embed"
	"fmt"
	"github.com/gorilla/sessions"
	"log/slog"
	"net/http"
	"os"
)

//go:embed static
var staticContent embed.FS

//go:embed htmlTemplate.tmpl
var htmlTemplate []byte

func main() {
	// configure logger
	utils.ConfigureLogger(utils.GetEnvOrFallback("LOG_LEVEL", slog.LevelInfo.String()))

	// get env veriables
	port := utils.GetEnvOrFallback("SERVER_PORT", "8900")
	serverBasepath := fmt.Sprintf("http://localhost:%s", port)
	clientID, err := utils.GetEnvOrErr("CLIENT_ID")
	if err != nil {
		slog.Error(err.Error())
		os.Exit(-1)
	}
	clientSecret, err := utils.GetEnvOrErr("CLIENT_SECRET")
	if err != nil {
		slog.Error(err.Error())
		os.Exit(-1)
	}
	redirectURL, err := utils.GetEnvOrErr("OAUTH_LANDING_PAGE")
	if err != nil {
		slog.Error(err.Error())
		os.Exit(-1)
	}

	// session storage, used to store the data needed for the oauth2 login flow
	sessionStore := sessions.NewCookieStore([]byte((os.Getenv("SESSION_KEY"))))

	// create oauth2 config
	oauth2C := auth.CreateOauth2Config(clientID, clientSecret, redirectURL)

	// register handlers
	http.HandleFunc("/login", auth.Login(oauth2C, sessionStore))
	http.HandleFunc("/landing", auth.CheckVerifierMiddleware(
		auth.Oauth2Redirect(oauth2C, serverBasepath), sessionStore, serverBasepath))
	http.HandleFunc("/check-youtube", handlers.GetYoutubeChannelsVideosNotification(serverBasepath, string(htmlTemplate)))
	http.HandleFunc("/switch-account", auth.CheckVerifierMiddleware(
		auth.SwitchAccount(oauth2C), sessionStore, serverBasepath))
	http.HandleFunc("/mark-as-viewed", handlers.MarkAsViewed(serverBasepath))
	http.Handle("/static/", http.FileServer(http.FS(staticContent)))

	// start the server
	slog.Info(fmt.Sprintf("listening on port %s...", port))
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		slog.Error(err.Error())
		os.Exit(-1)
	}
}
