// Package x is a client for the X API v2 and a set of agent tools built on it.
//
// Authenticate with an oauth2.TokenSource: an app bearer token for public reads
// (Bearer), or a user token from the OAuth 2.0 PKCE flow (OAuthConfig) for
// acting as that user. Tokens refresh through golang.org/x/oauth2; SaveTokens
// keeps refreshed tokens in a file only the owner can read.
//
// For search, prefer the xAI x_search server tool (tool.XSearch); it needs no X key.
package x

import (
	"cmp"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ifitsmanu/xai-sdk-go/internal/rest"
	"golang.org/x/oauth2"
)

// DefaultBase is the X API origin.
const DefaultBase = "https://api.x.com"

// Endpoint is X's OAuth 2.0 endpoint, as X's own CLI uses it.
var Endpoint = oauth2.Endpoint{
	AuthURL:  "https://x.com/i/oauth2/authorize",
	TokenURL: "https://api.x.com/2/oauth2/token",
}

// Scopes for reading and acting as the user. offline.access returns a refresh token.
var (
	ReadScopes  = []string{"tweet.read", "users.read", "bookmark.read", "like.read", "follows.read", "offline.access"}
	WriteScopes = []string{"tweet.write", "like.write", "follows.write", "bookmark.write", "dm.write", "dm.read", "media.write"}
)

// OAuthConfig is the PKCE flow for an X app. Use it with oauth2.GenerateVerifier,
// AuthCodeURL with oauth2.S256ChallengeOption, and Exchange with oauth2.VerifierOption.
// An empty secret is a public client.
func OAuthConfig(clientID, secret, redirectURL string, scopes ...string) *oauth2.Config {
	return &oauth2.Config{ClientID: clientID, ClientSecret: secret, RedirectURL: redirectURL, Endpoint: Endpoint, Scopes: scopes}
}

// Bearer is an app-only token for public reads.
func Bearer(token string) oauth2.TokenSource {
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token, TokenType: "Bearer"})
}

// SaveTokens returns a source that writes every new token to path with mode 0600.
func SaveTokens(path string, src oauth2.TokenSource) oauth2.TokenSource {
	return &savingSource{path: path, src: src}
}

// LoadToken reads a token written by SaveTokens.
func LoadToken(path string) (*oauth2.Token, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t oauth2.Token
	return &t, json.Unmarshal(raw, &t)
}

type savingSource struct {
	mu   sync.Mutex
	path string
	src  oauth2.TokenSource
	last string
}

func (s *savingSource) Token() (*oauth2.Token, error) {
	t, err := s.src.Token()
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t.AccessToken != s.last {
		raw, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(s.path, raw, 0o600); err != nil {
			return nil, err
		}
		s.last = t.AccessToken
	}
	return t, nil
}

// Client calls the X API.
type Client struct {
	rest *rest.Client
}

// Options configure a Client. Zero values use the defaults.
type Options struct {
	Base     string        // DefaultBase
	Timeout  time.Duration // per call when the context has no deadline
	Attempts int           // tries for GET and DELETE; DefaultAttempts when zero, 1 turns retries off
	MaxWait  time.Duration // longest wait for a rate limit before returning the error
}

// DefaultAttempts is how many times a GET or DELETE is tried by default.
const DefaultAttempts = 3

// New returns a client that authenticates every call with tokens.
func New(tokens oauth2.TokenSource, opts Options) *Client {
	base := opts.Base
	if base == "" {
		base = DefaultBase
	}
	httpClient := oauth2.NewClient(context.Background(), oauth2.ReuseTokenSource(nil, tokens))
	return &Client{rest: &rest.Client{
		Base: base, HTTP: httpClient, UserAgent: "xai-sdk-go",
		Timeout: opts.Timeout, Attempts: cmp.Or(opts.Attempts, DefaultAttempts), MaxWait: opts.MaxWait,
	}}
}

// User is an X account.
type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

