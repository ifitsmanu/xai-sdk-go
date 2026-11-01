// Package files uploads and manages files.
package files

import (
	"context"
	"errors"
	"io"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
)

// Service is the files API.
type Service struct {
	conn *rpc.Conn
	stub xaiapi.FilesClient
}

// NewService binds the files API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn, stub: xaiapi.NewFilesClient(conn.GRPC)}
}

// UploadOptions controls an upload.
type UploadOptions struct {
	// ExpiresAfter is a TTL in seconds. Zero means the server default (no expiry).
	ExpiresAfter int64
}

const fileChunkSize = 4 << 20

// Upload streams r to the Files API. name is the filename stored with the file.
func (s *Service) Upload(ctx context.Context, name string, r io.Reader, opt *UploadOptions) (*xaiapi.File, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	s.conn.Logger.InfoContext(ctx, "file upload started", "file.name", name)
	stream, err := s.stub.UploadFile(ctx)
	if err != nil {
		return nil, err
	}
	init := &xaiapi.UploadFileInit{Name: name}
	if opt != nil && opt.ExpiresAfter > 0 {
		ttl := opt.ExpiresAfter
		init.ExpiresAfter = &ttl
	}
	if err := stream.Send(&xaiapi.UploadFileChunk{Chunk: &xaiapi.UploadFileChunk_Init{Init: init}}); err != nil {
		return nil, err
	}
	buf := make([]byte, fileChunkSize)
	var chunks int
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			chunks++
			if err := stream.Send(&xaiapi.UploadFileChunk{Chunk: &xaiapi.UploadFileChunk_Data{Data: buf[:n]}}); err != nil {
				return nil, err
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return nil, rerr
		}
	}
	file, err := stream.CloseAndRecv()
	if err != nil {
		s.conn.Logger.ErrorContext(ctx, "file upload failed", "file.name", name, "error", err.Error())
		return nil, err
	}
	s.conn.Logger.InfoContext(ctx, "file upload finished", "file.id", file.GetId(), "file.bytes", file.GetSize(), "chunks", chunks)
	return file, nil
}

// List returns a page of file metadata.
func (s *Service) List(ctx context.Context, req *xaiapi.ListFilesRequest) (*xaiapi.ListFilesResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	if req == nil {
		req = &xaiapi.ListFilesRequest{}
	}
	return s.stub.ListFiles(ctx, req)
}

// Get returns metadata for one file.
func (s *Service) Get(ctx context.Context, id string) (*xaiapi.File, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.RetrieveFile(ctx, &xaiapi.RetrieveFileRequest{FileId: id})
}

// Delete removes a file.
func (s *Service) Delete(ctx context.Context, id string) (*xaiapi.DeleteFileResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	s.conn.Logger.InfoContext(ctx, "file delete", "file.id", id)
	return s.stub.DeleteFile(ctx, &xaiapi.DeleteFileRequest{FileId: id})
}

// Download concatenates the content stream for a file.
func (s *Service) Download(ctx context.Context, id string) ([]byte, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	s.conn.Logger.DebugContext(ctx, "file download", "file.id", id)
	stream, err := s.stub.RetrieveFileContent(ctx, &xaiapi.RetrieveFileContentRequest{FileId: id})
	if err != nil {
		return nil, err
	}
	var out []byte
	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, chunk.GetData()...)
	}
}

// CreatePublicURL creates or returns the public URL for a file.
func (s *Service) CreatePublicURL(ctx context.Context, req *xaiapi.CreatePublicUrlRequest) (*xaiapi.CreatePublicUrlResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.CreatePublicUrl(ctx, req)
}

// RevokePublicURL revokes a file's public URL.
func (s *Service) RevokePublicURL(ctx context.Context, id string) (*xaiapi.RevokePublicUrlResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.RevokePublicUrl(ctx, &xaiapi.RevokePublicUrlRequest{FileId: id})
}
