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

	// serve endpoints
	http.HandleFunc("/login", auth.Login(clientID, clientSecret, RedirectURL))
	http.HandleFunc("/landing", auth.Oauth2Redirect(port))
	http.HandleFunc("/check-youtube", handlers.GetYoutubeChannelsVideosNotification(port, string(htmlTemplate)))
	http.HandleFunc("/switch-account", auth.SwitchAccount)
	http.HandleFunc("/mark-as-viewed", handlers.MarkAsViewed)
	http.Handle("/static/", http.FileServer(http.FS(staticContent)))

	log.Println(fmt.Sprintf("listening on port %s...", port))
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatal(err)
	}
}
