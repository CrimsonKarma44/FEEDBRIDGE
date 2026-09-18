package handlers

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/config"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/models"
	rssdetector "github.com/CrimsonKarma44/rss_detector"
	"github.com/mmcdole/gofeed"
	"golang.org/x/sync/singleflight"
)

const (
	itemsTTL    = 10 * time.Minute
	linksTTL    = 24 * time.Hour
	negativeTTL = 5 * time.Minute

	itemKeyPrefix      = "items:"
	negativeKeyPrefix  = "notfound:"
	orderedLinksPrefix = "feedlinks:"
	fetchFlightKey     = "fetch:"
	detectFlightKey    = "detect:"
)

// cachedFeedLink is the JSON shape stored under feedlinks:<inputURL>.
// A list (not a Redis hash) so detector rank is preserved across cache hits.
type cachedFeedLink struct {
	URL  string          `json:"url"`
	Type models.FeedType `json:"type"`
}

var flightGroup singleflight.Group

// CachedItem is the JSON shape stored under items:<feedURL>.
type CachedItem struct {
	SourceTitle string       `json:"src"`
	Item        *gofeed.Item `json:"item"`
}

// fetchedFeed bundles a parsed feed with its channel title.
type fetchedFeed struct {
	title string
	items []*gofeed.Item
}

func (h *FeedHandler) loadCachedItems(ctx context.Context, urls []string) (map[string]fetchedFeed, []string) {
	hits := make(map[string]fetchedFeed, len(urls))
	if len(urls) == 0 {
		return hits, nil
	}

	keys := make([]string, len(urls))
	for i, u := range urls {
		keys[i] = itemKeyPrefix + u
	}

	vals, err := h.RedisClient.MGet(ctx, keys...)
	if err != nil {
		log.Println("item cache lookup failed:", err)
		return hits, urls
	}

	var misses []string
	for i, val := range vals {
		if val == nil {
			misses = append(misses, urls[i])
			continue
		}
		var cached []CachedItem
		if jerr := json.Unmarshal([]byte(*val), &cached); jerr != nil || len(cached) == 0 {
			log.Println("item cache decode failed:", jerr)
			misses = append(misses, urls[i])
			continue
		}
		ff := fetchedFeed{}
		for _, c := range cached {
			if c.Item == nil {
				ff.items = nil
				break
			}
			if ff.title == "" {
				ff.title = c.SourceTitle
			}
			ff.items = append(ff.items, c.Item)
		}
		if ff.items == nil {
			misses = append(misses, urls[i])
			continue
		}
		hits[urls[i]] = ff
	}
	return hits, misses
}

func (h *FeedHandler) storeItems(ctx context.Context, feedURL, sourceTitle string, items []*gofeed.Item) {
	cached := make([]CachedItem, 0, len(items))
	for _, it := range items {
		cached = append(cached, CachedItem{SourceTitle: sourceTitle, Item: it})
	}
	if err := h.RedisClient.SetJSON(ctx, itemKeyPrefix+feedURL, itemsTTL, cached); err != nil {
		log.Println("item cache store failed:", err)
	}
}

func (h *FeedHandler) negativeExists(ctx context.Context, url string) bool {
	exists, err := h.RedisClient.Exists(ctx, negativeKeyPrefix+url)
	if err != nil {
		log.Println("negative cache lookup failed:", err)
		return false
	}
	return exists
}

func (h *FeedHandler) markNegative(ctx context.Context, url string) {
	ok, err := h.RedisClient.SetNX(ctx, negativeKeyPrefix+url, 1, negativeTTL)
	if err != nil {
		log.Println("negative cache store failed:", err)
		return
	}
	if ok {
		log.Printf("negative-cached %s for %s", url, negativeTTL)
	}
}

func orderedLinksKey(url string) string {
	return orderedLinksPrefix + url
}

func feedLinksToCached(links []rssdetector.FeedLink) []cachedFeedLink {
	out := make([]cachedFeedLink, len(links))
	for i, l := range links {
		out[i] = cachedFeedLink{URL: l.URL, Type: models.FeedType(l.Type)}
	}
	return out
}

func cachedToFeedLinks(cached []cachedFeedLink) []rssdetector.FeedLink {
	out := make([]rssdetector.FeedLink, 0, len(cached))
	for _, c := range cached {
		if c.URL == "" {
			continue
		}
		out = append(out, rssdetector.FeedLink{URL: c.URL, Type: rssdetector.FeedType(c.Type)})
	}
	return out
}

func linkMapFrom(links []rssdetector.FeedLink) map[string]models.FeedType {
	m := make(map[string]models.FeedType, len(links))
	for _, l := range links {
		m[l.URL] = models.FeedType(l.Type)
	}
	return m
}

func loadCachedFeedLinks(ctx context.Context, r *config.RedisDB, url string) ([]rssdetector.FeedLink, bool) {
	if r == nil {
		return nil, false
	}
	var ordered []cachedFeedLink
	if err := r.GetJSON(ctx, orderedLinksKey(url), &ordered); err == nil {
		if links := cachedToFeedLinks(ordered); len(links) > 0 {
			return links, true
		}
	}
	hash, err := r.HGetAll(ctx, url)
	if err != nil || len(hash) == 0 {
		return nil, false
	}
	m := make(map[string]models.FeedType, len(hash))
	for k, v := range hash {
		m[k] = models.FeedType(v)
	}
	return linksFromCached(m), true
}

func storeCachedFeedLinks(ctx context.Context, r *config.RedisDB, url string, links []rssdetector.FeedLink) error {
	if r == nil || len(links) == 0 {
		return nil
	}
	return r.SetJSON(ctx, orderedLinksKey(url), linksTTL, feedLinksToCached(links))
}
