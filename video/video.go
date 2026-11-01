// Package video generates and extends videos.
package video

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// Service is the video API. Generate and Extend poll until the job finishes.
type Service struct {
	conn *rpc.Conn
	stub xaiapi.VideoClient
}

// NewService binds the video API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn, stub: xaiapi.NewVideoClient(conn.GRPC)}
}

// Generate submits a video job and polls until it finishes or the context ends.
func (s *Service) Generate(ctx context.Context, req *xaiapi.GenerateVideoRequest) (*xaiapi.VideoResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	s.conn.Logger.InfoContext(ctx, "video generate started", "model", req.GetModel())
	started, err := s.stub.GenerateVideo(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.wait(ctx, started.GetRequestId())
}

// Extend continues an existing video and polls until it finishes.
func (s *Service) Extend(ctx context.Context, req *xaiapi.ExtendVideoRequest) (*xaiapi.VideoResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	s.conn.Logger.InfoContext(ctx, "video extend started", "model", req.GetModel())
	started, err := s.stub.ExtendVideo(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.wait(ctx, started.GetRequestId())
}

// Start returns the request id for a video generation without waiting.
func (s *Service) Start(ctx context.Context, req *xaiapi.GenerateVideoRequest) (string, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	started, err := s.stub.GenerateVideo(ctx, req)
	if err != nil {
		return "", err
	}
	return started.GetRequestId(), nil
}

// Get fetches the current state of a deferred video.
func (s *Service) Get(ctx context.Context, requestID string) (*xaiapi.GetDeferredVideoResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.GetDeferredVideo(ctx, &xaiapi.GetDeferredVideoRequest{RequestId: requestID})
}

const (
	pollInterval = time.Second
	pollBudget   = 10 * time.Minute
)

func (s *Service) wait(ctx context.Context, requestID string) (*xaiapi.VideoResponse, error) {
	if requestID == "" {
		return nil, errors.New("xai: video request id is empty")
	}
	pollCtx, pollCancel := context.WithTimeout(ctx, pollBudget)
	defer pollCancel()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		resp, err := s.pollOnce(pollCtx, requestID)
		if err != nil {
			return nil, err
		}
		s.conn.Logger.DebugContext(pollCtx, "video poll", "request_id", requestID, "status", resp.GetStatus().String(), "progress", resp.GetResponse().GetProgress())
		switch resp.GetStatus() {
		case xaiapi.DeferredStatus_DONE:
			if resp.Response == nil {
				return nil, errors.New("xai: video completed without a response")
			}
			s.conn.Logger.InfoContext(pollCtx, "video ready", "request_id", requestID)
			return resp.Response, nil
		case xaiapi.DeferredStatus_FAILED:
			msg := "video generation failed"
			if resp.Response != nil && resp.Response.Error != nil && resp.Response.Error.Message != "" {
				msg = resp.Response.Error.Message
			}
			s.conn.Logger.ErrorContext(pollCtx, "video failed", "request_id", requestID, "error", msg)
			return resp.GetResponse(), fmt.Errorf("xai: %s", msg)
		case xaiapi.DeferredStatus_EXPIRED:
			s.conn.Logger.WarnContext(pollCtx, "video result expired", "request_id", requestID)
			return nil, errors.New("xai: video result expired")
		}
		select {
		case <-pollCtx.Done():
			return nil, pollCtx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Service) pollOnce(pollCtx context.Context, requestID string) (*xaiapi.GetDeferredVideoResponse, error) {
	timeout := s.conn.Timeout
	if timeout <= 0 {
		timeout = rpc.DefaultTimeout
	}
	callCtx, cancel := context.WithTimeout(context.WithoutCancel(pollCtx), timeout)
	stop := context.AfterFunc(pollCtx, cancel)
	defer stop()
	defer cancel()
	resp, err := s.stub.GetDeferredVideo(callCtx, &xaiapi.GetDeferredVideoRequest{RequestId: requestID})
	if err != nil && pollCtx.Err() != nil {
		return nil, pollCtx.Err()
	}
	return resp, err
}
