// Package voice covers speech, transcription, and the realtime WebSockets.
// These endpoints are REST and WebSocket. They are not in the public gRPC protos.
package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

// Speech is a POST /v1/tts body. Language is required; "auto" detects it.
type Speech struct {
	Text              string            `json:"text"`
	Language          string            `json:"language"`
	VoiceID           string            `json:"voice_id,omitempty"`
	Format            *Format           `json:"output_format,omitempty"`
	Speed             float64           `json:"speed,omitempty"`
	TextNormalization bool              `json:"text_normalization,omitempty"`
	Replace           map[string]string `json:"replace,omitempty"`
}

// Format is the audio codec. The server default is MP3 at 24 kHz and 128 kbps.
type Format struct {
	Codec      string `json:"codec"`
	SampleRate int    `json:"sample_rate,omitempty"`
	BitRate    int    `json:"bit_rate,omitempty"`
}

// Audio is synthesized speech and its media type, such as audio/mpeg.
type Audio struct {
	Data        []byte
	ContentType string
}

// Speak synthesizes speech with POST /v1/tts.
func (s *Service) Speak(ctx context.Context, req Speech) (*Audio, error) {
	if req.Language == "" {
		return nil, errors.New("xai: speech language is required; use \"auto\" to detect it")
	}
	data, contentType, err := s.conn.RESTClient().DoBody(ctx, http.MethodPost, "/v1/tts", req)
	if err != nil {
		return nil, err
	}
	return &Audio{Data: data, ContentType: contentType}, nil
}

// Voices returns GET /v1/tts/voices.
func (s *Service) Voices(ctx context.Context) (json.RawMessage, error) {
	return s.conn.RESTClient().Do(ctx, http.MethodGet, "/v1/tts/voices", nil)
}

// CustomVoices returns one page of GET /v1/custom-voices. Zero and empty values use server defaults.
func (s *Service) CustomVoices(ctx context.Context, limit int, pageToken string) (json.RawMessage, error) {
	q := url.Values{"pagination_token": {pageToken}}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return s.conn.RESTClient().Do(ctx, http.MethodGet, rest.Query("/v1/custom-voices", q), nil)
}

// CustomVoice returns one custom voice.
func (s *Service) CustomVoice(ctx context.Context, id string) (json.RawMessage, error) {
	return s.conn.RESTClient().Do(ctx, http.MethodGet, rest.Path("/v1/custom-voices", id), nil)
}

// DeleteCustomVoice deletes a custom voice.
func (s *Service) DeleteCustomVoice(ctx context.Context, id string) error {
	_, err := s.conn.RESTClient().Do(ctx, http.MethodDelete, rest.Path("/v1/custom-voices", id), nil)
	return err
}

// Transcription is a POST /v1/stt request. Set Audio or URL.
type Transcription struct {
	Model    string
	Audio    []byte
	Filename string
	URL      string
	Language string
	Format   bool // written-form numbers and units; needs Language
	Diarize  bool
	Keyterms []string
}

// Transcript is a POST /v1/stt result.
type Transcript struct {
	Text     string  `json:"text"`
	Language string  `json:"language"`
	Duration float64 `json:"duration"`
	Words    []Word  `json:"words"`
}

// Word is one timed word. Speaker is set only when diarization is on.
type Word struct {
	Text       string  `json:"text"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Confidence float64 `json:"confidence"`
	Speaker    *int    `json:"speaker"`
}

// Transcribe turns audio into text with POST /v1/stt.
func (s *Service) Transcribe(ctx context.Context, req Transcription) (*Transcript, error) {
	if (len(req.Audio) == 0) == (req.URL == "") {
		return nil, errors.New("xai: set exactly one of Audio or URL")
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fields := url.Values{"model": {req.Model}, "url": {req.URL}, "language": {req.Language}, "keyterm": req.Keyterms}
	if req.Format {
		fields.Set("format", "true")
	}
	if req.Diarize {
		fields.Set("diarize", "true")
	}
	for key, values := range fields {
		for _, v := range values {
			if v == "" {
				continue
			}
			if err := w.WriteField(key, v); err != nil {
				return nil, err
			}
		}
	}
	if len(req.Audio) > 0 { // the file must be the last field
		name := req.Filename
		if name == "" {
			name = "audio"
		}
		part, err := w.CreateFormFile("file", name)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(req.Audio); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	raw, _, err := s.conn.RESTClient().Read(ctx, http.MethodPost, "/v1/stt", &rest.Body{Reader: &buf, ContentType: w.FormDataContentType()})
	if err != nil {
		return nil, err
	}
	var out Transcript
	return &out, json.Unmarshal(raw, &out)
}

// Secret is a short-lived token a browser or phone can use for the realtime socket.
type Secret struct {
	Value     string `json:"value"`
	ExpiresAt int64  `json:"expires_at"`
}

// ClientSecret mints a token with POST /v1/realtime/client_secrets.
func (s *Service) ClientSecret(ctx context.Context, seconds int) (*Secret, error) {
	body := map[string]any{"expires_after": map[string]int{"seconds": seconds}}
	var out Secret
	return &out, s.conn.RESTClient().Call(ctx, http.MethodPost, "/v1/realtime/client_secrets", body, &out)
}

// Dial opens the speech-to-speech socket at /v1/realtime.
func (s *Service) Dial(ctx context.Context, model string) (*Socket, error) {
	return s.dial(ctx, "/v1/realtime", url.Values{"model": {model}})
}

// DialTTS opens the streaming text-to-speech socket. Query keys follow the docs:
// language, voice, codec, sample_rate, bit_rate.
func (s *Service) DialTTS(ctx context.Context, query url.Values) (*Socket, error) {
	return s.dial(ctx, "/v1/tts", query)
}

// DialSTT opens the streaming speech-to-text socket. Query keys follow the docs:
// model, language, encoding, sample_rate, interim_results.
func (s *Service) DialSTT(ctx context.Context, query url.Values) (*Socket, error) {
	return s.dial(ctx, "/v1/stt", query)
}

func (s *Service) dial(ctx context.Context, path string, query url.Values) (*Socket, error) {
	ws, err := s.conn.DialWS(ctx, rest.Query(path, query))
	if err != nil {
		return nil, err
	}
	return &Socket{Conn: ws}, nil
}

// Socket is one voice WebSocket. Send and Recv pass JSON events through.
type Socket struct {
	*wsx.Conn
}

// Update sends a session.update event. session is the session object from the docs.
func (s *Socket) Update(ctx context.Context, session any) error {
	return s.Send(ctx, map[string]any{"type": "session.update", "session": session})
}
