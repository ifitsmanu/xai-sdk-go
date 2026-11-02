package voice_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coder/websocket"
	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/voice"
)

func TestSpeak(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tts" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte("audio"))
	}))
	defer srv.Close()

	svc := voice.NewService(&rpc.Conn{RESTBase: srv.URL, APIKey: "test-key", Logger: rpc.NopLogger()})
	got, err := svc.Speak(context.Background(), voice.Speech{Text: "hi", VoiceID: "eve", Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "audio" {
		t.Fatalf("audio = %q", got)
	}
}

func TestDialRealtime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("model") != "voice-model" {
			t.Errorf("model = %q", r.URL.Query().Get("model"))
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		ctx := r.Context()
		_, msg, err := c.Read(ctx)
		if err != nil {
			return
		}
		if err := c.Write(ctx, websocket.MessageText, msg); err != nil {
			return
		}
	}))
	defer srv.Close()

	svc := voice.NewService(&rpc.Conn{RESTBase: srv.URL, APIKey: "test-key", Logger: rpc.NopLogger()})
	sock, err := svc.Dial(context.Background(), "voice-model")
	if err != nil {
		t.Fatal(err)
	}
	defer sock.Close()
	if err := sock.Update(context.Background(), map[string]any{"voice": "eve"}); err != nil {
		t.Fatal(err)
	}
	got, err := sock.Recv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("empty event")
	}
}
