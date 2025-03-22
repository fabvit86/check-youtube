package handlers

import (
	"checkYoutube/auth"
	"checkYoutube/clients"
	"checkYoutube/duration"
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
	"sync"
)

type YTChannel struct {
	Title                  string
	ChannelID              string
	URL                    string
	LatestVideoID          string
	LatestVideoURL         string
	LatestVideoTitle       string
	LatestVideoPublishedAt string
	LatestVideoDuration    string
}

type templateResponse struct {
	YTChannels     []YTChannel
	Username       string
	ServerBasepath string
}

type callUrlRequest struct {
	ChannelsID []string `json:"channels_id"`
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
	videoIDs := make([]string, 0)
	ctx := context.Background()

	if svc == nil {
		slog.Warn("uninitialized youtube service", logging.FuncNameAttr(funcName), logging.UserAttr(username))
		return nil
	}

	// get user's subscriptions list from the YouTube API
	err := svc.GetAndProcessSubscriptions(ctx, func(subs *youtube.SubscriptionListResponse) error {
		// collect channels having published new videos
		wg := &sync.WaitGroup{}
		mutex := sync.RWMutex{}
		for _, item := range subs.Items {
			newItems := item.ContentDetails.NewItemCount
			if !filtered || newItems > 0 {
				wg.Add(1)
				go func(item *youtube.Subscription) {
					defer wg.Done()
					responseItem, err := processYouTubeChannel(svc, item, username)
					if err != nil {
						slog.Warn(fmt.Sprintf("failed to retrieve latest YouTube video from playlist, "+
							"skipping info for channel %s", responseItem.Title),
							logging.FuncNameAttr(funcName), logging.UserAttr(username))
					}
					mutex.Lock()
					response = append(response, responseItem)
					videoIDs = append(videoIDs, responseItem.LatestVideoID)
					mutex.Unlock()
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
			logging.FuncNameAttr(funcName), logging.UserAttr(username))
		return response
	}

	if len(response) == 0 {
		slog.Info("no new video published by user's YouTube channels",
			logging.FuncNameAttr(funcName), logging.UserAttr(username))
		return response
	}

	// retrieve additional videos info from videos API
	err = svc.GetVideos(ctx, videoIDs, func(videos *youtube.VideoListResponse) error {
		// add video duration to each response item
		for _, item := range videos.Items {
			dur := item.ContentDetails.Duration
			if dur != "" {
				for i, ytChannel := range response {
					if ytChannel.LatestVideoID == item.Id {
						formattedDur, err := duration.FormatISO8601Duration(dur, username)
						if err != nil {
							slog.Warn(fmt.Sprintf("error formatting video duration: %s", err.Error()),
								logging.FuncNameAttr(funcName), logging.UserAttr(username))
							continue
						}
						response[i].LatestVideoDuration = formattedDur
						break
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		slog.Error(fmt.Sprintf("error retrieving videos: %s",
			err.Error()), logging.FuncNameAttr(funcName), logging.UserAttr(username))
		return response
	}

	// sort results by title
	slices.SortFunc(response, func(a, b YTChannel) int {
		return cmp.Compare(strings.ToLower(a.Title), strings.ToLower(b.Title))
	})

	return response
}

// check a subscription for new videos and add it to the list
func processYouTubeChannel(svc clients.YoutubeClientInterface, item *youtube.Subscription,
	username string) (YTChannel, error) {
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
		return responseItem, err
	}
	if playlistItem != nil {
		responseItem.LatestVideoURL = fmt.Sprintf("%s/watch?v=%s", youTubeBasepath,
			playlistItem.Snippet.ResourceId.VideoId)
		responseItem.LatestVideoTitle = playlistItem.Snippet.Title
		responseItem.LatestVideoPublishedAt = playlistItem.Snippet.PublishedAt
		responseItem.LatestVideoID = playlistItem.Snippet.ResourceId.VideoId
		slog.Debug(fmt.Sprintf("found latest video for channel %s", channelTitle),
			logging.FuncNameAttr(funcName), logging.UserAttr(username))
	}

	return responseItem, nil
}

// MarkAsViewed visits subscription channels in the background to clear the notification of new videos
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

		if len(req.ChannelsID) == 0 {
			slog.Info("no channels ID found in request body", logging.FuncNameAttr(funcName))
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
				logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
			http.Redirect(w, r, fmt.Sprintf("%s/login", serverBasepath), http.StatusTemporaryRedirect)
			return
		}

		// limit max concurrent goroutines that will perform the http call
		maxConcurrentCalls := 5

		// enqueue URLs to visit in a buffered channel
		ch := make(chan string, len(req.ChannelsID))
		errorsCh := make(chan error)
		wg := &sync.WaitGroup{}
		for _, channelID := range req.ChannelsID {
			url := fmt.Sprintf("%s/channel/%s/videos", youTubeBasepath, channelID)
			wg.Add(1)
			ch <- url
		}

		// visit each url of the YouTube channels to mark them as viewed
		for i := 0; i < maxConcurrentCalls; i++ {
			go func() {
				for url := range ch {
					func() {
						defer wg.Done()
						res, err := client.Get(url)
						if err != nil {
							slog.Error(fmt.Sprintf("markAsViewed get request failed, error: %s", err.Error()),
								logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
							errorsCh <- err
							return
						}

						if res.StatusCode != http.StatusOK {
							err = fmt.Errorf("call to youtube returned status: %d", res.StatusCode)
							slog.Error(err.Error(), logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
							errorsCh <- err
							return
						}

						slog.Debug(fmt.Sprintf("visited url: %s", url),
							logging.FuncNameAttr(funcName), logging.UserAttr(tokenInfo.Username))
					}()
				}
			}()
		}

		close(ch)

		// close errors channel when all goroutines are done
		go func() {
			wg.Wait()
			close(errorsCh)
		}()

		// check for errors in the error channel, return the first error encountered
		for err := range errorsCh {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
