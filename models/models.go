// Package models lists and fetches model metadata.
package models

import (
	"context"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
	"github.com/ifitsmanu/xai-sdk-go/xaiapi"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Service is the models API.
type Service struct {
	conn *rpc.Conn
	stub xaiapi.ModelsClient
}

// NewService binds the models API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn, stub: xaiapi.NewModelsClient(conn.GRPC)}
}

// LanguageNames lists the language model ids available to this key, in API order.
func (s *Service) LanguageNames(ctx context.Context) ([]string, error) {
	listed, err := s.ListLanguage(ctx)
	if err != nil {
		return nil, err
	}
	return ids(listed.GetModels()), nil
}

// ImageNames lists the image model ids available to this key, in API order.
func (s *Service) ImageNames(ctx context.Context) ([]string, error) {
	listed, err := s.ListImage(ctx)
	if err != nil {
		return nil, err
	}
	return ids(listed.GetModels()), nil
}

// EmbeddingNames lists the embedding model ids available to this key, in API order.
func (s *Service) EmbeddingNames(ctx context.Context) ([]string, error) {
	listed, err := s.ListEmbedding(ctx)
	if err != nil {
		return nil, err
	}
	return ids(listed.GetModels()), nil
}

// ListLanguage returns the language models available to the API key.
func (s *Service) ListLanguage(ctx context.Context) (*xaiapi.ListLanguageModelsResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.ListLanguageModels(ctx, &emptypb.Empty{})
}

// ListEmbedding returns the embedding models available to the API key.
func (s *Service) ListEmbedding(ctx context.Context) (*xaiapi.ListEmbeddingModelsResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.ListEmbeddingModels(ctx, &emptypb.Empty{})
}

// ListImage returns the image generation models available to the API key.
func (s *Service) ListImage(ctx context.Context) (*xaiapi.ListImageGenerationModelsResponse, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.ListImageGenerationModels(ctx, &emptypb.Empty{})
}

// Language returns one language model.
func (s *Service) Language(ctx context.Context, name string) (*xaiapi.LanguageModel, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.GetLanguageModel(ctx, &xaiapi.GetModelRequest{Name: name})
}

// Embedding returns one embedding model.
func (s *Service) Embedding(ctx context.Context, name string) (*xaiapi.EmbeddingModel, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.GetEmbeddingModel(ctx, &xaiapi.GetModelRequest{Name: name})
}

// Image returns one image generation model.
func (s *Service) Image(ctx context.Context, name string) (*xaiapi.ImageGenerationModel, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	return s.stub.GetImageGenerationModel(ctx, &xaiapi.GetModelRequest{Name: name})
}

func ids[T interface{ GetName() string }](items []T) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if name := item.GetName(); name != "" {
			out = append(out, name)
		}
	}
	return out
}
