// Package account is GET /v1/me.
package account

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
)

// Service reads the caller identity.
type Service struct {
	conn *rpc.Conn
}

// NewService binds the account call to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn}
}

// Me is the authenticated caller.
type Me struct {
	UserID      string `json:"user_id"`
	TeamID      string `json:"team_id"`
	ZDRStatus   string `json:"zdr_status"`
	TeamBlocked bool   `json:"team_blocked"`
	APIKey      *Key   `json:"api_key"`
	// OAuth is set when the caller used an OAuth token instead of an API key.
	OAuth json.RawMessage `json:"oauth,omitempty"`
}

// Key is the redacted key that made the call. The secret is not returned.
type Key struct {
	Redacted string `json:"redacted_api_key"`
	ID       string `json:"api_key_id"`
	Blocked  bool   `json:"blocked"`
	Disabled bool   `json:"disabled"`
}

// Me returns the caller behind the API key.
func (s *Service) Me(ctx context.Context) (*Me, error) {
	var me Me
	if err := s.conn.RESTClient().Call(ctx, http.MethodGet, "/v1/me", nil, &me); err != nil {
		return nil, err
	}
	return &me, nil
}
