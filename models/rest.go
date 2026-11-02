package models

import (
	"context"
	"net/http"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
)

// VideoModel is one video generation model from GET /v1/video-generation-models.
// The gRPC models API does not list video models.
type VideoModel struct {
	ID               string   `json:"id"`
	Fingerprint      string   `json:"fingerprint"`
	Created          int64    `json:"created"`
	Version          string   `json:"version"`
	Aliases          []string `json:"aliases"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
	OwnedBy          string   `json:"owned_by"`
}

// ListedModel is one row from GET /v1/models.
// That list includes language, image, video, and embedding models, plus prices.
// Token prices are USD cents per 100M tokens; 12500 is $1.25 per 1M tokens.
// Above LongContextThreshold prompt tokens the *LongContext prices apply.
type ListedModel struct {
	ID                                    string        `json:"id"`
	Aliases                               []string      `json:"aliases"`
	Created                               int64         `json:"created"`
	OwnedBy                               string        `json:"owned_by"`
	ContextLength                         int64         `json:"context_length"`
	Capabilities                          *Capabilities `json:"capabilities"`
	PromptTextTokenPrice                  int64         `json:"prompt_text_token_price"`
	CachedPromptTextTokenPrice            int64         `json:"cached_prompt_text_token_price"`
	PromptImageTokenPrice                 int64         `json:"prompt_image_token_price"`
	CompletionTextTokenPrice              int64         `json:"completion_text_token_price"`
	PromptTextTokenPriceLongContext       int64         `json:"prompt_text_token_price_long_context"`
	CachedPromptTextTokenPriceLongContext int64         `json:"cached_prompt_text_token_price_long_context"`
	CompletionTextTokenPriceLongContext   int64         `json:"completion_text_token_price_long_context"`
	LongContextThreshold                  int64         `json:"long_context_threshold"`
	ImagePrice                            int64         `json:"image_price"`
	Pricing                               []ImagePrice  `json:"pricing"`
}

// Capabilities lists the request settings a model accepts.
type Capabilities struct {
	ReasoningEffort        []string `json:"reasoning_effort"`
	DefaultReasoningEffort string   `json:"default_reasoning_effort"`
}

// ImagePrice is one image tier. Prices use the same units as ImagePrice on the model.
type ImagePrice struct {
	Quality       string `json:"quality"`
	Resolution    string `json:"resolution"`
	PricePerImage int64  `json:"price_per_image"`
}

// All lists every model this key can call, from the REST models API.
func (s *Service) All(ctx context.Context) ([]ListedModel, error) {
	var page struct {
		Data []ListedModel `json:"data"`
	}
	err := s.conn.RESTClient().Call(ctx, http.MethodGet, "/v1/models", nil, &page)
	return page.Data, err
}

// ListVideo lists the video generation models this key can call.
func (s *Service) ListVideo(ctx context.Context) ([]VideoModel, error) {
	var page struct {
		Models []VideoModel `json:"models"`
	}
	err := s.conn.RESTClient().Call(ctx, http.MethodGet, "/v1/video-generation-models", nil, &page)
	return page.Models, err
}

// Video returns one video generation model.
func (s *Service) Video(ctx context.Context, id string) (*VideoModel, error) {
	var model VideoModel
	return &model, s.conn.RESTClient().Call(ctx, http.MethodGet, rest.Path("/v1/video-generation-models", id), nil, &model)
}

// VideoNames lists video model ids in API order.
func (s *Service) VideoNames(ctx context.Context) ([]string, error) {
	listed, err := s.ListVideo(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(listed))
	for _, model := range listed {
		if model.ID != "" {
			out = append(out, model.ID)
		}
	}
	return out, nil
}
