package main

import (
	"checkYoutube/auth"
	"checkYoutube/handlers"
	"checkYoutube/utils"
	"embed"
	_ "embed"
	"fmt"
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

	// init oauth2 config
	auth.InitOauth2Config(clientID, clientSecret, redirectURL)

	// register handlers
	http.HandleFunc("/login", auth.Login)
	http.HandleFunc("/landing", auth.CheckVerifierMiddleware(auth.Oauth2Redirect(serverBasepath), serverBasepath))
	http.HandleFunc("/check-youtube", handlers.GetYoutubeChannelsVideosNotification(serverBasepath, string(htmlTemplate)))
	http.HandleFunc("/switch-account", auth.CheckVerifierMiddleware(auth.SwitchAccount(), serverBasepath))
	http.HandleFunc("/mark-as-viewed", handlers.MarkAsViewed(serverBasepath))
	http.Handle("/static/", http.FileServer(http.FS(staticContent)))

	// start the server
	slog.Info(fmt.Sprintf("listening on port %s...", port))
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		slog.Error(err.Error())
		os.Exit(-1)
	}
}
