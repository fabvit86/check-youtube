package test

import (
	"github.com/gorilla/sessions"
	"net/http"
	"testing"
)

func SetOauth2SessionValue[T any](t *testing.T, sessionStore *sessions.CookieStore,
	req *http.Request, sessionName, key string, value T) {
	session, err := sessionStore.Get(req, sessionName)
	if err != nil {
		t.Fatal(err)
	}
	session.Values[key] = value
}

func DeleteOauth2SessionValue(t *testing.T, sessionStore *sessions.CookieStore,
	req *http.Request, sessionName, key string) {
	session, err := sessionStore.Get(req, sessionName)
	if err != nil {
		t.Fatal(err)
	}
	delete(session.Values, key)
}
