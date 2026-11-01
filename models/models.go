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
