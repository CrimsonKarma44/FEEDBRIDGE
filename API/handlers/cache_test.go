package handlers

import (
	"encoding/json"
	"testing"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/models"
	rssdetector "github.com/CrimsonKarma44/rss_detector"
)

func TestFeedLinksCacheRoundTripPreservesOrder(t *testing.T) {
	in := []rssdetector.FeedLink{
		{URL: "https://example.com/atom.xml", Type: rssdetector.FeedTypeAtom},
		{URL: "https://example.com/rss.xml", Type: rssdetector.FeedTypeRSS},
	}
	out := cachedToFeedLinks(feedLinksToCached(in))
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].URL != in[0].URL || out[1].URL != in[1].URL {
		t.Fatalf("order lost: %q then %q", out[0].URL, out[1].URL)
	}
	if models.FeedType(out[0].Type) != models.FeedTypeAtom {
		t.Fatalf("type = %q", out[0].Type)
	}
}

func TestLinksFromCached_StableSort(t *testing.T) {
	m := map[string]models.FeedType{
		"https://b.example/feed": models.FeedTypeRSS,
		"https://a.example/feed": models.FeedTypeAtom,
	}
	got := linksFromCached(m)
	if len(got) != 2 || got[0].URL != "https://a.example/feed" || got[1].URL != "https://b.example/feed" {
		t.Fatalf("got %#v", got)
	}
}

func TestLinkMapFrom_RoundTrip(t *testing.T) {
	links := []rssdetector.FeedLink{
		{URL: "https://example.com/atom.xml", Type: rssdetector.FeedTypeAtom},
		{URL: "https://example.com/rss.xml", Type: rssdetector.FeedTypeRSS},
	}
	m := linkMapFrom(links)
	if m["https://example.com/atom.xml"] != models.FeedTypeAtom {
		t.Fatalf("map = %#v", m)
	}
}

func TestFeedLinksJSON_PreservesOrder(t *testing.T) {
	in := []rssdetector.FeedLink{
		{URL: "https://example.com/comments.xml", Type: rssdetector.FeedTypeAtom},
		{URL: "https://example.com/atom.xml", Type: rssdetector.FeedTypeAtom},
		{URL: "https://example.com/rss.xml", Type: rssdetector.FeedTypeRSS},
	}
	raw, err := json.Marshal(feedLinksToCached(in))
	if err != nil {
		t.Fatal(err)
	}
	var decoded []cachedFeedLink
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	out := cachedToFeedLinks(decoded)
	if len(out) != 3 || out[0].URL != in[0].URL || out[2].URL != in[2].URL {
		t.Fatalf("json order lost: %#v", out)
	}
}

func TestCachedToFeedLinks_SkipsEmptyURL(t *testing.T) {
	out := cachedToFeedLinks([]cachedFeedLink{
		{URL: "https://example.com/atom.xml", Type: models.FeedTypeAtom},
		{URL: "", Type: models.FeedTypeRSS},
		{URL: "https://example.com/rss.xml", Type: models.FeedTypeRSS},
	})
	if len(out) != 2 || out[0].URL != "https://example.com/atom.xml" || out[1].URL != "https://example.com/rss.xml" {
		t.Fatalf("got %#v", out)
	}
}

func TestLinksFromCached_EmptyAndBlankKeys(t *testing.T) {
	if got := linksFromCached(nil); len(got) != 0 {
		t.Fatalf("nil: %#v", got)
	}
	got := linksFromCached(map[string]models.FeedType{
		"":                      models.FeedTypeRSS,
		"https://example.com/a": models.FeedTypeAtom,
	})
	if len(got) != 1 || got[0].URL != "https://example.com/a" {
		t.Fatalf("got %#v", got)
	}
}
