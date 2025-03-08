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
	GetLoggedUserinfo() Userinfo
}

type PeopleClientFactoryInterface interface {
	NewClient(oauth2.TokenSource) (PeopleClientInterface, error)
}

type peopleClient struct {
	svc people.Service
}

type PeopleClientFactory struct{}

type Userinfo struct {
	Id          string
	DisplayName string
}

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

func (p *peopleClient) GetLoggedUserinfo() Userinfo {
	const funcName = "GetLoggedUserinfo"
	user := Userinfo{}

	userinfo, err := p.svc.People.
		Get("people/me").
		PersonFields("names,metadata").
		Do()
	if err != nil {
		slog.Error(fmt.Sprintf("error retrieving logged user info: %s", err.Error()),
			logging.FuncNameAttr(funcName))
		return user
	}

	if len(userinfo.Names) > 0 {
		user.DisplayName = userinfo.Names[0].DisplayName
	}
	if len(userinfo.Metadata.Sources) > 0 && userinfo.Metadata.Sources[0].Id != "" {
		user.Id = userinfo.Metadata.Sources[0].Id
	}

	return user
}
