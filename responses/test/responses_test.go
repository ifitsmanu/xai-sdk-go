package responses_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/responses"
)

func TestCreateAndText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_1","model":"m","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"hello"}]}]}`))
	}))
	defer srv.Close()

	svc := responses.NewService(&rpc.Conn{RESTBase: srv.URL, APIKey: "test-key", Logger: rpc.NopLogger()})
	resp, err := svc.Create(context.Background(), responses.Request{Model: "m", Input: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Text() != "hello" || resp.ID != "resp_1" {
		t.Fatalf("response = %+v text %q", resp, resp.Text())
	}
}
