package storage

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/api/option"
)

// TestGCSReadFileLimited drives GCSClient.ReadFileLimited against a fake
// endpoint that honours HTTP Range requests, and checks the client asks for
// at most max+1 bytes and enforces the cap.
func TestGCSReadFileLimited(t *testing.T) {
	body := bytes.Repeat([]byte("x"), 1000)
	var lastRange atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastRange.Store(r.Header.Get("Range"))
		http.ServeContent(w, r, "obj", time.Time{}, bytes.NewReader(body))
	}))
	defer srv.Close()

	ctx := context.Background()
	c, err := NewGCSClient(ctx, option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ReadFileLimited(ctx, "bucket", "obj", 100); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("over the cap: %v", err)
	}
	if r, _ := lastRange.Load().(string); r != "bytes=0-100" {
		t.Fatalf("Range header = %q, want bytes=0-100 (max+1 bytes)", r)
	}
	got, err := c.ReadFileLimited(ctx, "bucket", "obj", 5000)
	if err != nil || len(got) != len(body) {
		t.Fatalf("under the cap: %d bytes, %v", len(got), err)
	}
}
