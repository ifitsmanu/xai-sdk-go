// Package chat is the conversation client: transcripts, sampling, streaming, and tools.
package chat

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"time"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
	"google.golang.org/protobuf/proto"
)

// ErrNoMessages is returned when a completion is requested with an empty transcript.
var ErrNoMessages = errors.New("xai: chat request has no messages")

// Config is the mutable request behind a Chat.
// Pointer fields are omitted when nil so the server keeps its default.
type Config struct {
	Model               string
	Messages            []*xaiapi.Message
	User                string
	SafetyIdentifier    string
	MaxTokens           *int32
	N                   *int32
	Seed                *int32
	Stop                []string
	Temperature         *float32
	TopP                *float32
	Logprobs            bool
	TopLogprobs         *int32
	Tools               []*xaiapi.Tool
	ToolChoice          *xaiapi.ToolChoice
	ParallelToolCalls   *bool
	ResponseFormat      *xaiapi.ResponseFormat
	FrequencyPenalty    *float32
	PresencePenalty     *float32
	ReasoningEffort     xaiapi.ReasoningEffort
	SearchParameters    *xaiapi.SearchParameters
	StoreMessages       bool
	PreviousResponseID  string
	UseEncryptedContent bool
	MaxTurns            *int32
	Include             []xaiapi.IncludeOption
	AgentCount          xaiapi.AgentCount
	ServiceTier         xaiapi.ServiceTier
	Guidelines          []*xaiapi.Guideline
}

// Service is the chat API.
type Service struct {
	conn *rpc.Conn
	stub xaiapi.ChatClient
}

// NewService binds the chat API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn, stub: xaiapi.NewChatClient(conn.GRPC)}
}

// Create starts a local conversation. It does not call the API.
func (s *Service) Create(cfg Config) *Chat {
	req := &xaiapi.GetCompletionsRequest{
		Model:               cfg.Model,
		Messages:            append([]*xaiapi.Message(nil), cfg.Messages...),
		User:                cfg.User,
		MaxTokens:           cfg.MaxTokens,
		Seed:                cfg.Seed,
		Stop:                append([]string(nil), cfg.Stop...),
		Temperature:         cfg.Temperature,
		TopP:                cfg.TopP,
		Logprobs:            cfg.Logprobs,
		TopLogprobs:         cfg.TopLogprobs,
		Tools:               cfg.Tools,
		ToolChoice:          cfg.ToolChoice,
		ParallelToolCalls:   cfg.ParallelToolCalls,
		ResponseFormat:      cfg.ResponseFormat,
		FrequencyPenalty:    cfg.FrequencyPenalty,
		PresencePenalty:     cfg.PresencePenalty,
		SearchParameters:    cfg.SearchParameters,
		StoreMessages:       cfg.StoreMessages,
		UseEncryptedContent: cfg.UseEncryptedContent,
		MaxTurns:            cfg.MaxTurns,
		Include:             append([]xaiapi.IncludeOption(nil), cfg.Include...),
		ServiceTier:         cfg.ServiceTier,
		Guidelines:          cfg.Guidelines,
	}
	if cfg.SafetyIdentifier != "" {
		req.SafetyIdentifier = &cfg.SafetyIdentifier
	}
	if cfg.PreviousResponseID != "" {
		req.PreviousResponseId = &cfg.PreviousResponseID
	}
	if cfg.ReasoningEffort != xaiapi.ReasoningEffort_INVALID_EFFORT {
		effort := cfg.ReasoningEffort
		req.ReasoningEffort = &effort
	}
	if cfg.AgentCount != xaiapi.AgentCount_AGENT_COUNT_UNSPECIFIED {
		count := cfg.AgentCount
		req.AgentCount = &count
	}
	if cfg.N != nil {
		req.N = cfg.N
	}
	return &Chat{svc: s, req: req}
}

// Complete calls GetCompletion with the request as given.
func (s *Service) Complete(ctx context.Context, req *xaiapi.GetCompletionsRequest) (*xaiapi.GetChatCompletionResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.GetCompletion(ctx, req)
}

