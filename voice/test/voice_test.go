package voice_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	audio := bytes.Repeat([]byte{1, 2, 3}, 70<<10)
	delta := `{"type":"response.output_audio.delta","delta":"` + base64.StdEncoding.EncodeToString(audio) + `"}`
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

	sock, err := service(srv.URL).Dial(context.Background(), url.Values{"model": {"voice-model"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sock.Close() }()
	event, err := sock.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := event.Audio()
	if err != nil || event.Type != voice.EventAudioDelta || !bytes.Equal(got, audio) {
		t.Fatalf("type %q, %d audio bytes, err %v", event.Type, len(got), err)
	}
}

// Realtime tools use their own wire shape: a function is nested under "function",
// while web search takes its domain list directly. The Responses API differs on both.
func TestRealtimeToolShapes(t *testing.T) {
	raw, err := json.Marshal(voice.Session{Tools: []voice.Tool{
		voice.Function{Name: "lookup", Parameters: map[string]any{"type": "object"}},
		voice.WebSearch{AllowedDomains: []string{"x.ai"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"object"}}},{"type":"web_search","allowed_domains":["x.ai"]}]}`
	if string(raw) != want {
		t.Fatalf("got  %s\nwant %s", raw, want)
	}
}
