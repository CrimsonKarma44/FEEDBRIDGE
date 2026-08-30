package utility

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
)

func TestRetryableFeedError(t *testing.T) {
	if RetryableFeedError(errors.New("nope")) {
		t.Fatal("plain error should not retry")
	}
	if RetryableFeedError(gofeed.HTTPError{StatusCode: 404, Status: "404 Not Found"}) {
		t.Fatal("404 should not retry")
	}
	if !RetryableFeedError(gofeed.HTTPError{StatusCode: 500, Status: "500 Internal Server Error"}) {
		t.Fatal("500 should retry")
	}
	if !RetryableFeedError(gofeed.HTTPError{StatusCode: 429, Status: "429 Too Many Requests"}) {
		t.Fatal("429 should retry")
	}
}

func TestFeedParserRetriesThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	atom := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>T</title>
  <entry><title>One</title><link href="https://example.com/1"/></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua != FeedUserAgent {
			t.Errorf("User-Agent = %q", ua)
		}
		if acc := r.Header.Get("Accept"); acc == "" {
			t.Error("missing Accept")
		}
		n := hits.Add(1)
		if n < 2 {
			http.Error(w, "nope", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(atom))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	feed, err := FeedParser(ctx, srv.URL)
	if err != nil {
		t.Fatalf("FeedParser: %v", err)
	}
	if feed.Title != "T" || len(feed.Items) != 1 {
		t.Fatalf("got title=%q items=%d", feed.Title, len(feed.Items))
	}
	if hits.Load() < 2 {
		t.Fatalf("hits=%d, want retry", hits.Load())
	}
}
