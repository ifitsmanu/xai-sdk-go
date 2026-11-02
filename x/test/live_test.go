//go:build live

package x_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ifitsmanu/xai-sdk-go/x"
)

// Read-only: looks up @xai with an app bearer token. Posts nothing.
func TestLiveUserLookup(t *testing.T) {
	token := os.Getenv("X_BEARER_TOKEN")
	if token == "" {
		t.Skip("X_BEARER_TOKEN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	u, err := x.New(x.Bearer(token), x.Options{}).UserByUsername(ctx, "xai")
	if err != nil || u.ID == "" {
		t.Fatalf("user = %+v, %v", u, err)
	}
	t.Logf("@%s is %s (%s)", u.Username, u.ID, u.Name)
}
