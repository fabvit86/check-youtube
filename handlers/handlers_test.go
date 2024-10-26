package handlers

import (
	"bytes"
	"checkYoutube/testing_utils"
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
type peopleClientMock struct {
	getLoggedUserinfoStub func() string
}

func (y youtubeClientMock) getAndProcessSubscriptions(ctx context.Context,
	processFunction func(*youtube.SubscriptionListResponse) error) error {
	return y.getAndProcessSubscriptionsStub(ctx, processFunction)
}
func (y youtubeClientMock) getLatestVideoFromPlaylist(playlistID string) (*youtube.PlaylistItem, error) {
	return y.getLatestVideoFromPlaylistStub(playlistID)
}
func (p peopleClientMock) getLoggedUserinfo() string {
	return p.getLoggedUserinfoStub()
}

func TestGetYoutubeChannelsVideosNotification(t *testing.T) {
	const (
		serverBasepath = "http://localhost:8900"
		successCase    = "success case"
		redirectCase   = "redirect case"
	)

	// mocks
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/check-youtube", nil)
	if err != nil {
		t.Fatal(err)
	}
	clientSvc = clientService{
		peopleSvc: &peopleClientMock{
			getLoggedUserinfoStub: func() string {
				return "usertest"
			},
		},
	}

	type args struct {
		serverBasepath string
		htmlTemplate   string
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: successCase,
			args: args{
				serverBasepath: serverBasepath,
				htmlTemplate:   "",
			},
			want: http.StatusOK,
		},
		{
			name: redirectCase,
			args: args{
				serverBasepath: serverBasepath,
				htmlTemplate:   "",
			},
			want: http.StatusTemporaryRedirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerFunction := GetYoutubeChannelsVideosNotification(tt.args.serverBasepath, tt.args.htmlTemplate)
			switch tt.name {
			case successCase:
				clientSvc.youtubeSvc = &youtubeClientMock{
					getAndProcessSubscriptionsStub: func(ctx context.Context, f func(*youtube.SubscriptionListResponse) error) error {
						return nil
					},
				}
			case redirectCase:
				clientSvc.youtubeSvc = nil
			}
			handlerFunction(recorder, req)
			if recorder.Code != tt.want {
				t.Errorf("GetYoutubeChannelsVideosNotification() = %v, want %v", recorder.Code, tt.want)
			}
		})
	}
}

