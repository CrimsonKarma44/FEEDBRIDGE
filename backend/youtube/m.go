package youtube

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

func runNewResolver(rawURL string) {
	// Optional: export YOUTUBE_API_KEY=... to prefer the official Data API.
	r := NewResolver("")
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	// Step 1 — parse without network (see what kind of URL we have).
	parsed, err := ParseURL(rawURL)
	if err != nil {
		fmt.Println("parse error:", err)
		return
	}
	fmt.Printf("parsed: kind=%s videoId=%q channelId=%q handle=%q\n",
		parsed.Kind, parsed.VideoID, parsed.ChannelID, parsed.Handle)

	// Step 2 — resolve to channel + RSS URL.
	ch, err := r.Resolve(ctx, rawURL)
	if err != nil {
		if errors.Is(err, ErrRateLimited) {
			fmt.Println("rate limited — wait before retrying; do not loop:")
		}
		fmt.Println("resolve error:", err)
		return
	}
	fmt.Printf("channel id : %s\n", ch.ID)
	fmt.Printf("title      : %s\n", ch.Title)
	fmt.Printf("rss url    : %s\n", ch.RSSURL)
	fmt.Printf("source     : %s\n", ch.Source)

	// Step 3 — fetch the Atom feed (this endpoint is usually fine).
	feed, err := feedParser(ch.RSSURL)
	if err != nil {
		fmt.Println("feed parse error:", err)
		return
	}
	fmt.Printf("feed title : %s (%d entries)\n", feed.Title, len(feed.Items))
	if len(feed.Items) > 0 {
		fmt.Printf("latest     : %s\n", feed.Items[0].Title)
	}

	// Step 4 — second call hits the in-memory cache (no extra YouTube request).
	ch2, err := r.Resolve(ctx, rawURL)
	if err == nil {
		fmt.Printf("2nd resolve source: %s\n", ch2.Source)
	}
}

// feedParser is your existing gofeed helper (kept for the demo path).
func feedParser(url string) (*gofeed.Feed, error) {
	fp := gofeed.NewParser()
	fp.Client = &http.Client{Timeout: 20 * time.Second}
	feed, err := fp.ParseURL(url)
	if err != nil {
		return nil, err
	}
	return feed, nil
}
