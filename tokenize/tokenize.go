// Package tokenize counts tokens for a model.
package tokenize

import (
	"context"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// Service is the tokenize API.
type Service struct {
	conn *rpc.Conn
	stub xaiapi.TokenizeClient
}

// NewService binds the tokenize API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn, stub: xaiapi.NewTokenizeClient(conn.GRPC)}
}

// Text tokenizes text with the named model.
func (s *Service) Text(ctx context.Context, model, text string) (*xaiapi.TokenizeTextResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	s.conn.Logger.DebugContext(ctx, "tokenize", "model", model, "text.bytes", len(text))
	return s.stub.TokenizeText(ctx, &xaiapi.TokenizeTextRequest{Model: model, Text: text})
}
