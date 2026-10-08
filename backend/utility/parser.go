package utility

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/mmcdole/gofeed"
)

const (
	fetchTimeout  = 20 * time.Second
	maxFeedBody   = 4 << 20
	FeedUserAgent = "FEEDBRIDGE/1.0 (+https://github.com/CrimsonKarma44/FEEDBRIDGE)"
)

var (
	ErrFeedTooLarge = errors.New("feed body exceeds size limit")
	feedHTTPClient  = NewSafeHTTPClient(fetchTimeout)
)

func RetryableFeedError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr gofeed.HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
			http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
	}
	return false
}

func FeedParser(ctx context.Context, url string) (*gofeed.Feed, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", FeedUserAgent)
		req.Header.Set("Accept", "application/rss+xml, application/xml, text/xml, application/atom+xml, application/json, */*")

		resp, err := feedHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, err
			}
			if attempt < 2 {
				time.Sleep(250 * time.Duration(attempt+1) * time.Millisecond)
				continue
			}
			return nil, err
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = gofeed.HTTPError{StatusCode: resp.StatusCode, Status: resp.Status}
			_ = resp.Body.Close()
			if RetryableFeedError(lastErr) && attempt < 2 {
				time.Sleep(250 * time.Duration(attempt+1) * time.Millisecond)
				continue
			}
			return nil, lastErr
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			return nil, gofeed.HTTPError{StatusCode: resp.StatusCode, Status: resp.Status + ": " + string(body)}
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBody+1))
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		if len(body) > maxFeedBody {
			return nil, fmt.Errorf("%w (%d bytes)", ErrFeedTooLarge, len(body))
		}

		fp := gofeed.NewParser()
		feed, err := fp.Parse(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		return feed, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("feed fetch failed")
}
