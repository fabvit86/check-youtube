package main

import (
	"checkYoutube/auth"
	"checkYoutube/handlers"
	"checkYoutube/utils"
	"embed"
	_ "embed"
	"fmt"
	"log"
	"net/http"
)

//go:embed static
var staticContent embed.FS

//go:embed htmlTemplate.tmpl
var htmlTemplate []byte

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	port := utils.GetEnvOrFallback("SERVER_PORT", "8900")
	serverBasepath := fmt.Sprintf("http://localhost:%s", port)
	clientID, err := utils.GetEnvOrErr("CLIENT_ID")
	if err != nil {
		log.Fatal(err)
	}
	clientSecret, err := utils.GetEnvOrErr("CLIENT_SECRET")
	if err != nil {
		log.Fatal(err)
	}
	RedirectURL, err := utils.GetEnvOrErr("OAUTH_LANDING_PAGE")
	if err != nil {
		log.Fatal(err)
	}

	// init oauth2 config
	auth.InitOauth2Config(clientID, clientSecret, RedirectURL)

	// serve endpoints
	http.HandleFunc("/login", auth.Login)
	http.HandleFunc("/landing", auth.Oauth2Redirect(serverBasepath))
	http.HandleFunc("/check-youtube", handlers.GetYoutubeChannelsVideosNotification(serverBasepath, string(htmlTemplate)))
	http.HandleFunc("/switch-account", auth.SwitchAccount)
	http.HandleFunc("/mark-as-viewed", handlers.MarkAsViewed(serverBasepath))
	http.Handle("/static/", http.FileServer(http.FS(staticContent)))

	log.Println(fmt.Sprintf("listening on port %s...", port))
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatal(err)
	}
}
