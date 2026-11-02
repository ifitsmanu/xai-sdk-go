// Package skills is the REST skills API at /v1/skills.
package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
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
	path := "/v1/skills"
	q := ""
	add := func(k, v string) {
		if v == "" {
			return
		}
		if q == "" {
			q = "?"
		} else {
			q += "&"
		}
		q += k + "=" + v
	}
	if limit > 0 {
		add("limit", strconv.Itoa(limit))
	}
	add("after", after)
	add("order", order)
	var page Page
	if err := s.conn.CallJSON(ctx, http.MethodGet, path+q, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Get returns one skill.
func (s *Service) Get(ctx context.Context, id string) (*Skill, error) {
	var skill Skill
	if err := s.conn.CallJSON(ctx, http.MethodGet, "/v1/skills/"+id, nil, &skill); err != nil {
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
	if err := s.conn.CallJSON(ctx, http.MethodDelete, "/v1/skills/"+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Content downloads the skill file bytes.
func (s *Service) Content(ctx context.Context, id string) ([]byte, error) {
	return s.conn.RESTClient().Do(ctx, http.MethodGet, "/v1/skills/"+id+"/content", nil)
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
	raw, err := s.postForm(ctx, "/v1/skills", w.FormDataContentType(), &buf)
	if err != nil {
		return nil, err
	}
	var skill Skill
	if err := json.Unmarshal(raw, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

func (s *Service) postForm(ctx context.Context, path, contentType string, body io.Reader) ([]byte, error) {
	ctx, cancel := s.conn.Context(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.conn.RESTBase+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.conn.APIKey)
	req.Header.Set("Content-Type", contentType)
	if s.conn.UserAgent != "" {
		req.Header.Set("User-Agent", s.conn.UserAgent)
	}
	client := s.conn.RESTClient().HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &rest.Error{Method: http.MethodPost, Path: path, Status: resp.StatusCode, Body: string(raw)}
	}
	return raw, nil
}
