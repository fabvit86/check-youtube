package handlers

import (
	"bytes"
	"checkYoutube/auth"
	"checkYoutube/clients"
	"checkYoutube/test"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/oauth2"
	"google.golang.org/api/youtube/v3"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mocks
type youtubeClientMock struct {
	getAndProcessSubscriptionsStub func(context.Context, func(*youtube.SubscriptionListResponse) error) error
	getLatestVideoFromPlaylistStub func(string) (*youtube.PlaylistItem, error)
}
type youtubeClientFactoryMock struct {
	newClientStub func(oauth2.TokenSource) (clients.YoutubeClientInterface, error)
}

func (y youtubeClientMock) GetAndProcessSubscriptions(ctx context.Context,
	processFunction func(*youtube.SubscriptionListResponse) error) error {
	return y.getAndProcessSubscriptionsStub(ctx, processFunction)
}
func (y youtubeClientMock) GetLatestVideoFromPlaylist(playlistID string) (*youtube.PlaylistItem, error) {
	return y.getLatestVideoFromPlaylistStub(playlistID)
}
func (yf *youtubeClientFactoryMock) NewClient(ts oauth2.TokenSource) (clients.YoutubeClientInterface, error) {
	return yf.newClientStub(ts)
}

func TestGetYoutubeChannelsVideos(t *testing.T) {
	// mocks
	const serverBasepath = "http://localhost:8900"
	const tokenNotFound = "redirect case - token not found in context"
	oauth2C := auth.Oauth2Config{Oauth2ConfigProvider: &test.Oauth2Mock{}}
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/check-youtube", nil)
	if err != nil {
		t.Fatal(err)
	}
	ytcf := &youtubeClientFactoryMock{
		newClientStub: func(ts oauth2.TokenSource) (clients.YoutubeClientInterface, error) {
			return &youtubeClientMock{
				getAndProcessSubscriptionsStub: func(ctx context.Context,
					f func(*youtube.SubscriptionListResponse) error) error {
					return nil
				},
			}, nil
		},
	}

	type args struct {
		oauth2C        auth.Oauth2Config
		ytcf           clients.YoutubeClientFactoryInterface
		serverBasepath string
		htmlTemplate   string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "success case",
			args: args{
				oauth2C:        oauth2C,
				ytcf:           ytcf,
				serverBasepath: serverBasepath,
				htmlTemplate:   "",
			},
			want: http.StatusOK,
		},
		{
			name: "redirect case - error on creating youtube client",
			args: args{
				oauth2C: oauth2C,
				ytcf: &youtubeClientFactoryMock{
					newClientStub: func(ts oauth2.TokenSource) (clients.YoutubeClientInterface, error) {
						return nil, fmt.Errorf("testerror")
					},
				},
				serverBasepath: serverBasepath,
				htmlTemplate:   "",
			},
			want: http.StatusTemporaryRedirect,
		},
		{
			name: tokenNotFound,
			args: args{
				oauth2C:        oauth2C,
				ytcf:           ytcf,
				serverBasepath: serverBasepath,
				htmlTemplate:   "",
			},
			want: http.StatusTemporaryRedirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == tokenNotFound {
				req = req.WithContext(context.Background())
			} else {
				req = req.WithContext(addTokenInfoToContext(req.Context(), &auth.TokenInfo{Token: &oauth2.Token{}}))
			}
			handlerFunction := GetYoutubeChannelsVideos(tt.args.oauth2C, tt.args.ytcf,
				tt.args.serverBasepath, tt.args.htmlTemplate)
			handlerFunction(recorder, req)
			if recorder.Code != tt.want {
				t.Errorf("GetYoutubeChannelsVideos() = %v, want %v", recorder.Code, tt.want)
			}
		})
	}
}

