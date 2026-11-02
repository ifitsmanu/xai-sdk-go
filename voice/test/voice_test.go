package voice_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/voice"
)

func service(url string) *voice.Service {
	return voice.NewService(&rpc.Conn{RESTBase: url, APIKey: "test-key", Logger: rpc.NopLogger()})
}

// The live API returns 422 when language is missing, so Speak rejects it before sending.
func TestSpeakRequiresLanguage(t *testing.T) {
	if _, err := service("http://unused").Speak(context.Background(), voice.Speech{Text: "hi"}); err == nil {
		t.Fatal("expected an error for a missing language")
	}
}

func TestSpeakReturnsAudioWithType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("mp3"))
	}))
	defer srv.Close()

	got, err := service(srv.URL).Speak(context.Background(), voice.Speech{Text: "hi", Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Data) != "mp3" || got.ContentType != "audio/mpeg" {
		t.Fatalf("audio = %q %q", got.Data, got.ContentType)
	}
}

// Realtime audio deltas reach about 108 KB live. The socket must read messages
// far above the WebSocket library's 32 KiB default.
func TestRealtimeReadsLargeAudioDelta(t *testing.T) {
	delta := `{"type":"response.output_audio.delta","delta":"` + strings.Repeat("A", 200<<10) + `"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		_ = c.Write(r.Context(), websocket.MessageText, []byte(delta))
		_, _, _ = c.Read(r.Context()) // hold the socket open until the client closes
	}))
	defer srv.Close()

	sock, err := service(srv.URL).Dial(context.Background(), "voice-model")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sock.Close() }()
	got, err := sock.Recv(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(delta) {
		t.Fatalf("read %d bytes, want %d", len(got), len(delta))
	}
}
