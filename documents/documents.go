// Package documents searches document collections.
package documents

import (
	"context"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// Service is the documents API.
type Service struct {
	conn *rpc.Conn
	stub xaiapi.DocumentsClient
}

// NewService binds the documents API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn, stub: xaiapi.NewDocumentsClient(conn.GRPC)}
}

// Search runs a document search.
func (s *Service) Search(ctx context.Context, req *xaiapi.SearchRequest) (*xaiapi.SearchResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.Search(ctx, req)
}