func TestMarkAsViewed(t *testing.T) {
	// mocks
	oauth2C := auth.Oauth2Config{Oauth2ConfigProvider: &test.Oauth2Mock{}}
	createMockRequest := func(channelID string) *http.Request {
		reqBody := callUrlRequest{
			ChannelID: channelID,
		}

		reqBytes, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatal(err)
		}

		req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(reqBytes))
		if err != nil {
			t.Fatal(err)
		}

		ctx := req.Context()
		ctx = context.WithValue(ctx, auth.TokenCtxKey{}, &oauth2.Token{})
		req = req.WithContext(ctx)

		return req
	}

	const (
		failureCaseBadStatus      = "failure case - bad status code"
		failureCaseMissingReqBody = "failure case - empty request body"
		failureCaseBadRequest     = "failure case - bad request"
		tokenNotFound             = "failure case - token not found in context"
	)
	type args struct {
		oauth2C        auth.Oauth2Config
		serverBasepath string
		recorder       *httptest.ResponseRecorder
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: failureCaseBadStatus,
			args: args{
				oauth2C:        oauth2C,
				serverBasepath: "http://localhost:8900",
				recorder:       httptest.NewRecorder(),
			},
			want: http.StatusInternalServerError,
		},
		{
			name: failureCaseMissingReqBody,
			args: args{
				oauth2C:        oauth2C,
				serverBasepath: "http://localhost:8900",
				recorder:       httptest.NewRecorder(),
			},
			want: http.StatusBadRequest,
		},
		{
			name: failureCaseBadRequest,
			args: args{
				oauth2C:        oauth2C,
				serverBasepath: "http://localhost:8900",
				recorder:       httptest.NewRecorder(),
			},
			want: http.StatusBadRequest,
		},
		{
			name: tokenNotFound,
			args: args{
				oauth2C:        oauth2C,
				serverBasepath: "http://localhost:8900",
				recorder:       httptest.NewRecorder(),
			},
			want: http.StatusTemporaryRedirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerFunction := MarkAsViewed(tt.args.oauth2C, tt.args.serverBasepath)
			switch tt.name {
			case failureCaseBadStatus:
				req := createMockRequest("channelIdTest")
				req = req.WithContext(addTokenInfoToContext(req.Context(), &auth.TokenInfo{Token: &oauth2.Token{}}))
				handlerFunction(tt.args.recorder, req)
				if tt.args.recorder.Code != tt.want {
					t.Errorf("MarkAsViewed() = %v, want %v", tt.args.recorder.Code, tt.want)
				}
			case failureCaseMissingReqBody:
				reqEmptyBody, err := http.NewRequest(http.MethodPost, "/", nil)
				if err != nil {
					t.Fatal(err)
				}
				handlerFunction(tt.args.recorder, reqEmptyBody)
				if tt.args.recorder.Code != tt.want {
					t.Errorf("MarkAsViewed() = %v, want %v", tt.args.recorder.Code, tt.want)
				}
			case failureCaseBadRequest:
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer([]byte(`{invalid_json}`)))
				req = req.WithContext(addTokenInfoToContext(req.Context(), &auth.TokenInfo{Token: &oauth2.Token{}}))
				if err != nil {
					t.Fatal(err)
				}
				handlerFunction(tt.args.recorder, req)
				if tt.args.recorder.Code != tt.want {
					t.Errorf("MarkAsViewed() = %v, want %v", tt.args.recorder.Code, tt.want)
				}
			case tokenNotFound:
				req := createMockRequest("channelIdTest")
				req = req.WithContext(context.Background())
				handlerFunction(tt.args.recorder, req)
				if tt.args.recorder.Code != tt.want {
					t.Errorf("MarkAsViewed() = %v, want %v", tt.args.recorder.Code, tt.want)
				}
			}
		})
	}
}

