package utility

import (
	"context"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

func FeedParser(ctx context.Context, url string) (*gofeed.Feed, error) {
	fp := gofeed.NewParser()
	fp.Client = &http.Client{Timeout: 20 * time.Second}
	feed, err := fp.ParseURLWithContext(url, ctx)
	if err != nil {
		return nil, err
	}
	return feed, nil
}
