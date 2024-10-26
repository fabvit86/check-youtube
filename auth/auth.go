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

// oauth2Config embeds the interface that wraps oauth2.Config
type oauth2Config struct {
	Oauth2ConfigProvider
}

// loginStorage stores the data needed for a single oauth2 login flow
type loginStorage struct {
	oauth2Verifier string
}

var oauthStore loginStorage

// package-shared singleton that specifies oauth2 configuration
var oauth2C *oauth2Config
var once sync.Once

// InitOauth2Config initializes the oauth2 config as singleton
func InitOauth2Config(clientID, clientSecret, redirectURL string) {
	once.Do(func() {
		oauth2C = &oauth2Config{
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
	})
}

// Login oauth2 login
func Login(w http.ResponseWriter, r *http.Request) {
	// generate and store oauth code verifier
	verifier := oauth2C.generateVerifier()

	// get auth url for user's authentication
	url := oauth2C.generateAuthURL("state", oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(verifier))

	// store verifier
	oauthStore = loginStorage{verifier}

	// redirect to the Google's auth url
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

// Oauth2Redirect oauth2 redirect landing endpoint
func Oauth2Redirect(serverBasepath string) http.HandlerFunc {
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
		http.Redirect(w, r, fmt.Sprintf("%s/check-youtube?filtered=true", serverBasepath), http.StatusSeeOther)
	}
}

// get auth token from OAUTH2 code exchange, and init the services
func getToken(code string) error {
	ctx := context.Background()

	// get the token source from token exchange
	ts, err := oauth2C.exchangeCodeWithTokenSource(ctx, code, oauth2.VerifierOption(oauthStore.oauth2Verifier))
	if err != nil {
		log.Println(fmt.Sprintf("failed to exchange auth code with token, error: %v", err))
		return err
	}

	// init the http client using the token
	token, err := ts.Token()
	if err != nil {
		fmt.Println(fmt.Sprintf("failed to get token from token source, error: %v", err))
	}

	// init services
	err = handlers.InitServices(ts, oauth2C.createHTTPClient(ctx, token))
	if err != nil {
		log.Println(fmt.Sprintf("failed to init services, error: %v", err))
		return err
	}

	log.Println("user successfully authenticated")
	return nil
}

// SwitchAccount redirect the user to select an account
func SwitchAccount(w http.ResponseWriter, r *http.Request) {
	promptAccountSelect := oauth2.SetAuthURLParam("prompt", "select_account")
	url := oauth2C.generateAuthURL("state", oauth2.AccessTypeOffline,
		oauth2.S256ChallengeOption(oauthStore.oauth2Verifier), promptAccountSelect)

	// redirect to the Google's auth url
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
