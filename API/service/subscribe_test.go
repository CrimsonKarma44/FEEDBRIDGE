package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/internal/testdb"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/store"
)

type fakeDetector struct {
	links []FeedLink
	err   error
	calls int
	last  string
}

func (f *fakeDetector) Detect(_ context.Context, url string) ([]FeedLink, error) {
	f.calls++
	f.last = url
	if f.err != nil {
		return nil, f.err
	}
	return f.links, nil
}

func newSubService(t *testing.T, d Detector) *SubscribeService {
	t.Helper()
	st, err := store.New(testdb.Open(t))
	if err != nil {
		t.Fatal(err)
	}
	return NewSubscribeService(st, d)
}

func TestSubscribe_StoresCanonicalFeedNotHomepage(t *testing.T) {
	d := &fakeDetector{links: []FeedLink{
		{URL: "https://example.com/atom.xml"},
		{URL: "https://example.com/rss.xml"},
	}}
	svc := newSubService(t, d)
	res, err := svc.Subscribe(context.Background(), SubscribeRequest{
		Platform:   "telegram",
		ExternalID: "42",
		URL:        "https://example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Created {
		t.Fatal("expected created")
	}
	if res.Subscription.URL != "https://example.com/atom.xml" {
		t.Fatalf("stored %q", res.Subscription.URL)
	}
	if d.calls != 1 || d.last != "https://example.com" {
		t.Fatalf("detect calls=%d last=%s", d.calls, d.last)
	}
}

func TestSubscribe_HomepageMatchesExistingCanonical(t *testing.T) {
	d := &fakeDetector{links: []FeedLink{
		{URL: "https://example.com/atom.xml"},
		{URL: "https://example.com/rss.xml"},
	}}
	svc := newSubService(t, d)
	ctx := context.Background()
	first, err := svc.Subscribe(ctx, SubscribeRequest{
		Platform: "telegram", ExternalID: "42", URL: "https://example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Subscribe(ctx, SubscribeRequest{
		Platform: "telegram", ExternalID: "42", URL: "https://example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.Created {
		t.Fatal("second subscribe should not create")
	}
	if second.Subscription.ID != first.Subscription.ID {
		t.Fatalf("ids %d vs %d", second.Subscription.ID, first.Subscription.ID)
	}
}

func TestSubscribe_DetectErrorSurfaces(t *testing.T) {
	d := &fakeDetector{err: errors.New("no feeds")}
	svc := newSubService(t, d)
	_, err := svc.Subscribe(context.Background(), SubscribeRequest{
		Platform: "telegram", ExternalID: "1", URL: "https://example.com",
	})
	if err == nil {
		t.Fatal("expected detect error")
	}
}

func TestSubscribe_CursorStartsNow(t *testing.T) {
	d := &fakeDetector{links: []FeedLink{{URL: "https://example.com/feed.xml"}}}
	svc := newSubService(t, d)
	before := time.Now().UTC().Add(-time.Second)
	res, err := svc.Subscribe(context.Background(), SubscribeRequest{
		Platform: "telegram", ExternalID: "7", URL: "https://example.com/feed.xml",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Subscription.LastSeenPublished.Before(before) {
		t.Fatalf("cursor %v is in the past", res.Subscription.LastSeenPublished)
	}
}

func TestResolveURL_HomepageFindsStoredFeed(t *testing.T) {
	d := &fakeDetector{links: []FeedLink{{URL: "https://example.com/atom.xml"}}}
	svc := newSubService(t, d)
	ctx := context.Background()
	if _, err := svc.Subscribe(ctx, SubscribeRequest{
		Platform: "telegram", ExternalID: "42", URL: "https://example.com",
	}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ResolveURL(ctx, "telegram", "42", "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.com/atom.xml" {
		t.Fatalf("got %q", got)
	}
}

func TestSubscribe_EmptyDetectIsError(t *testing.T) {
	d := &fakeDetector{links: nil}
	svc := newSubService(t, d)
	_, err := svc.Subscribe(context.Background(), SubscribeRequest{
		Platform: "telegram", ExternalID: "1", URL: "https://example.com",
	})
	if err == nil {
		t.Fatal("expected error for no links")
	}
}
