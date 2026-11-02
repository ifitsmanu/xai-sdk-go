package x

import (
	"context"
	"sync"

	"github.com/ifitsmanu/xai-sdk-go/tool"
)

// ToolOptions choose which X tools an agent gets.
type ToolOptions struct {
	// Writes adds tools that post, delete, like, repost, bookmark, follow, and
	// send DMs. They are marked Writes, so a runner asks before each call.
	Writes bool
}

type postID struct {
	ID string `json:"id" jsonschema:"post id"`
}

type handle struct {
	Username string `json:"username" jsonschema:"handle without the @"`
}

type listing struct {
	UserID string `json:"user_id" jsonschema:"numeric user id"`
	Max    int    `json:"max,omitempty" jsonschema:"results to return, 5 to 100"`
}

type search struct {
	Query string `json:"query" jsonschema:"X search query, such as from:xai grok -is:retweet"`
	Max   int    `json:"max,omitempty" jsonschema:"results to return, 10 to 100"`
}

type newPost struct {
	Text      string `json:"text" jsonschema:"post text"`
	ReplyToID string `json:"reply_to_id,omitempty" jsonschema:"post id to reply to"`
	QuoteID   string `json:"quote_id,omitempty" jsonschema:"post id to quote"`
}

type follow struct {
	UserID string `json:"user_id" jsonschema:"numeric id of the account to follow"`
}

type dm struct {
	UserID string `json:"user_id" jsonschema:"numeric id of the recipient"`
	Text   string `json:"text"`
}

// Tools returns X API operations as agent tools. Results are JSON.
func Tools(c *Client, opts ToolOptions) ([]tool.Tool, error) {
	fields := Query{PostFields: []string{"created_at", "author_id", "public_metrics"}, Expand: []string{"author_id"}}
	page := func(max int) Query { q := fields; q.Max = max; return q }
	me := sync.OnceValues(func() (*User, error) { return c.Me(context.Background()) })
	self := func() (string, error) {
		u, err := me()
		if err != nil {
			return "", err
		}
		return u.ID, nil
	}

	var tools []tool.Tool
	var firstErr error
	add := func(f *tool.Function, err error, writes bool) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if f != nil {
			f.Writes = writes
			tools = append(tools, f)
		}
	}
	read := func(f *tool.Function, err error) { add(f, err, false) }
	write := func(f *tool.Function, err error) { add(f, err, true) }
	read(tool.Func("x_me", "The X account these tools act as.", func(context.Context, struct{}) (*User, error) { return me() }))
	read(tool.Func("x_user", "Look up an X account by handle.", func(ctx context.Context, in handle) (*User, error) { return c.UserByUsername(ctx, in.Username) }))
	read(tool.Func("x_post", "Read one X post by id.", func(ctx context.Context, in postID) (*Post, error) { return c.Post(ctx, in.ID, fields) }))
	read(tool.Func("x_search_recent", "Search X posts from the last 7 days.", func(ctx context.Context, in search) (*Page, error) {
		return c.Search(ctx, in.Query, page(in.Max))
	}))
	read(tool.Func("x_user_posts", "A user's recent posts.", func(ctx context.Context, in listing) (*Page, error) {
		return c.UserPosts(ctx, in.UserID, page(in.Max))
	}))
	read(tool.Func("x_mentions", "Recent posts that mention a user.", func(ctx context.Context, in listing) (*Page, error) {
		return c.Mentions(ctx, in.UserID, page(in.Max))
	}))
	if !opts.Writes {
		return tools, firstErr
	}
	write(tool.Func("x_create_post", "Publish a post, reply, or quote as the user.", func(ctx context.Context, in newPost) (*Post, error) {
		return c.CreatePost(ctx, NewPost{Text: in.Text, ReplyToID: in.ReplyToID, QuoteID: in.QuoteID})
	}))
	write(tool.Func("x_delete_post", "Delete one of the user's posts.", func(ctx context.Context, in postID) (string, error) {
		return "deleted", c.DeletePost(ctx, in.ID)
	}))
	act := func(name, desc string, do func(context.Context, string, string) error) {
		write(tool.Func(name, desc, func(ctx context.Context, in postID) (string, error) {
			id, err := self()
			if err != nil {
				return "", err
			}
			return "done", do(ctx, id, in.ID)
		}))
	}
	act("x_like", "Like a post as the user.", c.Like)
	act("x_repost", "Repost a post as the user.", c.Repost)
	act("x_bookmark", "Bookmark a post for the user.", c.Bookmark)
	write(tool.Func("x_follow", "Follow an account as the user.", func(ctx context.Context, in follow) (string, error) {
		id, err := self()
		if err != nil {
			return "", err
		}
		return "following", c.Follow(ctx, id, in.UserID)
	}))
	write(tool.Func("x_send_dm", "Send a direct message as the user.", func(ctx context.Context, in dm) (string, error) {
		return "sent", c.SendDM(ctx, in.UserID, in.Text)
	}))
	return tools, firstErr
}
