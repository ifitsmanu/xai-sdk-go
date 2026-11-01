// Package embed creates embeddings.
package embed

import (
	"context"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// Service is the embedding API.
type Service struct {
	conn *rpc.Conn
	stub xaiapi.EmbedderClient
}

// NewService binds the embedding API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn, stub: xaiapi.NewEmbedderClient(conn.GRPC)}
}

// Embed calls the embedding model.
func (s *Service) Embed(ctx context.Context, req *xaiapi.EmbedRequest) (*xaiapi.EmbedResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	s.conn.Logger.DebugContext(ctx, "embed", "model", req.GetModel(), "inputs", len(req.GetInput()))
	return s.stub.Embed(ctx, req)
}