func Test_checkYoutube(t *testing.T) {
	const (
		channelUrl = "https://www.youtube.com/channel/%s/videos"
		videoUrl   = "https://www.youtube.com/watch?v=%s"
	)
	subsInput := []*youtube.Subscription{
		{
			ContentDetails: &youtube.SubscriptionContentDetails{
				NewItemCount: 1,
			},
			Snippet: &youtube.SubscriptionSnippet{
				ResourceId: &youtube.ResourceId{
					ChannelId: "channelidtest-1",
				},
				Title: "channeltest-1",
			},
		},
		{
			ContentDetails: &youtube.SubscriptionContentDetails{
				NewItemCount: 3,
			},
			Snippet: &youtube.SubscriptionSnippet{
				ResourceId: &youtube.ResourceId{
					ChannelId: "channelidtest-2",
				},
				Title: "channeltest-2",
			},
		},
		{
			ContentDetails: &youtube.SubscriptionContentDetails{
				NewItemCount: 0,
			},
			Snippet: &youtube.SubscriptionSnippet{
				ResourceId: &youtube.ResourceId{
					ChannelId: "channelidtest-3",
				},
				Title: "channeltest-3",
			},
		},
	}
	playlistItemOuput := &youtube.PlaylistItem{
		Snippet: &youtube.PlaylistItemSnippet{
			Title: "videotitletest",
			ResourceId: &youtube.ResourceId{
				VideoId: "videoidtest",
			},
		},
	}

	type args struct {
		svc      clients.YoutubeClientInterface
		filtered bool
		username string
	}
	tests := []struct {
		name string
		args args
		want []YTChannel
	}{
		{
			name: "success case - filtered",
			args: args{
				svc: &youtubeClientMock{
					getAndProcessSubscriptionsStub: func(ctx context.Context,
						processFunction func(*youtube.SubscriptionListResponse) error) error {
						_ = processFunction(&youtube.SubscriptionListResponse{
							Items: subsInput,
						})
						return nil
					},
					getLatestVideoFromPlaylistStub: func(string) (*youtube.PlaylistItem, error) {
						return playlistItemOuput, nil
					},
				},
				filtered: true,
			},
			want: []YTChannel{
				{
					Title:            subsInput[0].Snippet.Title,
					ChannelID:        subsInput[0].Snippet.ResourceId.ChannelId,
					URL:              fmt.Sprintf(channelUrl, subsInput[0].Snippet.ResourceId.ChannelId),
					LatestVideoURL:   fmt.Sprintf(videoUrl, playlistItemOuput.Snippet.ResourceId.VideoId),
					LatestVideoTitle: playlistItemOuput.Snippet.Title,
				},
				{
					Title:            subsInput[1].Snippet.Title,
					ChannelID:        subsInput[1].Snippet.ResourceId.ChannelId,
					URL:              fmt.Sprintf(channelUrl, subsInput[1].Snippet.ResourceId.ChannelId),
					LatestVideoURL:   fmt.Sprintf(videoUrl, playlistItemOuput.Snippet.ResourceId.VideoId),
					LatestVideoTitle: playlistItemOuput.Snippet.Title,
				},
			},
		},
		{
			name: "success case - all",
			args: args{
				svc: &youtubeClientMock{
					getAndProcessSubscriptionsStub: func(ctx context.Context,
						processFunction func(*youtube.SubscriptionListResponse) error) error {
						_ = processFunction(&youtube.SubscriptionListResponse{
							Items: subsInput,
						})
						return nil
					},
					getLatestVideoFromPlaylistStub: func(string) (*youtube.PlaylistItem, error) {
						return playlistItemOuput, nil
					},
				},
				filtered: false,
			},
			want: []YTChannel{
				{
					Title:            subsInput[0].Snippet.Title,
					ChannelID:        subsInput[0].Snippet.ResourceId.ChannelId,
					URL:              fmt.Sprintf(channelUrl, subsInput[0].Snippet.ResourceId.ChannelId),
					LatestVideoURL:   fmt.Sprintf(videoUrl, playlistItemOuput.Snippet.ResourceId.VideoId),
					LatestVideoTitle: playlistItemOuput.Snippet.Title,
				},
				{
					Title:            subsInput[1].Snippet.Title,
					ChannelID:        subsInput[1].Snippet.ResourceId.ChannelId,
					URL:              fmt.Sprintf(channelUrl, subsInput[1].Snippet.ResourceId.ChannelId),
					LatestVideoURL:   fmt.Sprintf(videoUrl, playlistItemOuput.Snippet.ResourceId.VideoId),
					LatestVideoTitle: playlistItemOuput.Snippet.Title,
				},
				{
					Title:            subsInput[2].Snippet.Title,
					ChannelID:        subsInput[2].Snippet.ResourceId.ChannelId,
					URL:              fmt.Sprintf(channelUrl, subsInput[2].Snippet.ResourceId.ChannelId),
					LatestVideoURL:   fmt.Sprintf(videoUrl, playlistItemOuput.Snippet.ResourceId.VideoId),
					LatestVideoTitle: playlistItemOuput.Snippet.Title,
				},
			},
		},
		{
			name: "success case - no new videos",
			args: args{
				svc: &youtubeClientMock{
					getAndProcessSubscriptionsStub: func(ctx context.Context,
						processFunction func(*youtube.SubscriptionListResponse) error) error {
						_ = processFunction(&youtube.SubscriptionListResponse{
							Items: []*youtube.Subscription{subsInput[2]},
						})
						return nil
					},
				},
				filtered: true,
			},
			want: make([]YTChannel, 0),
		},
		{
			name: "nil svc case",
			args: args{},
			want: nil,
		},
		{
			name: "failure case - getAndProcessSubscriptions error",
			args: args{
				svc: &youtubeClientMock{
					getAndProcessSubscriptionsStub: func(ctx context.Context,
						processFunction func(*youtube.SubscriptionListResponse) error) error {
						return fmt.Errorf("test error")
					},
				},
				filtered: true,
			},
			want: make([]YTChannel, 0),
		},
		{
			name: "failure case - processYouTubeChannel error",
			args: args{
				svc: &youtubeClientMock{
					getAndProcessSubscriptionsStub: func(ctx context.Context,
						processFunction func(*youtube.SubscriptionListResponse) error) error {
						_ = processFunction(&youtube.SubscriptionListResponse{
							Items: subsInput,
						})
						return nil
					},
					getLatestVideoFromPlaylistStub: func(string) (*youtube.PlaylistItem, error) {
						return nil, fmt.Errorf("test error")
					},
				},
				filtered: true,
			},
			want: []YTChannel{
				{
					Title:     subsInput[0].Snippet.Title,
					ChannelID: subsInput[0].Snippet.ResourceId.ChannelId,
					URL:       fmt.Sprintf(channelUrl, subsInput[0].Snippet.ResourceId.ChannelId),
				},
				{
					Title:     subsInput[1].Snippet.Title,
					ChannelID: subsInput[1].Snippet.ResourceId.ChannelId,
					URL:       fmt.Sprintf(channelUrl, subsInput[1].Snippet.ResourceId.ChannelId),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkYoutube(tt.args.svc, tt.args.filtered, tt.args.username)
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("checkYoutube() - diff: \n%v", diff)
			}
		})
	}
}

