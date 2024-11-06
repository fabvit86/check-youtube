package handlers

import (
	"checkYoutube/auth"
	"checkYoutube/logging"
	sessionsutils "checkYoutube/utils/sessions"
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
	"google.golang.org/api/youtube/v3"
	"log/slog"
	"net/http"
)

type PeopleClientInterface interface {
	getLoggedUserinfo() string
}

type PeopleClientFactoryInterface interface {
	NewClient(oauth2C auth.Oauth2Config, r *http.Request) (PeopleClientInterface, error)
}

type peopleClient struct {
	svc people.Service
}

type PeopleClientFactory struct{}

// NewClient creates a new people service client
func (p *PeopleClientFactory) NewClient(oauth2C auth.Oauth2Config, r *http.Request) (PeopleClientInterface, error) {
	const funcName = "NewClient"

	// get token from context
	token, tokenOk := r.Context().Value(sessionsutils.TokenCtxKey{}).(*oauth2.Token)
	if !tokenOk {
		err := fmt.Errorf("token not found in context")
		slog.Error(err.Error(), logging.FuncNameAttr(funcName))
		return nil, err
	}

	// create token source
	ts := oauth2C.CreateTokenSource(r.Context(), token)

	// create service
	peopleSvc, err := people.NewService(context.Background(), option.WithTokenSource(ts))
	if err != nil {
		slog.Error(fmt.Sprintf("unable to create people service: %s", err.Error()),
			logging.FuncNameAttr(funcName))
		return nil, err
	}

	return &peopleClient{
		svc: *peopleSvc,
	}, nil
}

func (p *peopleClient) getLoggedUserinfo() string {
	const funcName = "getLoggedUserinfo"

	userinfo, err := p.svc.People.
		Get("people/me").
		PersonFields("names").
		Do()
	if err != nil {
		slog.Error(fmt.Sprintf("error retrieving logged user info: %s", err.Error()),
			logging.FuncNameAttr(funcName))
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

type YoutubeClientFactoryInterface interface {
	NewClient(oauth2C auth.Oauth2Config, r *http.Request) (YoutubeClientInterface, error)
}

type youtubeClient struct {
	svc youtube.Service
}

type YoutubeClientFactory struct{}

// NewClient creates a new youtube service client
func (p *YoutubeClientFactory) NewClient(oauth2C auth.Oauth2Config, r *http.Request) (YoutubeClientInterface, error) {
	const funcName = "NewClient"

	// get token from context
	token, tokenOk := r.Context().Value(sessionsutils.TokenCtxKey{}).(*oauth2.Token)
	if !tokenOk {
		err := fmt.Errorf("token not found in context")
		slog.Error(err.Error(), logging.FuncNameAttr(funcName))
		return nil, err
	}

	// create token source
	ts := oauth2C.CreateTokenSource(r.Context(), token)

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

func (y *youtubeClient) getAndProcessSubscriptions(ctx context.Context,
	processFunction func(*youtube.SubscriptionListResponse) error) error {
	const funcName = "getAndProcessSubscriptions"

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

func (y *youtubeClient) getLatestVideoFromPlaylist(playlistID string) (*youtube.PlaylistItem, error) {
	const funcName = "getLatestVideoFromPlaylist"

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
