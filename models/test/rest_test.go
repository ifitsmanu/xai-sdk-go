package models_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/models"
)

func TestVideoNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v1/video-generation-models" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"id":"grok-imagine-video","aliases":["alias"]}]}`))
	}))
	defer srv.Close()

	svc := models.NewService(&rpc.Conn{
		RESTBase: srv.URL,
		APIKey:   "test-key",
		Logger:   rpc.NopLogger(),
	})
	names, err := svc.VideoNames(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "grok-imagine-video" {
		t.Fatalf("names = %v", names)
	}
}
