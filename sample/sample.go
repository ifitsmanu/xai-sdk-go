// Package sample is the lower-level text sampling API.
package sample

import (
	"context"
	"errors"
	"io"
	"iter"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// Service is the sample API.
type Service struct {
	conn *rpc.Conn
	stub xaiapi.SampleClient
}

// NewService binds the sample API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn, stub: xaiapi.NewSampleClient(conn.GRPC)}
}

// Text samples text once.
func (s *Service) Text(ctx context.Context, req *xaiapi.SampleTextRequest) (*xaiapi.SampleTextResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.SampleText(ctx, req)
}

// Stream samples text as a stream of responses.
func (s *Service) Stream(ctx context.Context, req *xaiapi.SampleTextRequest) iter.Seq2[*xaiapi.SampleTextResponse, error] {
	return func(yield func(*xaiapi.SampleTextResponse, error) bool) {
		ctx, cancel := s.conn.Context(ctx)
		defer cancel()
		stream, err := s.stub.SampleTextStreaming(ctx, req)
		if err != nil {
			yield(nil, err)
			return
		}
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(resp, nil) {
				return
			}
		}
	}
}
