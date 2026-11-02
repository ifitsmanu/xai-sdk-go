// Package skills is the REST skills API at /v1/skills.
package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
	"github.com/ifitsmanu/xai-sdk-go/internal/rpc"
)

// Service is the skills API.
type Service struct {
	conn *rpc.Conn
}

// NewService binds the skills API to a connection.
func NewService(conn *rpc.Conn) *Service {
	return &Service{conn: conn}
}

// Skill is one uploaded skill.
type Skill struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	CreatedAt      int64  `json:"created_at"`
	DefaultVersion string `json:"default_version"`
	LatestVersion  string `json:"latest_version"`
	Object         string `json:"object"`
}

// Page is one page of skills.
type Page struct {
	Data    []Skill `json:"data"`
	FirstID string  `json:"first_id"`
	LastID  string  `json:"last_id"`
	HasMore bool    `json:"has_more"`
	Object  string  `json:"object"`
}

// List returns one page. limit 0, an empty after, and an empty order use the server default.
func (s *Service) List(ctx context.Context, limit int, after, order string) (*Page, error) {
	q := url.Values{"after": {after}, "order": {order}}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	var page Page
	if err := s.conn.CallJSON(ctx, http.MethodGet, rest.Query("/v1/skills", q), nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Get returns one skill.
func (s *Service) Get(ctx context.Context, id string) (*Skill, error) {
	var skill Skill
	if err := s.conn.CallJSON(ctx, http.MethodGet, rest.Path("/v1/skills", id), nil, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

// Deleted is the body of a successful delete.
type Deleted struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
	Object  string `json:"object"`
}

// Delete removes a skill.
func (s *Service) Delete(ctx context.Context, id string) (*Deleted, error) {
	var out Deleted
	if err := s.conn.CallJSON(ctx, http.MethodDelete, rest.Path("/v1/skills", id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Content downloads the skill file bytes.
func (s *Service) Content(ctx context.Context, id string) ([]byte, error) {
	return s.conn.RESTClient().Do(ctx, http.MethodGet, rest.Path("/v1/skills", id, "content"), nil)
}

// File is one part of a skill upload.
type File struct {
	Name string
	Data []byte
}

// Upload sends skill files as multipart form parts named files.
func (s *Service) Upload(ctx context.Context, files []File) (*Skill, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, file := range files {
		part, err := w.CreateFormFile("files", file.Name)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(file.Data); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	raw, _, err := s.conn.RESTClient().Read(ctx, http.MethodPost, "/v1/skills", &rest.Body{Reader: &buf, ContentType: w.FormDataContentType()})
	if err != nil {
		return nil, err
	}
	var skill Skill
	if err := json.Unmarshal(raw, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}
