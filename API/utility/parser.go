package utility

import (
	"context"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

const fetchTimeout = 20 * time.Second

var feedHTTPClient = &http.Client{Timeout: fetchTimeout}

func FeedParser(ctx context.Context, url string) (*gofeed.Feed, error) {
	fp := gofeed.NewParser()
	fp.Client = feedHTTPClient
	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, err
	}
	return feed, nil
}
