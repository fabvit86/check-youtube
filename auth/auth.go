package auth

import (
	"checkYoutube/handlers"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/people/v1"
	"google.golang.org/api/youtube/v3"
	"log"
	"net/http"
	"os"
)

var oauth2Config oauth2.Config
var oauth2Verifier string

// Login oauth2 login
func Login(w http.ResponseWriter, r *http.Request) {
	// oauth authentication
	oauth2Config = oauth2.Config{
		ClientID:     os.Getenv("CLIENT_ID"),
		ClientSecret: os.Getenv("CLIENT_SECRET"),
		Endpoint:     google.Endpoint,
		RedirectURL:  os.Getenv("OAUTH_LANDING_PAGE"),
		Scopes:       []string{youtube.YoutubeScope, people.UserinfoProfileScope},
	}
	oauth2Verifier = oauth2.GenerateVerifier()

	// get auth url for user's authentication
	url := oauth2Config.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(oauth2Verifier))

	// redirect to the Google's auth url
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Oauth2Redirect oauth2 redirect landing endpoint
func Oauth2Redirect(w http.ResponseWriter, r *http.Request) {
	// get code from request URL
	code := r.URL.Query().Get("code")
	err := getToken(code)
	if err != nil {
		encErr := json.NewEncoder(w).Encode(err.Error())
		if encErr != nil {
			log.Fatal(encErr)
		}
	}

	// redirect to YouTube check endpoint
	http.Redirect(w, r, fmt.Sprintf("http://localhost:%s/check-youtube", os.Getenv("SERVER_PORT")), http.StatusSeeOther)
}

// get auth token from OAUTH2 code exchange, and init the services
func getToken(code string) error {
	ctx := context.Background()

	// get the token
	token, err := oauth2Config.Exchange(ctx, code, oauth2.VerifierOption(oauth2Verifier))
	if err != nil {
		log.Println(fmt.Sprintf("failed to retrieve auth token, error: %v", err))
		return err
	}

	// init YouTube service
	err = handlers.InitYoutubeService(oauth2Config, token)
	if err != nil {
		log.Println(fmt.Sprintf("failed to init YouTube service, error: %v", err))
		return err
	}

	// init Google People service
	err = handlers.InitPeopleService(oauth2Config, token)
	if err != nil {
		log.Println(fmt.Sprintf("failed to init People service, error: %v", err))
		return err
	}

	log.Println("user successfully authenticated")
	return nil
}
