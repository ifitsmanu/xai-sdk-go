// Package auth reads API key metadata.
package auth

import (
	"context"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Service is the auth API.
type Service struct {
	conn *rpc.Conn
	stub xaiapi.AuthClient
}

// NewService binds the auth API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn, stub: xaiapi.NewAuthClient(conn.GRPC)}
}

// APIKey returns information about the key used by this client.
// The secret itself is not returned.
func (s *Service) APIKey(ctx context.Context) (*xaiapi.ApiKey, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.GetApiKeyInfo(ctx, &emptypb.Empty{})
}
