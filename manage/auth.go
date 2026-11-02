package manage

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
)

// Auth manages API keys and the model and endpoint names used in ACLs.
type Auth struct {
	c *Client
}

// CreateKey is POST /auth/teams/{teamID}/api-keys.
// ACLs use the strings from Models and Endpoints. The all-access values are api-key:model:** and api-key:endpoint:**.
type CreateKey struct {
	Name       string   `json:"name"`
	ACLs       []string `json:"acls,omitempty"`
	QPS        *int     `json:"qps,omitempty"`
	QPM        *int     `json:"qpm,omitempty"`
	TPM        string   `json:"tpm,omitempty"`
	ExpireTime string   `json:"expireTime,omitempty"`
}

// Key is an API key record. APIKey is set only on create. Do not log it.
type Key struct {
	APIKey         string `json:"apiKey,omitempty"`
	RedactedAPIKey string `json:"redactedApiKey,omitempty"`
	Name           string `json:"name,omitempty"`
	UserID         string `json:"userId,omitempty"`
	CreateTime     string `json:"createTime,omitempty"`
	ModifyTime     string `json:"modifyTime,omitempty"`
}

// CreateKey creates an API key. The full secret is only present on this response.
func (s *Auth) CreateKey(ctx context.Context, teamID string, req CreateKey) (*Key, error) {
	var out Key
	if err := s.c.rest.Call(ctx, http.MethodPost, rest.Path("/auth/teams", teamID, "api-keys"), req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Keys lists API keys for a team. Secrets are redacted.
func (s *Auth) Keys(ctx context.Context, teamID string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/auth/teams", teamID, "api-keys"), nil)
}

// UpdateKey sends PUT /auth/api-keys/{id}.
func (s *Auth) UpdateKey(ctx context.Context, id string, body any) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodPut, rest.Path("/auth/api-keys", id), body)
}

// RotateKey sends POST /auth/api-keys/{id}/rotate.
func (s *Auth) RotateKey(ctx context.Context, id string) (*Key, error) {
	var out Key
	if err := s.c.rest.Call(ctx, http.MethodPost, rest.Path("/auth/api-keys", id, "rotate"), map[string]any{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteKey deletes an API key.
func (s *Auth) DeleteKey(ctx context.Context, id string) error {
	return s.c.rest.Call(ctx, http.MethodDelete, rest.Path("/auth/api-keys", id), nil, nil)
}

// Propagation calls GET /auth/api-keys/{id}/propagation.
func (s *Auth) Propagation(ctx context.Context, id string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/auth/api-keys", id, "propagation"), nil)
}

// Models lists the model ACL names for a team.
func (s *Auth) Models(ctx context.Context, teamID string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/auth/teams", teamID, "models"), nil)
}

// Endpoints lists the endpoint ACL names for a team.
func (s *Auth) Endpoints(ctx context.Context, teamID string) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, rest.Path("/auth/teams", teamID, "endpoints"), nil)
}

// Validate calls GET /auth/management-keys/validation.
func (s *Auth) Validate(ctx context.Context) (json.RawMessage, error) {
	return s.raw(ctx, http.MethodGet, "/auth/management-keys/validation", nil)
}

func (s *Auth) raw(ctx context.Context, method, path string, in any) (json.RawMessage, error) {
	var out json.RawMessage
	if err := s.c.rest.Call(ctx, method, path, in, &out); err != nil {
		return nil, err
	}
	return out, nil
}
