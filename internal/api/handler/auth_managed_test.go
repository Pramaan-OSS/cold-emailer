package handler

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pramaan-oss/cold-email/internal/app/auth"
	"github.com/pramaan-oss/cold-email/internal/errx"
	"github.com/pramaan-oss/cold-email/internal/pramaan"
)

type managedBeginAuth struct {
	auth.AuthService
	calls int
}

func (f *managedBeginAuth) ManagedBegin(context.Context) (string, *errx.Error) {
	f.calls++
	return strings.Repeat("a", 64), nil
}

type managedNeverCall struct{ calls int }

func (f *managedNeverCall) Call(context.Context, string, any, any, ...pramaan.Option) error {
	f.calls++
	return pramaan.ErrUnavailable
}
func TestManagedBeginOnlyAcceptsOwnAppOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, origin := range []string{"", "https://other.example.test", "http://localhost:5173"} {
		a := &managedBeginAuth{}
		client := &managedNeverCall{}
		h := &Handler{AuthService: a, ManagedClient: client, ManagedState: pramaan.ManagedState{AuthReady: true, AuthOrigin: "http://localhost:5173"}}
		r := gin.New()
		r.POST("/begin", h.ManagedBegin)
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/begin", nil)
		req.Header.Set("Origin", origin)
		r.ServeHTTP(w, req)
		if origin == "http://localhost:5173" {
			if w.Code != 200 || a.calls != 1 {
				t.Fatal("same app refused", w.Code)
			}
		} else if w.Code == 200 || a.calls != 0 {
			t.Fatal("other origin accepted")
		}
	}
}
func TestManagedSessionRefusesWrongBindingBeforeVerification(t *testing.T) {
	client := &managedNeverCall{}
	h := &Handler{ManagedClient: client, ManagedState: pramaan.ManagedState{AuthReady: true, AuthOrigin: "http://localhost:5173"}}
	r := gin.New()
	r.POST("/session", h.ManagedSession)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/session", strings.NewReader(`{"state":"`+strings.Repeat("a", 64)+`","binding":"other","token":"private-token"}`))
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code == 200 || client.calls != 0 || strings.Contains(w.Body.String(), "private-token") {
		t.Fatal("binding was not refused safely")
	}
}
