package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/store"
	"gorm.io/gorm"
)

var ErrNoFeeds = errors.New("no feeds were found")

type FeedLink struct {
	URL string
}

type Detector interface {
	Detect(ctx context.Context, url string) ([]FeedLink, error)
}

type SubscribeRequest struct {
	Platform   string
	ExternalID string
	URL        string
}

type SubscribeResult struct {
	Subscription *store.Subscription
	Created      bool
}

type SubscribeService struct {
	store    *store.Store
	detector Detector
}

func NewSubscribeService(st *store.Store, d Detector) *SubscribeService {
	return &SubscribeService{store: st, detector: d}
}

func (s *SubscribeService) Store() *store.Store { return s.store }

func (s *SubscribeService) Subscribe(ctx context.Context, req SubscribeRequest) (*SubscribeResult, error) {
	if req.Platform == "" || req.ExternalID == "" || req.URL == "" {
		return nil, fmt.Errorf("platform, external_id, and url are required")
	}
	links, err := s.detector.Detect(ctx, req.URL)
	if err != nil {
		return nil, err
	}
	if len(links) == 0 {
		return nil, ErrNoFeeds
	}
	feedURLs := make([]string, 0, len(links))
	for _, l := range links {
		if l.URL != "" {
			feedURLs = append(feedURLs, l.URL)
		}
	}
	if len(feedURLs) == 0 {
		return nil, ErrNoFeeds
	}

	existing, err := s.store.List(req.Platform, req.ExternalID)
	if err != nil {
		return nil, err
	}
	have := make([]string, len(existing))
	for i, sub := range existing {
		have[i] = sub.URL
	}
	canonical := feedURLs[0]
	if match := firstExisting(store.MatchingURLs(req.URL, feedURLs), have); match != "" {
		canonical = match
	}
	sub, created, err := s.store.Add(req.Platform, req.ExternalID, canonical, store.DefaultInterval)
	if err != nil {
		return nil, err
	}
	return &SubscribeResult{Subscription: sub, Created: created}, nil
}

func (s *SubscribeService) ResolveURL(ctx context.Context, platform, externalID, url string) (string, error) {
	ok, err := s.store.Exists(platform, externalID, url)
	if err != nil {
		return "", err
	}
	if ok {
		return url, nil
	}
	links, err := s.detector.Detect(ctx, url)
	if err != nil {
		return "", err
	}
	feedURLs := make([]string, 0, len(links))
	for _, l := range links {
		if l.URL != "" {
			feedURLs = append(feedURLs, l.URL)
		}
	}
	existing, err := s.store.List(platform, externalID)
	if err != nil {
		return "", err
	}
	have := make([]string, len(existing))
	for i, sub := range existing {
		have[i] = sub.URL
	}
	match := firstExisting(store.MatchingURLs(url, feedURLs), have)
	if match == "" {
		return "", gorm.ErrRecordNotFound
	}
	return match, nil
}

func firstExisting(candidates, existing []string) string {
	have := make(map[string]struct{}, len(existing))
	for _, e := range existing {
		have[e] = struct{}{}
	}
	for _, c := range candidates {
		if _, ok := have[c]; ok {
			return c
		}
	}
	return ""
}
