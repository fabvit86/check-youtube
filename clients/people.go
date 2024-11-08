package clients

import (
	"checkYoutube/logging"
	"context"
	"fmt"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/people/v1"
	"log/slog"
)

type PeopleClientInterface interface {
	GetLoggedUserinfo() string
}

type PeopleClientFactoryInterface interface {
	NewClient(oauth2.TokenSource) (PeopleClientInterface, error)
}

type peopleClient struct {
	svc people.Service
}

type PeopleClientFactory struct{}

// NewClient creates a new people service client using the given token source
func (p *PeopleClientFactory) NewClient(ts oauth2.TokenSource) (PeopleClientInterface, error) {
	const funcName = "NewClient"

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

func (p *peopleClient) GetLoggedUserinfo() string {
	const funcName = "GetLoggedUserinfo"

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
