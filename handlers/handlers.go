package handlers

import (
	"checkYoutube/auth"
	"checkYoutube/clients"
	"checkYoutube/logging"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"google.golang.org/api/youtube/v3"
	"html/template"
	"log"
	"log/slog"
	"net/http"
	"slices"
	"strings"
)

type YTChannel struct {
	Title            string
	ChannelID        string
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
	ChannelID string `json:"channel_id"`
}

const youTubeBasepath = "https://www.youtube.com"

// GetYoutubeChannelsVideos call YouTube API to check for new videos
func GetYoutubeChannelsVideos(oauth2C auth.Oauth2Config, ytcf clients.YoutubeClientFactoryInterface,
	serverBasepath, htmlTemplate string) http.HandlerFunc {
	const funcName = "GetYoutubeChannelsVideos"
	return func(w http.ResponseWriter, r *http.Request) {
		filtered := r.URL.Query().Get("filtered") == "true"

		// get token from context
		tokenInfo, tokenOk := r.Context().Value(auth.TokenCtxKey{}).(*auth.TokenInfo)
		if !tokenOk {
			slog.Warn(fmt.Sprintf("token not found in context, redirecting user to login page"),
				logging.FuncNameAttr(funcName))
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// create youtube service
		youtubeSvc, err := ytcf.NewClient(oauth2C.CreateTokenSource(r.Context(), tokenInfo.Token))
		if err != nil {
			slog.Warn(fmt.Sprintf("unable to create youtube service, redirecting user to login page: %s",
				err.Error()), logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// get YouTube subscriptions info
		ytChannels := checkYoutube(youtubeSvc, filtered, tokenInfo.Username)

		response := templateResponse{
			YTChannels:     ytChannels,
			Username:       tokenInfo.Username,
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
func checkYoutube(svc clients.YoutubeClientInterface, filtered bool, username string) []YTChannel {
	const funcName = "checkYoutube"
	response := make([]YTChannel, 0)
	ctx := context.Background()

	if svc == nil {
		slog.Warn("uninitialized youtube service", logging.FuncNameAttr(funcName), logging.UserAttr(username))
		return nil
	}

	// get user's subscriptions list from the YouTube API
	err := svc.GetAndProcessSubscriptions(ctx, func(subs *youtube.SubscriptionListResponse) error {
		// collect channels having published new videos
		ch := make(chan YTChannel)
		var itemsCount int
		for _, item := range subs.Items {
			newItems := item.ContentDetails.NewItemCount
			if !filtered || newItems > 0 {
				itemsCount++
				go func(item *youtube.Subscription) {
					err := processYouTubeChannel(svc, item, ch, username)
					if err != nil {
						slog.Warn(fmt.Sprintf("failed to retrieve latest YouTube video from playlist, "+
							"skipping info for channel %s", item.Snippet.Title),
							logging.FuncNameAttr(funcName), logging.UserAttr(username))
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
		slog.Error(fmt.Sprintf("error retrieving YouTube subscriptions list: %s", err.Error()),
			logging.FuncNameAttr(funcName), logging.UserAttr(username))
		return response
	}

	if len(response) == 0 {
		slog.Info("no new video published by user's YouTube channels",
			logging.FuncNameAttr(funcName), logging.UserAttr(username))
	}

	// sort results by title
	slices.SortFunc(response, func(a, b YTChannel) int {
		return cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})

	return response
}

// check a subscription for new videos and add it to the list
func processYouTubeChannel(svc clients.YoutubeClientInterface, item *youtube.Subscription, ch chan<- YTChannel,
	username string) error {
	const funcName = "processYouTubeChannel"
	channelTitle := item.Snippet.Title
	channelID := item.Snippet.ResourceId.ChannelId
	responseItem := YTChannel{
		Title:     channelTitle,
		ChannelID: channelID,
		URL:       fmt.Sprintf("%s/channel/%s/videos", youTubeBasepath, channelID),
	}

	// the playlist ID can be obtained by changing the second letter of the channel ID
	playlistIDRunes := []rune(channelID)
	playlistIDRunes[1] = 'U'
	playlistID := string(playlistIDRunes)

	// get latest video info from the first playlist item
	playlistItem, err := svc.GetLatestVideoFromPlaylist(playlistID)
	if err != nil {
		slog.Error(fmt.Sprintf("error retrieving latest YouTube video from playlist: %s", err.Error()),
			logging.FuncNameAttr(funcName), logging.UserAttr(username))
		return err
	}
	if playlistItem != nil {
		responseItem.LatestVideoURL = fmt.Sprintf("%s/watch?v=%s", youTubeBasepath,
			playlistItem.Snippet.ResourceId.VideoId)
		responseItem.LatestVideoTitle = playlistItem.Snippet.Title
		slog.Debug(fmt.Sprintf("found latest video for channel %s", channelTitle),
			logging.FuncNameAttr(funcName), logging.UserAttr(username))
	}

	ch <- responseItem
	return nil
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

		// get token from context
		tokenInfo, tokenOk := r.Context().Value(auth.TokenCtxKey{}).(*auth.TokenInfo)
		if !tokenOk {
			slog.Warn(fmt.Sprintf("token not found in context, redirecting user to login page"),
				logging.FuncNameAttr(funcName))
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// create HTTP client
		client := oauth2C.CreateHTTPClient(r.Context(), tokenInfo.Token)
		if client == nil {
			slog.Warn("http client not initialized, redirecting user to login page",
				logging.FuncNameAttr(funcName))
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// visit the channel to mark it as viewed
		url := fmt.Sprintf("%s/channel/%s/videos", youTubeBasepath, req.ChannelID)
		res, err := client.Get(url)
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
