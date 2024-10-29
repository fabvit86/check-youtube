package handlers

import (
	"context"
	"fmt"
	"google.golang.org/api/people/v1"
	"google.golang.org/api/youtube/v3"
	"log/slog"
)

type PeopleClientInterface interface {
	getLoggedUserinfo() string
}

type peopleClient struct {
	svc people.Service
}

func (p *peopleClient) getLoggedUserinfo() string {
	userinfo, err := p.svc.People.
		Get("people/me").
		PersonFields("names").
		Do()
	if err != nil {
		slog.Error(fmt.Sprintf("error retrieving logged user info: %s", err.Error()))
		return ""
	}

	if len(userinfo.Names) > 0 {
		return userinfo.Names[0].DisplayName
	}

	return ""
}

type YoutubeClientInterface interface {
	getAndProcessSubscriptions(ctx context.Context,
		processFunction func(*youtube.SubscriptionListResponse) error) error
	getLatestVideoFromPlaylist(playlistID string) (*youtube.PlaylistItem, error)
}

type youtubeClient struct {
	svc youtube.Service
}

func (y *youtubeClient) getAndProcessSubscriptions(ctx context.Context,
	processFunction func(*youtube.SubscriptionListResponse) error) error {
	err := y.svc.Subscriptions.
		List([]string{"contentDetails", "snippet"}).
		Order("unread").
		Mine(true).
		MaxResults(50).
		Pages(ctx, processFunction)
	if err != nil {
		slog.Error(fmt.Sprintf("error retrieving YouTube subscriptions list: %s", err.Error()))
		return err
	}

	return nil
}

func (y *youtubeClient) getLatestVideoFromPlaylist(playlistID string) (*youtube.PlaylistItem, error) {
	playlistItemsResponse, err := y.svc.PlaylistItems.
		List([]string{"snippet"}).
		PlaylistId(playlistID).
		MaxResults(1).
		Do()
	if err != nil {
		slog.Error(fmt.Sprintf("error retrieving latest YouTube video from playlist %s: %s", playlistID, err.Error()))
		return nil, err
	}

	if len(playlistItemsResponse.Items) > 0 {
		return playlistItemsResponse.Items[0], nil
	}

	slog.Debug(fmt.Sprintf("no video found in playlist %s", playlistID))
	return nil, nil
}