func Test_processYouTubeChannel(t *testing.T) {
	const videoID = "videoidtest"
	item := &youtube.Subscription{
		Snippet: &youtube.SubscriptionSnippet{
			ResourceId: &youtube.ResourceId{
				ChannelId: "channelidtest",
			},
			Title: videoID,
		},
	}

	type args struct {
		svc      clients.YoutubeClientInterface
		item     *youtube.Subscription
		username string
	}
	tests := []struct {
		name    string
		args    args
		want    YTChannel
		wantErr bool
	}{
		{
			name: "success case",
			args: args{
				svc: &youtubeClientMock{
					getLatestVideoFromPlaylistStub: func(string) (*youtube.PlaylistItem, error) {
						return &youtube.PlaylistItem{
							Snippet: &youtube.PlaylistItemSnippet{
								Title: "titletest",
								ResourceId: &youtube.ResourceId{
									VideoId: videoID,
								},
							},
						}, nil
					},
				},
				item: item,
			},
			want: YTChannel{
				Title:            item.Snippet.Title,
				ChannelID:        item.Snippet.ResourceId.ChannelId,
				URL:              fmt.Sprintf("https://www.youtube.com/channel/%s/videos", item.Snippet.ResourceId.ChannelId),
				LatestVideoURL:   fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID),
				LatestVideoTitle: "titletest",
			},
			wantErr: false,
		},
		{
			name: "error case",
			args: args{
				svc: &youtubeClientMock{
					getLatestVideoFromPlaylistStub: func(string) (*youtube.PlaylistItem, error) {
						return nil, fmt.Errorf("test error")
					},
				},
				item: item,
			},
			want: YTChannel{
				Title:     item.Snippet.Title,
				ChannelID: item.Snippet.ResourceId.ChannelId,
				URL:       fmt.Sprintf("https://www.youtube.com/channel/%s/videos", item.Snippet.ResourceId.ChannelId),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processYouTubeChannel(tt.args.svc, tt.args.item, tt.args.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("processYouTubeChannel() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("processYouTubeChannel() - diff: \n%v", diff)
			}
		})
	}
}

func addTokenInfoToContext(ctx context.Context, value *auth.TokenInfo) context.Context {
	return context.WithValue(ctx, auth.TokenCtxKey{}, value)
}
