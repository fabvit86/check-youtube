package handlers

import (
	"cmp"
	"context"
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

var svc *youtube.Service
var peopleSvc *people.Service

type YTChannel struct {
	Title            string
	URL              string
	LatestVideoURL   string
	LatestVideoTitle string
}

type templateResponse struct {
	YTChannels []YTChannel
	Username   string
}

// InitYoutubeService create a YouTube service using the given oauth2 token
func InitYoutubeService(oauth2Config oauth2.Config, token *oauth2.Token) error {
	ctx := context.Background()

	// get an HTTP client from the token
	client := oauth2Config.Client(ctx, token)

	// create YouTube service
	var err error
	svc, err = youtube.NewService(
		ctx,
		option.WithHTTPClient(client),
	)
	if err != nil {
		log.Println(fmt.Sprintf("unable to create YouTube service: %v", err))
		return err
	}

	return nil
}

// InitPeopleService create a Google People service using the given oauth2 token
func InitPeopleService(oauth2Config oauth2.Config, token *oauth2.Token) error {
	ctx := context.Background()

	// get an HTTP client from the token
	client := oauth2Config.Client(ctx, token)

	// create YouTube service
	var err error
	peopleSvc, err = people.NewService(
		ctx,
		option.WithHTTPClient(client),
	)
	if err != nil {
		log.Println(fmt.Sprintf("unable to create People service: %v", err))
		return err
	}

	return nil
}

// GetYoutubeChannelsVideosNotification call YouTube API to check for new videos
func GetYoutubeChannelsVideosNotification(port, htmlTemplate string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc == nil || peopleSvc == nil {
			// redirect to login page
			log.Println("services not initialized, redirecting user to login page")
			http.Redirect(w, r, fmt.Sprintf("http://localhost:%s/login", port), http.StatusTemporaryRedirect)
			return
		}

		// get user info using the Google People API
		username := getLoggedUserinfo(peopleSvc)

		// get YouTube subscriptions info
		ytChannels := checkYoutube(svc)

		response := templateResponse{
			YTChannels: ytChannels,
			Username:   username,
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
func checkYoutube(svc *youtube.Service) []YTChannel {
	response := make([]YTChannel, 0)
	ctx := context.Background()

	if svc == nil {
		log.Println("warning: uninitialized youtube service")
		return nil
	}

	// get user's subscriptions list from the YouTube API
	err := svc.Subscriptions.
		List([]string{"contentDetails", "snippet"}).
		Order("unread").
		Mine(true).
		MaxResults(50).
		Pages(ctx, func(subs *youtube.SubscriptionListResponse) error {
			// collect channels having published new videos
			wg := &sync.WaitGroup{}
			var mutex sync.Mutex
			for _, item := range subs.Items {
				newItems := item.ContentDetails.NewItemCount
				if newItems > 0 {
					wg.Add(1)
					go processYouTubeChannel(item, &response, &mutex, wg)
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

// call Google People API to get logged user info
func getLoggedUserinfo(svc *people.Service) string {
	userinfo, err := svc.People.
		Get("people/me").
		PersonFields("names").
		Do()
	if err != nil {
		log.Println(fmt.Sprintf("error retrieving logged user info: %v", err))
		return ""
	}

	if len(userinfo.Names) > 0 {
		return userinfo.Names[0].DisplayName
	}

	return ""
}

// check a subscription for new videos and add it to the list
func processYouTubeChannel(item *youtube.Subscription, ytChannels *[]YTChannel, mutex *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	channelTitle := item.Snippet.Title
	channelID := item.Snippet.ResourceId.ChannelId
	responseItem := YTChannel{
		Title: channelTitle,
		URL:   fmt.Sprintf("https://www.youtube.com/channel/%s/videos", channelID),
	}

	// the playlist ID can be obtained by changing the second letter of the channel ID
	playlistIDRunes := []rune(channelID)
	playlistIDRunes[1] = 'U'
	playlistID := string(playlistIDRunes)

	// get latest video info from the first playlist item
	playlistItemsResponse, err := svc.PlaylistItems.
		List([]string{"snippet"}).
		PlaylistId(playlistID).
		MaxResults(1).
		Do()
	if err != nil {
		log.Println(fmt.Sprintf("error retrieving latest YouTube video for channel %s: %v", channelTitle, err))
	} else if len(playlistItemsResponse.Items) > 0 {
		playlistItemItem := playlistItemsResponse.Items[0]
		responseItem.LatestVideoURL = fmt.Sprintf("https://www.youtube.com/watch?v=%s", playlistItemItem.Snippet.ResourceId.VideoId)
		responseItem.LatestVideoTitle = playlistItemItem.Snippet.Title
	}

	mutex.Lock()
	*ytChannels = append(*ytChannels, responseItem)
	mutex.Unlock()
	log.Println(fmt.Sprintf("channel %s published new videos", channelTitle))
}
