package manage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
)

// Audit reads team audit events.
type Audit struct {
	c *Client
}

// Query is GET /audit/teams/{teamID}/events.
type Query struct {
	PageSize  int
	PageToken string
	UserID    string
	Query     string
	EventID   string
	From      string
	To        string
	OrderBy   string
}

// Events returns one page of audit events.
func (s *Audit) Events(ctx context.Context, teamID string, q Query) (json.RawMessage, error) {
	v := url.Values{}
	if q.PageSize > 0 {
		v.Set("pageSize", strconv.Itoa(q.PageSize))
	}
	if q.PageToken != "" {
		v.Set("pageToken", q.PageToken)
	}
	if q.UserID != "" {
		v.Set("eventFilter.userId", q.UserID)
	}
	if q.Query != "" {
		v.Set("eventFilter.query", q.Query)
	}
	if q.EventID != "" {
		v.Set("eventFilter.eventId", q.EventID)
	}
	if q.From != "" {
		v.Set("eventTimeFrom", q.From)
	}
	if q.To != "" {
		v.Set("eventTimeTo", q.To)
	}
	if q.OrderBy != "" {
		v.Set("orderBy", q.OrderBy)
	}
	path := "/audit/teams/" + teamID + "/events"
	if encoded := v.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var out json.RawMessage
	if err := s.c.Call(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