// Post is one post. Fields beyond ID and Text appear when requested with Fields.
type Post struct {
	ID              string `json:"id"`
	Text            string `json:"text"`
	AuthorID        string `json:"author_id,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	ConversationID  string `json:"conversation_id,omitempty"`
	InReplyToUserID string `json:"in_reply_to_user_id,omitempty"`
	PublicMetrics   *struct {
		Reposts int `json:"retweet_count"`
		Replies int `json:"reply_count"`
		Likes   int `json:"like_count"`
		Quotes  int `json:"quote_count"`
	} `json:"public_metrics,omitempty"`
}

// Page is one page of posts. Pass Next back as Query.PageToken for the next page.
type Page struct {
	Posts []Post
	Users []User // authors, when Query.Expand includes author_id
	Next  string
}

// Query selects posts. Zero values use X's defaults.
type Query struct {
	Max        int // 5 to 100 for search; 5 to 100 for timelines
	PageToken  string
	PostFields []string // such as created_at, author_id, public_metrics
	Expand     []string // such as author_id
}

func (q Query) values(pageKey string) url.Values {
	v := url.Values{pageKey: {q.PageToken}}
	if q.Max > 0 {
		v.Set("max_results", strconv.Itoa(q.Max))
	}
	if len(q.PostFields) > 0 {
		v.Set("tweet.fields", strings.Join(q.PostFields, ","))
	}
	if len(q.Expand) > 0 {
		v.Set("expansions", strings.Join(q.Expand, ","))
	}
	return v
}

// Me is the user the token acts for. It needs a user token.
func (c *Client) Me(ctx context.Context) (*User, error) {
	return data[User](ctx, c, http.MethodGet, "/2/users/me", nil)
}

// UserByUsername looks up an account by handle, without the @.
func (c *Client) UserByUsername(ctx context.Context, username string) (*User, error) {
	return data[User](ctx, c, http.MethodGet, rest.Path("/2/users/by/username", strings.TrimPrefix(username, "@")), nil)
}

// Post fetches one post.
func (c *Client) Post(ctx context.Context, id string, q Query) (*Post, error) {
	return data[Post](ctx, c, http.MethodGet, rest.Query(rest.Path("/2/tweets", id), q.values("")), nil)
}

// Search returns recent posts (last 7 days) matching query.
func (c *Client) Search(ctx context.Context, query string, q Query) (*Page, error) {
	v := q.values("next_token")
	v.Set("query", query)
	return c.page(ctx, rest.Query("/2/tweets/search/recent", v))
}

// UserPosts lists a user's posts, newest first.
func (c *Client) UserPosts(ctx context.Context, userID string, q Query) (*Page, error) {
	return c.page(ctx, rest.Query(rest.Path("/2/users", userID, "tweets"), q.values("pagination_token")))
}

// Mentions lists posts that mention a user, newest first.
func (c *Client) Mentions(ctx context.Context, userID string, q Query) (*Page, error) {
	return c.page(ctx, rest.Query(rest.Path("/2/users", userID, "mentions"), q.values("pagination_token")))
}

// NewPost is a post to create. Set at most one of QuoteID and MediaIDs.
type NewPost struct {
	Text      string
	ReplyToID string
	QuoteID   string
	MediaIDs  []string
}

// CreatePost publishes a post as the user and returns it. It is never retried.
func (c *Client) CreatePost(ctx context.Context, p NewPost) (*Post, error) {
	body := map[string]any{"text": p.Text}
	if p.ReplyToID != "" {
		body["reply"] = map[string]string{"in_reply_to_tweet_id": p.ReplyToID}
	}
	if p.QuoteID != "" {
		body["quote_tweet_id"] = p.QuoteID
	}
	if len(p.MediaIDs) > 0 {
		body["media"] = map[string]any{"media_ids": p.MediaIDs}
	}
	return data[Post](ctx, c, http.MethodPost, "/2/tweets", body)
}

// DeletePost deletes one of the user's posts.
func (c *Client) DeletePost(ctx context.Context, id string) error {
	return c.rest.Call(ctx, http.MethodDelete, rest.Path("/2/tweets", id), nil, nil)
}

// Like likes a post as userID (usually Me().ID).
func (c *Client) Like(ctx context.Context, userID, postID string) error {
	return c.rest.Call(ctx, http.MethodPost, rest.Path("/2/users", userID, "likes"), map[string]string{"tweet_id": postID}, nil)
}

// Repost reposts a post as userID.
func (c *Client) Repost(ctx context.Context, userID, postID string) error {
	return c.rest.Call(ctx, http.MethodPost, rest.Path("/2/users", userID, "retweets"), map[string]string{"tweet_id": postID}, nil)
}

// Bookmark saves a post to userID's bookmarks.
func (c *Client) Bookmark(ctx context.Context, userID, postID string) error {
	return c.rest.Call(ctx, http.MethodPost, rest.Path("/2/users", userID, "bookmarks"), map[string]string{"tweet_id": postID}, nil)
}

// Follow follows targetID as userID.
func (c *Client) Follow(ctx context.Context, userID, targetID string) error {
	return c.rest.Call(ctx, http.MethodPost, rest.Path("/2/users", userID, "following"), map[string]string{"target_user_id": targetID}, nil)
}

// SendDM sends a direct message to a user.
func (c *Client) SendDM(ctx context.Context, participantID, text string) error {
	return c.rest.Call(ctx, http.MethodPost, rest.Path("/2/dm_conversations/with", participantID, "messages"), map[string]string{"text": text}, nil)
}

func data[T any](ctx context.Context, c *Client, method, path string, in any) (*T, error) {
	var out struct {
		Data T `json:"data"`
	}
	if err := c.rest.Call(ctx, method, path, in, &out); err != nil {
		return nil, err
	}
	return &out.Data, nil
}

func (c *Client) page(ctx context.Context, path string) (*Page, error) {
	var out struct {
		Data     []Post `json:"data"`
		Includes struct {
			Users []User `json:"users"`
		} `json:"includes"`
		Meta struct {
			NextToken string `json:"next_token"`
		} `json:"meta"`
	}
	if err := c.rest.Call(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return &Page{Posts: out.Data, Users: out.Includes.Users, Next: out.Meta.NextToken}, nil
}
