// Package image generates images.
package image

import (
	"context"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// Service is the image API.
type Service struct {
	conn *rpc.Conn
	stub xaiapi.ImageClient
}

// NewService binds the image API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn, stub: xaiapi.NewImageClient(conn.GRPC)}
}

// Request starts a text-to-image call. Set Image or Images on the result to edit.
func Request(model, prompt string) *xaiapi.GenerateImageRequest {
	return &xaiapi.GenerateImageRequest{Model: model, Prompt: prompt}
}

// Generate creates images from the request.
func (s *Service) Generate(ctx context.Context, req *xaiapi.GenerateImageRequest) (*xaiapi.ImageResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	s.conn.Logger.DebugContext(ctx, "image generate", "model", req.GetModel())
	return s.stub.GenerateImage(ctx, req)
}
