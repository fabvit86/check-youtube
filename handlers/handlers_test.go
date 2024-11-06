package handlers

import (
	"bytes"
	"checkYoutube/auth"
	"checkYoutube/testing_utils"
	"checkYoutube/utils/sessions"
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
type youtubeClientFactoryMock struct {
	newClientStub func(auth.Oauth2Config, *http.Request) (YoutubeClientInterface, error)
}
type peopleClientFactoryMock struct {
	newClientStub func(auth.Oauth2Config, *http.Request) (PeopleClientInterface, error)
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
func (yf *youtubeClientFactoryMock) NewClient(config auth.Oauth2Config, req *http.Request) (YoutubeClientInterface, error) {
	return yf.newClientStub(config, req)
}
func (pf *peopleClientFactoryMock) NewClient(config auth.Oauth2Config, req *http.Request) (PeopleClientInterface, error) {
	return pf.newClientStub(config, req)
}

func TestGetYoutubeChannelsVideos(t *testing.T) {
	// mocks
	const serverBasepath = "http://localhost:8900"
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest(http.MethodGet, "/check-youtube", nil)
	if err != nil {
		t.Fatal(err)
	}
	pcf := &peopleClientFactoryMock{
		newClientStub: func(config auth.Oauth2Config, request *http.Request) (PeopleClientInterface, error) {
			return &peopleClientMock{getLoggedUserinfoStub: func() string { return "usertest" }}, nil
		},
	}
	ytcf := &youtubeClientFactoryMock{
		newClientStub: func(config auth.Oauth2Config, request *http.Request) (YoutubeClientInterface, error) {
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
		ytcf           YoutubeClientFactoryInterface
		pcf            PeopleClientFactoryInterface
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
				oauth2C:        auth.Oauth2Config{},
				ytcf:           ytcf,
				pcf:            pcf,
				serverBasepath: serverBasepath,
				htmlTemplate:   "",
			},
			want: http.StatusOK,
		},
		{
			name: "redirect case - error on creating youtube client",
			args: args{
				oauth2C: auth.Oauth2Config{},
				ytcf: &youtubeClientFactoryMock{
					newClientStub: func(config auth.Oauth2Config, request *http.Request) (YoutubeClientInterface, error) {
						return nil, fmt.Errorf("testerror")
					},
				},
				pcf:            pcf,
				serverBasepath: serverBasepath,
				htmlTemplate:   "",
			},
			want: http.StatusTemporaryRedirect,
		},
		{
			name: "redirect case - error on creating youtube client",
			args: args{
				oauth2C: auth.Oauth2Config{},
				ytcf:    ytcf,
				pcf: &peopleClientFactoryMock{
					newClientStub: func(config auth.Oauth2Config, request *http.Request) (PeopleClientInterface, error) {
						return nil, fmt.Errorf("testerror")
					},
				},
				serverBasepath: serverBasepath,
				htmlTemplate:   "",
			},
			want: http.StatusTemporaryRedirect,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerFunction := GetYoutubeChannelsVideos(tt.args.oauth2C, tt.args.ytcf, tt.args.pcf,
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ko" {
			// simulate a not found
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()
	oauth2C := auth.Oauth2Config{Oauth2ConfigProvider: &testing_utils.Oauth2Mock{}}

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

		ctx := req.Context()
		ctx = context.WithValue(ctx, sessions.TokenCtxKey{}, &oauth2.Token{})
		req = req.WithContext(ctx)

		return req
	}

	const (
		successCase               = "success case"
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
			name: successCase,
			args: args{
				oauth2C:        oauth2C,
				serverBasepath: "http://localhost:8900",
				recorder:       httptest.NewRecorder(),
			},
			want: http.StatusOK,
		},
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
			want: http.StatusInternalServerError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerFunction := MarkAsViewed(tt.args.oauth2C, tt.args.serverBasepath)
			switch tt.name {
			case successCase:
				handlerFunction(tt.args.recorder, createMockRequest("/ok"))
				if tt.args.recorder.Code != tt.want {
					t.Errorf("MarkAsViewed() = %v, want %v", tt.args.recorder.Code, tt.want)
				}
			case failureCaseBadStatus:
				handlerFunction(tt.args.recorder, createMockRequest("/ko"))
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
				if err != nil {
					t.Fatal(err)
				}
				handlerFunction(tt.args.recorder, req)
				if tt.args.recorder.Code != tt.want {
					t.Errorf("MarkAsViewed() = %v, want %v", tt.args.recorder.Code, tt.want)
				}
			case tokenNotFound:
				req := createMockRequest("/ok")
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
		ch   chan<- YTChannel
	}
	tests := []struct {
		name    string
		args    args
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
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processYouTubeChannel(tt.args.svc, tt.args.item, tt.args.ch)
			if (err != nil) != tt.wantErr {
				t.Errorf("processYouTubeChannel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
