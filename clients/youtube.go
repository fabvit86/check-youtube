package clients

import (
	"checkYoutube/logging"
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"log/slog"
)

type YoutubeClientInterface interface {
	GetAndProcessSubscriptions(ctx context.Context,
		processFunction func(*youtube.SubscriptionListResponse) error) error
	GetLatestVideoFromPlaylist(playlistID string) (*youtube.PlaylistItem, error)
}

type YoutubeClientFactoryInterface interface {
	NewClient(oauth2.TokenSource) (YoutubeClientInterface, error)
}

type youtubeClient struct {
	svc youtube.Service
}

type YoutubeClientFactory struct{}

// NewClient creates a new youtube service client using the given token source
func (p *YoutubeClientFactory) NewClient(ts oauth2.TokenSource) (YoutubeClientInterface, error) {
	const funcName = "NewClient"

	// create service
	youtubeSvc, err := youtube.NewService(context.Background(), option.WithTokenSource(ts))
	if err != nil {
		slog.Error(fmt.Sprintf("unable to create youtube service: %s", err.Error()),
			logging.FuncNameAttr(funcName))
		return nil, err
	}

	return &youtubeClient{
		svc: *youtubeSvc,
	}, nil
}

func (y *youtubeClient) GetAndProcessSubscriptions(ctx context.Context,
	processFunction func(*youtube.SubscriptionListResponse) error) error {
	const funcName = "GetAndProcessSubscriptions"

	err := y.svc.Subscriptions.
		List([]string{"contentDetails", "snippet"}).
		Order("unread").
		Mine(true).
		MaxResults(50).
		Pages(ctx, processFunction)
	if err != nil {
		slog.Error(fmt.Sprintf("error retrieving YouTube subscriptions list: %s", err.Error()),
			logging.FuncNameAttr(funcName))
		return err
	}

	return nil
}

func (y *youtubeClient) GetLatestVideoFromPlaylist(playlistID string) (*youtube.PlaylistItem, error) {
	const funcName = "GetLatestVideoFromPlaylist"

	playlistItemsResponse, err := y.svc.PlaylistItems.
		List([]string{"snippet"}).
		PlaylistId(playlistID).
		MaxResults(1).
		Do()
	if err != nil {
		slog.Error(fmt.Sprintf("error retrieving latest YouTube video from playlist %s: %s",
			playlistID, err.Error()), logging.FuncNameAttr(funcName))
		return nil, err
	}

	if len(playlistItemsResponse.Items) > 0 {
		return playlistItemsResponse.Items[0], nil
	}

	slog.Debug(fmt.Sprintf("no video found in playlist %s", playlistID), logging.FuncNameAttr(funcName))
	return nil, nil
}
