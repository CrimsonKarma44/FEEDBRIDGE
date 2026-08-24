package handlers

import (
	"context"
	"errors"
	"log"
	"net/url"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/youtube"
	rssdetector "github.com/CrimsonKarma44/rss_detector"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// resolveFeeds discovers feed links for a URL. YouTube URLs are handled by the
// Innertube-based resolver (no HTML scraping, no captchas); everything else
// goes through the generic detector, with a one-shot fallback to the site root.
func resolveFeeds(ctx context.Context, yt *youtube.Resolver, rawURL string) ([]rssdetector.FeedLink, error) {
	if yt != nil {
		if _, perr := youtube.ParseURL(rawURL); perr == nil {
			ch, rerr := yt.Resolve(ctx, rawURL)
			if rerr != nil {
				return nil, rerr
			}
			title := ch.Title
			if title == "" {
				title = ch.ID
			}
			return []rssdetector.FeedLink{{
				URL:   ch.RSSURL,
				Title: title,
				Type:  rssdetector.FeedTypeAtom,
			}}, nil
		}
	}
	return detectWithFallback(ctx, rawURL)
}

// detectWithFallback retries detection against the site root when a deep link
// fails for reasons that the homepage may not share (blocked page, article
// without feeds, dead path). Rate-limits and invalid input never fall back:
// hammering a throttling server makes things worse, and stripping cannot fix
// a malformed URL. The original error wins when the retry fails too.
func detectWithFallback(ctx context.Context, rawURL string) ([]rssdetector.FeedLink, error) {
	links, err := rssdetector.Detect(ctx, rawURL)
	if err == nil || !fallbackable(err) {
		return links, err
	}

	root := siteRoot(rawURL)
	if root == "" || root == rawURL {
		return links, err
	}

	log.Printf("detect failed for %s (%v); retrying site root %s", rawURL, err, root)
	retryLinks, retryErr := rssdetector.Detect(ctx, root)
	if retryErr == nil && len(retryLinks) > 0 {
		return retryLinks, nil
	}
	return links, err
}

// fallbackable reports whether detection failure might be specific to the
// deep link rather than the whole site.
func fallbackable(err error) bool {
	return errors.Is(err, rssdetector.ErrNoFeeds) ||
		errors.Is(err, rssdetector.ErrNotHTML) ||
		errors.Is(err, rssdetector.ErrBlocked) ||
		errors.Is(err, rssdetector.ErrFetchFailed)
}

// siteRoot strips a URL down to scheme://host.
func siteRoot(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// isPermanentDetectErr reports whether a detection failure is stable enough
// that negative-caching it will not hide recoverable outages.
func isPermanentDetectErr(err error) bool {
	return errors.Is(err, rssdetector.ErrNoFeeds) ||
		errors.Is(err, rssdetector.ErrNotHTML) ||
		errors.Is(err, rssdetector.ErrInvalidURL)
}

// detectErrorStatus translates detector failures into precise gRPC statuses.
func detectErrorStatus(url string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, rssdetector.ErrNoFeeds), errors.Is(err, rssdetector.ErrNotHTML):
		return status.Errorf(codes.NotFound, "no feeds found for %s", url)
	case errors.Is(err, rssdetector.ErrRateLimited):
		return status.Errorf(codes.ResourceExhausted, "%s is temporarily limiting automated requests", url)
	case errors.Is(err, rssdetector.ErrBlocked):
		return status.Errorf(codes.PermissionDenied, "%s blocks automated access", url)
	case errors.Is(err, rssdetector.ErrInvalidURL):
		return status.Errorf(codes.InvalidArgument, "invalid URL: %s", url)
	default:
		log.Println("detect failed:", err)
		return status.Errorf(codes.Unavailable, "feed detection failed for %s: %v", url, err)
	}
}
