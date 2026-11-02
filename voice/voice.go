// Package voice covers speech, transcription, and the realtime WebSocket.
// These endpoints are REST and WebSocket. They are not in the public gRPC protos.
package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/internal/wsx"
)

// Service is the voice API.
type Service struct {
	conn *rpc.Conn
}

// NewService binds the voice API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn}
}

// Speech is a POST /v1/tts body. VoiceID and Language come from the caller, not a built-in catalog.
type Speech struct {
	Text     string `json:"text"`
	VoiceID  string `json:"voice_id,omitempty"`
	Language string `json:"language,omitempty"`
}

// Speak returns the audio bytes from POST /v1/tts.
func (s *Service) Speak(ctx context.Context, req Speech) ([]byte, error) {
	return s.conn.RESTClient().Do(ctx, http.MethodPost, "/v1/tts", req)
}

// CustomVoices returns GET /v1/custom-voices. limit 0 uses the server default.
func (s *Service) CustomVoices(ctx context.Context, limit int) (json.RawMessage, error) {
	path := "/v1/custom-voices"
	if limit > 0 {
		path += "?limit=" + strconv.Itoa(limit)
	}
	raw, err := s.conn.RESTClient().Do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// CustomVoice returns one custom voice.
func (s *Service) CustomVoice(ctx context.Context, id string) (json.RawMessage, error) {
	raw, err := s.conn.RESTClient().Do(ctx, http.MethodGet, "/v1/custom-voices/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// DeleteCustomVoice deletes a custom voice.
func (s *Service) DeleteCustomVoice(ctx context.Context, id string) error {
	_, err := s.conn.RESTClient().Do(ctx, http.MethodDelete, "/v1/custom-voices/"+url.PathEscape(id), nil)
	return err
}

// Voices returns GET /v1/tts/voices as JSON.
func (s *Service) Voices(ctx context.Context) (json.RawMessage, error) {
	raw, err := s.conn.RESTClient().Do(ctx, http.MethodGet, "/v1/tts/voices", nil)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// Transcript is a POST /v1/stt result.
type Transcript struct {
	Text     string          `json:"text"`
	Duration float64         `json:"duration"`
	Words    json.RawMessage `json:"words"`
	Raw      json.RawMessage `json:"-"`
}

// Transcribe sends one audio file to POST /v1/stt.
// Model is required by the caller. Format, language, and keyterms are optional.
func (s *Service) Transcribe(ctx context.Context, model, filename string, audio []byte, language string, format bool, keyterms []string) (*Transcript, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fields := [][2]string{{"model", model}}
	if language != "" {
		fields = append(fields, [2]string{"language", language})
	}
	if format {
		fields = append(fields, [2]string{"format", "true"})
	}
	for _, term := range keyterms {
		fields = append(fields, [2]string{"keyterm", term})
	}
	for _, field := range fields {
		if err := w.WriteField(field[0], field[1]); err != nil {
			return nil, err
		}
	}
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(audio); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	raw, err := s.postForm(ctx, "/v1/stt", w.FormDataContentType(), &buf)
	if err != nil {
		return nil, err
	}
	var out Transcript
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	out.Raw = json.RawMessage(raw)
	return &out, nil
}

// ClientSecret asks POST /v1/realtime/client_secrets for a short-lived token.
// seconds is expires_after.seconds. The docs do not accept a session field.
func (s *Service) ClientSecret(ctx context.Context, seconds int) (json.RawMessage, error) {
	body := map[string]any{"expires_after": map[string]int{"seconds": seconds}}
	raw, err := s.conn.RESTClient().Do(ctx, http.MethodPost, "/v1/realtime/client_secrets", body)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// Dial opens wss://…/v1/realtime. model is the query value the caller chooses.
func (s *Service) Dial(ctx context.Context, model string) (*Socket, error) {
	u := rest.WebSocketBase(s.conn.RESTBase) + "/v1/realtime?model=" + url.QueryEscape(model)
	ws, err := wsx.Dial(ctx, u, s.conn.APIKey, s.conn.UserAgent)
	if err != nil {
		return nil, err
	}
	return &Socket{conn: ws}, nil
}

// DialTTS opens the text-to-speech WebSocket. language, voice, and codec are query values.
func (s *Service) DialTTS(ctx context.Context, language, voice, codec string) (*Socket, error) {
	q := url.Values{}
	if language != "" {
		q.Set("language", language)
	}
	if voice != "" {
		q.Set("voice", voice)
	}
	if codec != "" {
		q.Set("codec", codec)
	}
	u := rest.WebSocketBase(s.conn.RESTBase) + "/v1/tts"
	if encoded := q.Encode(); encoded != "" {
		u += "?" + encoded
	}
	ws, err := wsx.Dial(ctx, u, s.conn.APIKey, s.conn.UserAgent)
	if err != nil {
		return nil, err
	}
	return &Socket{conn: ws}, nil
}

// STTQuery is the query string for the speech-to-text WebSocket.
type STTQuery struct {
	Model          string
	Language       string
	Encoding       string
	SampleRate     int
	InterimResults bool
}

// DialSTT opens the speech-to-text WebSocket.
func (s *Service) DialSTT(ctx context.Context, q STTQuery) (*Socket, error) {
	v := url.Values{}
	if q.Model != "" {
		v.Set("model", q.Model)
	}
	if q.Language != "" {
		v.Set("language", q.Language)
	}
	if q.Encoding != "" {
		v.Set("encoding", q.Encoding)
	}
	if q.SampleRate > 0 {
		v.Set("sample_rate", strconv.Itoa(q.SampleRate))
	}
	if q.InterimResults {
		v.Set("interim_results", "true")
	}
	u := rest.WebSocketBase(s.conn.RESTBase) + "/v1/stt"
	if encoded := v.Encode(); encoded != "" {
		u += "?" + encoded
	}
	ws, err := wsx.Dial(ctx, u, s.conn.APIKey, s.conn.UserAgent)
	if err != nil {
		return nil, err
	}
	return &Socket{conn: ws}, nil
}

// Socket is one voice WebSocket. Send and Recv pass JSON events through.
type Socket struct {
	conn *wsx.Conn
}

// Send writes one JSON event.
func (s *Socket) Send(ctx context.Context, event any) error { return s.conn.Send(ctx, event) }

// Recv reads the next server event.
func (s *Socket) Recv(ctx context.Context) (json.RawMessage, error) { return s.conn.Recv(ctx) }

// Close closes the socket.
func (s *Socket) Close() error { return s.conn.Close() }

// Update sends a session.update event. session is the session object from the docs.
func (s *Socket) Update(ctx context.Context, session any) error {
	return s.Send(ctx, map[string]any{"type": "session.update", "session": session})
}

func (s *Service) postForm(ctx context.Context, path, contentType string, body io.Reader) ([]byte, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.conn.RESTBase+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.conn.APIKey)
	req.Header.Set("Content-Type", contentType)
	if s.conn.UserAgent != "" {
		req.Header.Set("User-Agent", s.conn.UserAgent)
	}
	client := s.conn.RESTClient().HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &rest.Error{Method: http.MethodPost, Path: path, Status: resp.StatusCode, Body: string(raw)}
	}
	return raw, nil
}