// Stored fetches a response that was produced with StoreMessages.
func (s *Service) Stored(ctx context.Context, responseID string) (*Response, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	resp, err := s.stub.GetStoredCompletion(ctx, &xaiapi.GetStoredCompletionRequest{ResponseId: responseID})
	if err != nil {
		return nil, err
	}
	return &Response{Proto: resp}, nil
}

// DeleteStored deletes a stored response.
func (s *Service) DeleteStored(ctx context.Context, responseID string) (*xaiapi.DeleteStoredCompletionResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.DeleteStoredCompletion(ctx, &xaiapi.DeleteStoredCompletionRequest{ResponseId: responseID})
}

// Compact asks the server to fold the transcript into an opaque blob.
func (s *Service) Compact(ctx context.Context, model string, messages []*xaiapi.Message) (*xaiapi.CompactContextResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.CompactContext(ctx, &xaiapi.CompactContextRequest{Model: model, Input: messages})
}

// Wait polls a deferred completion until it finishes, expires, or the budget ends.
// The budget is ten minutes. Each poll is limited by the connection timeout.
func (s *Service) Wait(ctx context.Context, requestID string) (*Response, error) {
	if requestID == "" {
		return nil, errors.New("xai: deferred request id is empty")
	}
	pollCtx, pollCancel := context.WithTimeout(ctx, deferredBudget)
	defer pollCancel()
	ticker := time.NewTicker(deferredInterval)
	defer ticker.Stop()
	for {
		resp, err := s.pollDeferred(pollCtx, requestID)
		if err != nil {
			return nil, err
		}
		switch resp.GetStatus() {
		case xaiapi.DeferredStatus_DONE:
			if resp.GetResponse() == nil {
				return nil, errors.New("xai: deferred completion finished without a response")
			}
			return &Response{Proto: resp.GetResponse()}, nil
		case xaiapi.DeferredStatus_FAILED:
			return &Response{Proto: resp.GetResponse()}, errors.New("xai: deferred completion failed")
		case xaiapi.DeferredStatus_EXPIRED:
			return nil, errors.New("xai: deferred completion expired")
		case xaiapi.DeferredStatus_PENDING, xaiapi.DeferredStatus_INVALID_DEFERRED_STATUS:
		default:
			return nil, fmt.Errorf("xai: deferred status %s", resp.GetStatus())
		}
		select {
		case <-pollCtx.Done():
			return nil, pollCtx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Service) pollDeferred(pollCtx context.Context, requestID string) (*xaiapi.GetDeferredCompletionResponse, error) {
	timeout := s.conn.Timeout
	if timeout <= 0 {
		timeout = rpc.DefaultTimeout
	}
	callCtx, cancel := context.WithTimeout(context.WithoutCancel(pollCtx), timeout)
	stop := context.AfterFunc(pollCtx, cancel)
	defer stop()
	defer cancel()
	resp, err := s.stub.GetDeferredCompletion(callCtx, &xaiapi.GetDeferredRequest{RequestId: requestID})
	if err != nil && pollCtx.Err() != nil {
		return nil, pollCtx.Err()
	}
	return resp, err
}

// Deferred fetches one poll of a completion started with Chat.Defer.
func (s *Service) Deferred(ctx context.Context, requestID string) (*xaiapi.GetDeferredCompletionResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.GetDeferredCompletion(ctx, &xaiapi.GetDeferredRequest{RequestId: requestID})
}

// Chat is a local transcript bound to a model and sampling settings.
// It is not safe for concurrent use.
type Chat struct {
	svc *Service
	req *xaiapi.GetCompletionsRequest
}

// Request returns the underlying request. Mutating it changes later calls.
func (ch *Chat) Request() *xaiapi.GetCompletionsRequest { return ch.req }

// Append adds a message to the transcript and returns the chat.
func (ch *Chat) Append(msg *xaiapi.Message) *Chat {
	if msg != nil {
		ch.req.Messages = append(ch.req.Messages, msg)
	}
	return ch
}

// AppendResponse adds every output from a completion onto the transcript.
func (ch *Chat) AppendResponse(resp *Response) *Chat {
	ch.req.Messages = append(ch.req.Messages, MessagesFromResponse(resp)...)
	return ch
}

// UseCompacted drops the transcript and replaces it with the compaction blob.
func (ch *Chat) UseCompacted(compacted *xaiapi.CompactContextResponse) *Chat {
	ch.req.Messages = nil
	if compacted == nil {
		return ch
	}
	ch.req.Messages = []*xaiapi.Message{{
		Role:             xaiapi.MessageRole_ROLE_USER,
		EncryptedContent: compacted.EncryptedContent,
	}}
	return ch
}

// Sample blocks until the model finishes.
// The request is encoded in place. Do not change the chat until Sample returns.
func (ch *Chat) Sample(ctx context.Context) (*Response, error) {
	req, err := ch.request()
	if err != nil {
		return nil, err
	}
	resp, err := ch.svc.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	return &Response{Proto: resp}, nil
}

// SampleN asks for n completions in one request. It does not change the chat's saved N.
func (ch *Chat) SampleN(ctx context.Context, n int32) (*Response, error) {
	if n < 1 {
		return nil, errors.New("xai: completion count must be at least 1")
	}
	req, err := ch.request()
	if err != nil {
		return nil, err
	}
	copied, ok := proto.Clone(req).(*xaiapi.GetCompletionsRequest)
	if !ok || copied == nil {
		return nil, errors.New("xai: chat request could not be copied")
	}
	copied.N = &n
	resp, err := ch.svc.Complete(ctx, copied)
	if err != nil {
		return nil, err
	}
	return &Response{Proto: resp}, nil
}

// Compact sends the current transcript to CompactContext and replaces it with the result.
func (ch *Chat) Compact(ctx context.Context) (*xaiapi.CompactContextResponse, error) {
	req, err := ch.request()
	if err != nil {
		return nil, err
	}
	resp, err := ch.svc.Compact(ctx, req.GetModel(), req.GetMessages())
	if err != nil {
		return nil, err
	}
	ch.UseCompacted(resp)
	return resp, nil
}

// Stream yields each chunk and the response accumulated so far.
// The same StreamDelta and Response pointers are reused for every chunk.
// Content and Reasoning stay current during the stream. The protobuf text
// fields are written when the stream ends. Do not change the chat until then.
func (ch *Chat) Stream(ctx context.Context) iter.Seq2[*StreamDelta, error] {
	return func(yield func(*StreamDelta, error) bool) {
		req, err := ch.request()
		if err != nil {
			yield(nil, err)
			return
		}
		ctx, cancel := ch.svc.conn.Context(ctx)
		defer cancel()
		stream, err := ch.svc.stub.GetCompletionChunk(ctx, req)
		if err != nil {
			yield(nil, err)
			return
		}
		acc := &Response{Proto: &xaiapi.GetChatCompletionResponse{}}
		defer acc.Flush()
		delta := &StreamDelta{Response: acc}
		for {
			chunk, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				yield(nil, err)
				return
			}
			acc.ApplyChunk(chunk)
			delta.Chunk = chunk
			if !yield(delta, nil) {
				return
			}
		}
	}
}

// Defer starts a completion and returns the request id used to poll.
func (ch *Chat) Defer(ctx context.Context) (string, error) {
	req, err := ch.request()
	if err != nil {
		return "", err
	}
	ctx, cancel := ch.svc.conn.Context(ctx)
	defer cancel()
	resp, err := ch.svc.stub.StartDeferredCompletion(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.GetRequestId(), nil
}

// Await starts a deferred completion and polls until it finishes.
func (ch *Chat) Await(ctx context.Context) (*Response, error) {
	id, err := ch.Defer(ctx)
	if err != nil {
		return nil, err
	}
	return ch.svc.Wait(ctx, id)
}

const (
	deferredInterval = 100 * time.Millisecond
	deferredBudget   = 10 * time.Minute
)

func (ch *Chat) request() (*xaiapi.GetCompletionsRequest, error) {
	if ch == nil || ch.req == nil || len(ch.req.Messages) == 0 {
		return nil, ErrNoMessages
	}
	return ch.req, nil
}

// StreamDelta is one streaming chunk plus the response accumulated through that chunk.
type StreamDelta struct {
	Chunk    *xaiapi.GetChatCompletionChunk
	Response *Response
}
