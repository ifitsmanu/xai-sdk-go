// Package batch manages asynchronous batches.
package batch

import (
	"context"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// Service is the batch API.
type Service struct {
	conn *rpc.Conn
	stub xaiapi.BatchMgmtClient
}

// NewService binds the batch API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn, stub: xaiapi.NewBatchMgmtClient(conn.GRPC)}
}

// Create starts a batch.
func (s *Service) Create(ctx context.Context, req *xaiapi.CreateBatchRequest) (*xaiapi.Batch, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	s.conn.Logger.InfoContext(ctx, "batch create", "batch.name", req.GetName())
	b, err := s.stub.CreateBatch(ctx, req)
	if err != nil {
		return nil, err
	}
	s.conn.Logger.InfoContext(ctx, "batch created", "batch.id", b.GetBatchId())
	return b, nil
}

// Get fetches a batch.
func (s *Service) Get(ctx context.Context, id string) (*xaiapi.Batch, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.GetBatch(ctx, &xaiapi.GetBatchRequest{BatchId: id})
}

// List returns batches for the team.
func (s *Service) List(ctx context.Context, req *xaiapi.ListBatchesRequest) (*xaiapi.ListBatchesResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	if req == nil {
		req = &xaiapi.ListBatchesRequest{}
	}
	return s.stub.ListBatches(ctx, req)
}

// Cancel stops outstanding work in a batch.
func (s *Service) Cancel(ctx context.Context, id string) (*xaiapi.Batch, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	s.conn.Logger.InfoContext(ctx, "batch cancel", "batch.id", id)
	return s.stub.CancelBatch(ctx, &xaiapi.CancelBatchRequest{BatchId: id})
}

// AddRequests appends requests to a batch.
func (s *Service) AddRequests(ctx context.Context, req *xaiapi.AddBatchRequestsRequest) error {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	_, err := s.stub.AddBatchRequests(ctx, req)
	return err
}

// Requests lists metadata for the requests in a batch.
func (s *Service) Requests(ctx context.Context, req *xaiapi.ListBatchRequestMetadataRequest) (*xaiapi.ListBatchRequestMetadataResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	if req == nil {
		req = &xaiapi.ListBatchRequestMetadataRequest{}
	}
	return s.stub.ListBatchRequestMetadata(ctx, req)
}

// Result fetches one request result from a batch.
func (s *Service) Result(ctx context.Context, batchID, requestID string) (*xaiapi.GetBatchRequestResultResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.GetBatchRequestResult(ctx, &xaiapi.GetBatchRequestResultRequest{
		BatchId:        batchID,
		BatchRequestId: requestID,
	})
}

// Results lists processed results.
func (s *Service) Results(ctx context.Context, req *xaiapi.ListBatchResultsRequest) (*xaiapi.ListBatchResultsResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.ListBatchResults(ctx, req)
}
