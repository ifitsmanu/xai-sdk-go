package manage

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
)

// Collections manages collections on the management host.
// Search stays on the inference API, client.Documents.
type Collections struct {
	c *Client
}

// Collection is collection metadata. Unknown fields stay in Raw.
type Collection struct {
	ID          string          `json:"collection_id"`
	Name        string          `json:"collection_name"`
	Description string          `json:"collection_description"`
	Raw         json.RawMessage `json:"-"`
}

// CreateCollection is the POST /v1/collections body.
// ModelName is the embedding model. Leave it empty for the server default.
type CreateCollection struct {
	TeamID         string  `json:"team_id,omitempty"`
	CollectionName string  `json:"collection_name"`
	ModelName      string  `json:"-"`
	MetricSpace    string  `json:"metric_space,omitempty"`
	Description    string  `json:"collection_description,omitempty"`
	Fields         []Field `json:"field_definitions,omitempty"`
}

// Field is one custom metadata field.
type Field struct {
	Key             string `json:"key"`
	Required        bool   `json:"required,omitempty"`
	InjectIntoChunk bool   `json:"inject_into_chunk,omitempty"`
	Unique          bool   `json:"unique,omitempty"`
	Description     string `json:"description,omitempty"`
}

type createBody struct {
	TeamID         string `json:"team_id,omitempty"`
	CollectionName string `json:"collection_name"`
	Index          *struct {
		ModelName string `json:"model_name,omitempty"`
	} `json:"index_configuration,omitempty"`
	MetricSpace string  `json:"metric_space,omitempty"`
	Description string  `json:"collection_description,omitempty"`
	Fields      []Field `json:"field_definitions,omitempty"`
}

// Create creates a collection.
func (s *Collections) Create(ctx context.Context, req CreateCollection) (*Collection, error) {
	body := createBody{
		TeamID:         req.TeamID,
		CollectionName: req.CollectionName,
		MetricSpace:    req.MetricSpace,
		Description:    req.Description,
		Fields:         req.Fields,
	}
	if req.ModelName != "" {
		body.Index = &struct {
			ModelName string `json:"model_name,omitempty"`
		}{ModelName: req.ModelName}
	}
	return s.decode(ctx, http.MethodPost, "/v1/collections", body)
}

// List returns GET /v1/collections.
func (s *Collections) List(ctx context.Context) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, "/v1/collections", nil)
}

// Get returns one collection.
func (s *Collections) Get(ctx context.Context, id string) (*Collection, error) {
	return s.decode(ctx, http.MethodGet, rest.Path("/v1/collections", id), nil)
}

// Delete deletes a collection.
func (s *Collections) Delete(ctx context.Context, id string) error {
	return s.c.rest.Call(ctx, http.MethodDelete, rest.Path("/v1/collections", id), nil, nil)
}

// Update sends PUT /v1/collections/{id}. body is the documented update object.
func (s *Collections) Update(ctx context.Context, id string, body any) (*Collection, error) {
	return s.decode(ctx, http.MethodPut, rest.Path("/v1/collections", id), body)
}

// AddDocument sends POST /v1/collections/{id}/documents/{fileID}.
func (s *Collections) AddDocument(ctx context.Context, collectionID, fileID string, body any) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodPost, rest.Path("/v1/collections", collectionID, "documents", fileID), body)
}

// Documents lists documents in a collection.
func (s *Collections) Documents(ctx context.Context, collectionID string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/v1/collections", collectionID, "documents"), nil)
}

// Document returns one document.
func (s *Collections) Document(ctx context.Context, collectionID, fileID string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/v1/collections", collectionID, "documents", fileID), nil)
}

// UpdateDocument sends PATCH /v1/collections/{id}/documents/{fileID}.
func (s *Collections) UpdateDocument(ctx context.Context, collectionID, fileID string, body any) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodPatch, rest.Path("/v1/collections", collectionID, "documents", fileID), body)
}

// RemoveDocument removes a document from a collection.
func (s *Collections) RemoveDocument(ctx context.Context, collectionID, fileID string) error {
	return s.c.rest.Call(ctx, http.MethodDelete, rest.Path("/v1/collections", collectionID, "documents", fileID), nil, nil)
}

// BatchGetDocuments sends GET /v1/collections/{id}/documents:batchGet.
func (s *Collections) BatchGetDocuments(ctx context.Context, collectionID string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/v1/collections", collectionID, "documents:batchGet"), nil)
}

func (s *Collections) decode(ctx context.Context, method, path string, in any) (*Collection, error) {
	raw, err := s.raw(ctx, method, path, in)
	if err != nil {
		return nil, err
	}
	var out Collection
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	out.Raw = raw
	return &out, nil
}

func (s *Collections) raw(ctx context.Context, method, path string, in any) (json.RawMessage, error) {
	var out json.RawMessage
	if err := s.c.rest.Call(ctx, method, path, in, &out); err != nil {
		return nil, err
	}
	return out, nil
}
