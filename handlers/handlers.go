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
	"log/slog"
	"net/http"
	"slices"
	"strings"
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
		err := fmt.Errorf("token source not initialized")
		slog.Error(err.Error())
		return err
	}

	// create YouTube service
	youtubeSvc, err := youtube.NewService(
		ctx,
		option.WithTokenSource(tokenSource),
	)
	if err != nil {
		slog.Error(fmt.Sprintf("unable to create youtube service: %s", err.Error()))
		return err
	}

	// create people service
	peopleSvc, err := people.NewService(
		ctx,
		option.WithTokenSource(tokenSource),
	)
	if err != nil {
		slog.Error(fmt.Sprintf("unable to create people service: %s", err.Error()))
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
			slog.Warn("services not initialized, redirecting user to login page")
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
		slog.Warn("uninitialized youtube service")
		return nil
	}

	// get user's subscriptions list from the YouTube API
	err := svc.getAndProcessSubscriptions(ctx, func(subs *youtube.SubscriptionListResponse) error {
		// collect channels having published new videos
		ch := make(chan YTChannel)
		var itemsCount int
		for _, item := range subs.Items {
			newItems := item.ContentDetails.NewItemCount
			if !filtered || newItems > 0 {
				itemsCount++
				go func(item *youtube.Subscription) {
					err := processYouTubeChannel(svc, item, ch)
					if err != nil {
						slog.Warn("failed to retrieve latest YouTube video from playlist, "+
							"skipping info for channel ", item.Snippet.Title)
					}
				}(item)
			} else {
				break
			}
		}

		for i := 0; i < itemsCount; i++ {
			response = append(response, <-ch)
		}

		return nil
	})
	if err != nil {
		slog.Error(fmt.Sprintf("error retrieving YouTube subscriptions list: %s", err.Error()))
		return response
	}

	if len(response) == 0 {
		slog.Info("no new video published by user's YouTube channels")
	}

	// sort results by title
	slices.SortFunc(response, func(a, b YTChannel) int {
		return cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})

	return response
}

// check a subscription for new videos and add it to the list
func processYouTubeChannel(svc YoutubeClientInterface, item *youtube.Subscription, ch chan<- YTChannel) error {
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
		slog.Error(fmt.Sprintf("error retrieving latest YouTube video from playlist: %s", err.Error()))
		return err
	}
	if playlistItem != nil {
		responseItem.LatestVideoURL = fmt.Sprintf("%s/watch?v=%s", youTubeBasepath, playlistItem.Snippet.ResourceId.VideoId)
		responseItem.LatestVideoTitle = playlistItem.Snippet.Title
		slog.Debug(fmt.Sprintf("found latest video for channel %s", channelTitle))
	}

	ch <- responseItem
	return nil
}

// MarkAsViewed visits a subscription channel in the background to clear the notification of new videos
func MarkAsViewed(serverBasepath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if clientSvc.client == nil {
			slog.Warn("http client not initialized, redirecting user to login page")
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		if r.Body == nil {
			err := fmt.Errorf("empty request body")
			slog.Warn(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req callUrlRequest
		dec := json.NewDecoder(r.Body)
		err := dec.Decode(&req)
		if err != nil {
			slog.Error(err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// visit the channel to mark its videos as viewed
		res, err := clientSvc.client.Get(req.URL)
		if err != nil {
			slog.Error(fmt.Sprintf("markAsViewed get request failed, error: %s", err.Error()))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if res.StatusCode != http.StatusOK {
			err = fmt.Errorf("call to youtube returned status: %d", res.StatusCode)
			slog.Error(err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