func TestInitServices(t *testing.T) {
	type args struct {
		tokenSource oauth2.TokenSource
		client      *http.Client
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success case",
			args: args{
				tokenSource: &testing_utils.TokenSourceMock{},
				client:      nil,
			},
			wantErr: false,
		},
		{
			name: "error case - nil tokenSource",
			args: args{
				tokenSource: nil,
				client:      nil,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := InitServices(tt.args.tokenSource, tt.args.client); (err != nil) != tt.wantErr {
				t.Errorf("InitServices() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMarkAsViewed(t *testing.T) {
	// mocks
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ko" {
			// simulate a not found
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	clientSvc = clientService{
		client: server.Client(),
	}

	createMockRequest := func(path string) *http.Request {
		reqBody := callUrlRequest{
			URL: server.URL + path,
		}

		reqBytes, err := json.Marshal(reqBody)
		if err != nil {
			t.Fatal(err)
		}

		req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer(reqBytes))
		if err != nil {
			t.Fatal(err)
		}

		return req
	}

	const (
		successCase               = "success case"
		failureCaseBadStatus      = "failure case - bad status code"
		failureCaseMissingReqBody = "failure case - empty request body"
		failureCaseBadRequest     = "failure case - bad request"
		redirectToLoginPage       = "redirect to login"
	)
	type args struct {
		serverBasepath string
		recorder       *httptest.ResponseRecorder
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: successCase,
			args: args{
				serverBasepath: "http://localhost:8900",
				recorder:       httptest.NewRecorder(),
			},
		},
		{
			name: failureCaseBadStatus,
			args: args{
				serverBasepath: "http://localhost:8900",
				recorder:       httptest.NewRecorder(),
			},
		},
		{
			name: failureCaseMissingReqBody,
			args: args{
				serverBasepath: "http://localhost:8900",
				recorder:       httptest.NewRecorder(),
			},
		},
		{
			name: failureCaseBadRequest,
			args: args{
				serverBasepath: "http://localhost:8900",
				recorder:       httptest.NewRecorder(),
			},
		},
		{
			name: redirectToLoginPage,
			args: args{
				serverBasepath: "http://localhost:8900",
				recorder:       httptest.NewRecorder(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerFunction := MarkAsViewed(tt.args.serverBasepath)
			switch tt.name {
			case successCase:
				handlerFunction(tt.args.recorder, createMockRequest("/ok"))
				if tt.args.recorder.Code != http.StatusOK {
					t.Errorf("MarkAsViewed() = %v, want %v", tt.args.recorder.Code, http.StatusOK)
				}
			case failureCaseBadStatus:
				handlerFunction(tt.args.recorder, createMockRequest("/ko"))
				if tt.args.recorder.Code != http.StatusInternalServerError {
					t.Errorf("MarkAsViewed() = %v, want %v", tt.args.recorder.Code, http.StatusInternalServerError)
				}
			case failureCaseMissingReqBody:
				reqEmptyBody, err := http.NewRequest(http.MethodPost, "/", nil)
				if err != nil {
					t.Fatal(err)
				}
				handlerFunction(tt.args.recorder, reqEmptyBody)
				if tt.args.recorder.Code != http.StatusBadRequest {
					t.Errorf("MarkAsViewed() = %v, want %v", tt.args.recorder.Code, http.StatusBadRequest)
				}
			case failureCaseBadRequest:
				req, err := http.NewRequest(http.MethodPost, "/", bytes.NewBuffer([]byte(`{invalid_json}`)))
				if err != nil {
					t.Fatal(err)
				}
				handlerFunction(tt.args.recorder, req)
				if tt.args.recorder.Code != http.StatusBadRequest {
					t.Errorf("MarkAsViewed() = %v, want %v", tt.args.recorder.Code, http.StatusBadRequest)
				}
			case redirectToLoginPage:
				clientSvc.client = nil
				handlerFunction(tt.args.recorder, createMockRequest("/ok"))
				if tt.args.recorder.Code != http.StatusTemporaryRedirect {
					t.Errorf("MarkAsViewed() = %v, want %v", tt.args.recorder.Code, http.StatusTemporaryRedirect)
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
		svc      YoutubeClientInterface
		filtered bool
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
					URL:              fmt.Sprintf(channelUrl, subsInput[0].Snippet.ResourceId.ChannelId),
					LatestVideoURL:   fmt.Sprintf(videoUrl, playlistItemOuput.Snippet.ResourceId.VideoId),
					LatestVideoTitle: playlistItemOuput.Snippet.Title,
				},
				{
					Title:            subsInput[1].Snippet.Title,
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
					URL:              fmt.Sprintf(channelUrl, subsInput[0].Snippet.ResourceId.ChannelId),
					LatestVideoURL:   fmt.Sprintf(videoUrl, playlistItemOuput.Snippet.ResourceId.VideoId),
					LatestVideoTitle: playlistItemOuput.Snippet.Title,
				},
				{
					Title:            subsInput[1].Snippet.Title,
					URL:              fmt.Sprintf(channelUrl, subsInput[1].Snippet.ResourceId.ChannelId),
					LatestVideoURL:   fmt.Sprintf(videoUrl, playlistItemOuput.Snippet.ResourceId.VideoId),
					LatestVideoTitle: playlistItemOuput.Snippet.Title,
				},
				{
					Title:            subsInput[2].Snippet.Title,
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
					Title: subsInput[0].Snippet.Title,
					URL:   fmt.Sprintf(channelUrl, subsInput[0].Snippet.ResourceId.ChannelId),
				},
				{
					Title: subsInput[1].Snippet.Title,
					URL:   fmt.Sprintf(channelUrl, subsInput[1].Snippet.ResourceId.ChannelId),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkYoutube(tt.args.svc, tt.args.filtered)
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
		svc  YoutubeClientInterface
		item *youtube.Subscription
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
				Title: item.Snippet.Title,
				URL:   fmt.Sprintf("https://www.youtube.com/channel/%s/videos", item.Snippet.ResourceId.ChannelId),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processYouTubeChannel(tt.args.svc, tt.args.item)
			if (err != nil) != tt.wantErr {
				t.Errorf("processYouTubeChannel() error = %v, wantErr %v", err, tt.wantErr)
			}
			if diff := cmp.Diff(got, tt.want); diff != "" {
				t.Errorf("processYouTubeChannel() - diff: \n%v", diff)
			}
		})
	}
}
