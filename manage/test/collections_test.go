package manage_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/manage"
)

func TestCreateUsesManagementKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mgmt-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if r.URL.Path != "/v1/collections" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"collection_id":"col_1","collection_name":"docs"}`))
	}))
	defer srv.Close()

	client := manage.New(manage.Config{Key: "mgmt-key", Base: srv.URL})
	got, err := client.Collections.Create(context.Background(), manage.CreateCollection{
		CollectionName: "docs",
		ModelName:      "embed-model",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "col_1" || got.Name != "docs" {
		t.Fatalf("collection = %+v", got)
	}
}
