package handlers

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
	"google.golang.org/api/youtube/v3"
	"html/template"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
)

type clientService struct {
	peopleSvc  PeopleClientInterface
	youtubeSvc YoutubeClientInterface
	client     *http.Client
}

var clientSvc clientService

type YTChannel struct {
	Title            string
	URL              string
	LatestVideoURL   string
	LatestVideoTitle string
}

type templateResponse struct {
	YTChannels     []YTChannel
	Username       string
	ServerBasepath string
}

type callUrlRequest struct {
	URL string `json:"url"`
}

// InitServices initializes the required client services using the given token source
func InitServices(tokenSource oauth2.TokenSource, client *http.Client) error {
	ctx := context.Background()

	if tokenSource == nil {
		return fmt.Errorf("error: token source not initialized")
	}

	// create YouTube service
	youtubeSvc, err := youtube.NewService(
		ctx,
		option.WithTokenSource(tokenSource),
	)
	if err != nil {
		log.Println(fmt.Sprintf("unable to create youtube service: %v", err))
		return err
	}

	// create people service
	peopleSvc, err := people.NewService(
		ctx,
		option.WithTokenSource(tokenSource),
	)
	if err != nil {
		log.Println(fmt.Sprintf("unable to create people service: %v", err))
		return err
	}

	clientSvc = clientService{
		peopleSvc: &peopleClient{
			svc: *peopleSvc,
		},
		youtubeSvc: &youtubeClient{
			svc: *youtubeSvc,
		},
		client: client,
	}

	return nil
}

// GetYoutubeChannelsVideosNotification call YouTube API to check for new videos
func GetYoutubeChannelsVideosNotification(serverBasepath, htmlTemplate string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		filtered := r.URL.Query().Get("filtered") == "true"

		if clientSvc.youtubeSvc == nil || clientSvc.peopleSvc == nil {
			// redirect to login page
			log.Println("services not initialized, redirecting user to login page")
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// get user info using the Google People API
		username := clientSvc.peopleSvc.getLoggedUserinfo()

		// get YouTube subscriptions info
		ytChannels := checkYoutube(clientSvc.youtubeSvc, filtered)

		response := templateResponse{
			YTChannels:     ytChannels,
			Username:       username,
			ServerBasepath: serverBasepath,
		}

		// render response as HTML using a template
		tmpl, err := template.New("htmlTemplate.tmpl").Parse(htmlTemplate)
		if err != nil {
			log.Fatal(err)
		}
		err = tmpl.Execute(w, response)
		if err != nil {
			log.Fatal(err)
		}
	}
}

// call YouTube API to check for new videos
func checkYoutube(svc YoutubeClientInterface, filtered bool) []YTChannel {
	response := make([]YTChannel, 0)
	ctx := context.Background()

	if svc == nil {
		log.Println("warning: uninitialized youtube service")
		return nil
	}

	// get user's subscriptions list from the YouTube API
	err := svc.getAndProcessSubscriptions(ctx, func(subs *youtube.SubscriptionListResponse) error {
		// collect channels having published new videos
		wg := &sync.WaitGroup{}
		for _, item := range subs.Items {
			newItems := item.ContentDetails.NewItemCount
			if !filtered || newItems > 0 {
				wg.Add(1)
				go func(item *youtube.Subscription) {
					defer wg.Done()
					responseItem, err := processYouTubeChannel(svc, item)
					if err != nil {
						log.Println("error retrieving latest YouTube video from playlist, "+
							"skipping info for channel ", responseItem.Title)
					}
					response = append(response, responseItem)
				}(item)
			} else {
				break
			}
		}
		wg.Wait()
		return nil
	})
	if err != nil {
		log.Println(fmt.Sprintf("error retrieving YouTube subscriptions list: %v", err))
		return response
	}

	if len(response) == 0 {
		log.Println("no new video published by user's YouTube channels")
	}

	// sort results by title
	slices.SortFunc(response, func(a, b YTChannel) int {
		return cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})

	return response
}

// check a subscription for new videos and add it to the list
func processYouTubeChannel(svc YoutubeClientInterface, item *youtube.Subscription) (YTChannel, error) {
	const youTubeBasepath = "https://www.youtube.com"
	channelTitle := item.Snippet.Title
	channelID := item.Snippet.ResourceId.ChannelId
	responseItem := YTChannel{
		Title: channelTitle,
		URL:   fmt.Sprintf("%s/channel/%s/videos", youTubeBasepath, channelID),
	}

	// the playlist ID can be obtained by changing the second letter of the channel ID
	playlistIDRunes := []rune(channelID)
	playlistIDRunes[1] = 'U'
	playlistID := string(playlistIDRunes)

	// get latest video info from the first playlist item
	playlistItem, err := svc.getLatestVideoFromPlaylist(playlistID)
	if err != nil {
		log.Println("error retrieving latest YouTube video from playlist")
		return responseItem, err
	}
	if playlistItem != nil {
		responseItem.LatestVideoURL = fmt.Sprintf("%s/watch?v=%s", youTubeBasepath, playlistItem.Snippet.ResourceId.VideoId)
		responseItem.LatestVideoTitle = playlistItem.Snippet.Title
		log.Println(fmt.Sprintf("channel %s published new videos", channelTitle))
	}

	return responseItem, nil
}

// MarkAsViewed visits a subscription channel in the background to clear the notification of new videos
func MarkAsViewed(serverBasepath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clientSvc.client == nil {
			log.Println("http client not initialized, redirecting user to login page")
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		if r.Body == nil {
			err := fmt.Errorf("empty request body")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req callUrlRequest
		dec := json.NewDecoder(r.Body)
		err := dec.Decode(&req)
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// visit the channel to mark its videos as viewed
		res, err := clientSvc.client.Get(req.URL)
		if err != nil {
			log.Println(fmt.Sprintf("markAsViewed get request failed, error: %v", err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if res.StatusCode != http.StatusOK {
			err = fmt.Errorf("call to youtube returned status: %d", res.StatusCode)
			log.Println(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
