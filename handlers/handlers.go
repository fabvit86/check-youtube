package handlers

import (
	"checkYoutube/auth"
	"checkYoutube/logging"
	sessionsutils "checkYoutube/utils/sessions"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/oauth2"
	"google.golang.org/api/youtube/v3"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
)

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

// GetYoutubeChannelsVideos call YouTube API to check for new videos
func GetYoutubeChannelsVideos(oauth2C auth.Oauth2Config, ytcf YoutubeClientFactoryInterface,
	pcf PeopleClientFactoryInterface, serverBasepath, htmlTemplate string) http.HandlerFunc {
	const funcName = "GetYoutubeChannelsVideos"
	return func(w http.ResponseWriter, r *http.Request) {
		filtered := r.URL.Query().Get("filtered") == "true"

		// create youtube service
		youtubeSvc, err := ytcf.NewClient(oauth2C, r)
		if err != nil {
			slog.Warn(fmt.Sprintf("unable to create youtube service, redirecting user to login page: %s",
				err.Error()), logging.FuncNameAttr(funcName))
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// create people service
		peopleSvc, err := pcf.NewClient(oauth2C, r)
		if err != nil {
			slog.Warn(fmt.Sprintf("unable to create people service, redirecting user to login page: %s",
				err.Error()), logging.FuncNameAttr(funcName))
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// get user info using the Google People API
		username := peopleSvc.getLoggedUserinfo()

		// get YouTube subscriptions info
		ytChannels := checkYoutube(youtubeSvc, filtered)

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
	const funcName = "checkYoutube"
	response := make([]YTChannel, 0)
	ctx := context.Background()

	if svc == nil {
		slog.Warn("uninitialized youtube service", logging.FuncNameAttr(funcName))
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
						slog.Warn(fmt.Sprintf("failed to retrieve latest YouTube video from playlist, "+
							"skipping info for channel %s", responseItem.Title), logging.FuncNameAttr(funcName))
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
		slog.Error(fmt.Sprintf("error retrieving YouTube subscriptions list: %s", err.Error()),
			logging.FuncNameAttr(funcName))
		return response
	}

	if len(response) == 0 {
		slog.Info("no new video published by user's YouTube channels", logging.FuncNameAttr(funcName))
	}

	// sort results by title
	slices.SortFunc(response, func(a, b YTChannel) int {
		return cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})

	return response
}

// check a subscription for new videos and add it to the list
func processYouTubeChannel(svc YoutubeClientInterface, item *youtube.Subscription) (YTChannel, error) {
	const (
		funcName        = "processYouTubeChannel"
		youTubeBasepath = "https://www.youtube.com"
	)
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
		slog.Error(fmt.Sprintf("error retrieving latest YouTube video from playlist: %s", err.Error()),
			logging.FuncNameAttr(funcName))
		return responseItem, err
	}
	if playlistItem != nil {
		responseItem.LatestVideoURL = fmt.Sprintf("%s/watch?v=%s", youTubeBasepath, playlistItem.Snippet.ResourceId.VideoId)
		responseItem.LatestVideoTitle = playlistItem.Snippet.Title
		slog.Debug(fmt.Sprintf("found latest video for channel %s", channelTitle),
			logging.FuncNameAttr(funcName))
	}

	return responseItem, nil
}

// MarkAsViewed visits a subscription channel in the background to clear the notification of new videos
func MarkAsViewed(oauth2C auth.Oauth2Config, serverBasepath string) http.HandlerFunc {
	const funcName = "MarkAsViewed"
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			err := fmt.Errorf("empty request body")
			slog.Warn(err.Error(), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var req callUrlRequest
		dec := json.NewDecoder(r.Body)
		err := dec.Decode(&req)
		if err != nil {
			slog.Error(err.Error(), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// retrieve token from context
		token, tokenOk := r.Context().Value(sessionsutils.TokenCtxKey{}).(*oauth2.Token)
		if !tokenOk {
			err := fmt.Errorf("token not found in context")
			slog.Error(err.Error(), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// create HTTP client
		client := oauth2C.CreateHTTPClient(r.Context(), token)
		if client == nil {
			slog.Warn("http client not initialized, redirecting user to login page",
				logging.FuncNameAttr(funcName))
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// visit the channel to mark its videos as viewed
		res, err := client.Get(req.URL)
		if err != nil {
			slog.Error(fmt.Sprintf("markAsViewed get request failed, error: %s", err.Error()),
				logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if res.StatusCode != http.StatusOK {
			err = fmt.Errorf("call to youtube returned status: %d", res.StatusCode)
			slog.Error(err.Error(), logging.FuncNameAttr(funcName))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
