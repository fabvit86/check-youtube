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
	"sync"
)

type oauth2Config struct {
	provider       Oauth2ConfigProvider
	oauth2Verifier string
}

// package-shared singleton
var oauth2C *oauth2Config
var once sync.Once

// InitOauth2Config initializes the oauth2 config as singleton
func InitOauth2Config(clientID, clientSecret, redirectURL string) {
	once.Do(func() {
		oauth2C = &oauth2Config{
			provider: &oauth2ConfigInstance{
				oauth2Config: oauth2.Config{
					ClientID:     clientID,
					ClientSecret: clientSecret,
					Endpoint:     google.Endpoint,
					RedirectURL:  redirectURL,
					Scopes:       []string{youtube.YoutubeScope, people.UserinfoProfileScope},
				},
			},
		}
	})
}

// Login oauth2 login
func Login(w http.ResponseWriter, r *http.Request) {
	// generate and store oauth code verifier
	oauth2C.oauth2Verifier = oauth2C.provider.generateVerifier()

	// get auth url for user's authentication
	url := oauth2C.provider.generateAuthURL("state", oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(oauth2C.oauth2Verifier))

	// redirect to the Google's auth url
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Oauth2Redirect oauth2 redirect landing endpoint
func Oauth2Redirect(port string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		http.Redirect(w, r, fmt.Sprintf("http://localhost:%s/check-youtube?filtered=true", port), http.StatusSeeOther)
	}
}

// get auth token from OAUTH2 code exchange, and init the services
func getToken(code string) error {
	ctx := context.Background()

	// get the token
	token, err := oauth2C.provider.exchangeCodeWithToken(ctx, code, oauth2.VerifierOption(oauth2C.oauth2Verifier))
	if err != nil {
		log.Println(fmt.Sprintf("failed to exchange auth code with token, error: %v", err))
		return err
	}

	// init http client
	handlers.Client = oauth2C.provider.createHTTPClient(ctx, token)

	// init YouTube service
	err = handlers.InitService(handlers.YoutubeService)
	if err != nil {
		log.Println(fmt.Sprintf("failed to init YouTube service, error: %v", err))
		return err
	}

	// init Google People service
	err = handlers.InitService(handlers.PeopleService)
	if err != nil {
		log.Println(fmt.Sprintf("failed to init People service, error: %v", err))
		return err
	}

	log.Println("user successfully authenticated")
	return nil
}

// SwitchAccount redirect the user to select an account
func SwitchAccount(w http.ResponseWriter, r *http.Request) {
	promptAccountSelect := oauth2.SetAuthURLParam("prompt", "select_account")
	url := oauth2C.provider.generateAuthURL("state", oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(oauth2C.oauth2Verifier), promptAccountSelect)

	// redirect to the Google's auth url
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
