package main

import (
	"checkYoutube/auth"
	"checkYoutube/handlers"
	"embed"
	_ "embed"
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"net/http"
	"os"
)

//go:embed static
var staticContent embed.FS

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	port := os.Getenv("SERVER_PORT")

	// serve endpoints
	http.HandleFunc("/login", auth.Login)
	http.HandleFunc("/landing", auth.Oauth2Redirect)
	http.HandleFunc("/check-youtube", handlers.GetYoutubeChannelsVideosNotification)
	http.Handle("/static/", http.FileServer(http.FS(staticContent)))

	log.Println(fmt.Sprintf("listening on port %s...", port))
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatal(err)
	}
}
