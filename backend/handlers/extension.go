package handlers

import (
	"context"
	"errors"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/backend/config"
	pb "github.com/CrimsonKarma44/FEEDBRIDGE/backend/protoAPI/extension"
	"github.com/CrimsonKarma44/FEEDBRIDGE/backend/service"
	"github.com/CrimsonKarma44/FEEDBRIDGE/backend/youtube"
	rssdetector "github.com/CrimsonKarma44/rss_detector"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const maxPageURLLen = 2048

// FeedFinder finds the feeds behind a page URL.
type FeedFinder interface {
	Find(ctx context.Context, url string) ([]rssdetector.FeedLink, error)
}

// LinkTokenStore issues and resolves Telegram deep-link tokens.
// *service.LinkTokens implements it.
type LinkTokenStore interface {
	Create(ctx context.Context, url string) (string, time.Time, error)
	Resolve(ctx context.Context, token string, consume bool) (string, error)
}

// CachedFinder detects feeds through the Redis cache. Unlike SetUrl and
// GetFeed it never writes to Postgres, so pages people only look at from
// the browser extension don't pile up in link_repositories.
type CachedFinder struct {
	Redis   *config.RedisDB
	YouTube *youtube.Resolver
}

func (f CachedFinder) Find(ctx context.Context, url string) ([]rssdetector.FeedLink, error) {
	if negativeExists(ctx, f.Redis, url) {
		return nil, rssdetector.ErrNoFeeds
	}
	links, cached, err := detectCached(ctx, f.Redis, f.YouTube, url)
	if err != nil {
		if isPermanentDetectErr(err) {
			markNegative(ctx, f.Redis, url)
		}
		return nil, err
	}
	if len(links) == 0 {
		markNegative(ctx, f.Redis, url)
		return nil, rssdetector.ErrNoFeeds
	}
	if !cached {
		if serr := storeCachedFeedLinks(ctx, f.Redis, url, links); serr != nil {
			log.Println("cache links failed:", serr)
		}
	}
	return links, nil
}

// ExtensionHandler implements ExtensionService for the browser extension
// (Detect, CreateLinkToken) and the Telegram bot (ResolveLinkToken).
type ExtensionHandler struct {
	pb.UnimplementedExtensionServiceServer
	Feeds  FeedFinder
	Tokens LinkTokenStore
}

func (h *ExtensionHandler) Detect(ctx context.Context, req *pb.DetectRequest) (*pb.DetectResponse, error) {
	pageURL, err := validatePageURL(req.GetUrl())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid url: %v", err)
	}

	ctx, cancel := context.WithTimeout(ctx, setTimeout)
	defer cancel()

	links, err := h.Feeds.Find(ctx, pageURL)
	if err != nil {
		return nil, detectErrorStatus(pageURL, err)
	}

	feeds := make([]*pb.DetectedFeed, 0, len(links))
	for _, l := range links {
		if l.URL == "" {
			continue
		}
		feeds = append(feeds, &pb.DetectedFeed{
			Url:    l.URL,
			Title:  l.Title,
			Type:   feedTypeName(l.Type),
			Source: string(l.Source),
		})
	}
	return &pb.DetectResponse{Url: pageURL, Feeds: feeds}, nil
}

func (h *ExtensionHandler) CreateLinkToken(ctx context.Context, req *pb.CreateLinkTokenRequest) (*pb.LinkToken, error) {
	feedURL, err := validatePageURL(req.GetUrl())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid url: %v", err)
	}
	token, expires, err := h.Tokens.Create(ctx, feedURL)
	if err != nil {
		log.Println("create link token failed:", err)
		return nil, status.Error(codes.Unavailable, "could not create a link token, try again")
	}
	return &pb.LinkToken{Token: token, ExpiresAt: timestamppb.New(expires)}, nil
}

func (h *ExtensionHandler) ResolveLinkToken(ctx context.Context, req *pb.ResolveLinkTokenRequest) (*pb.ResolveLinkTokenResponse, error) {
	feedURL, err := h.Tokens.Resolve(ctx, req.GetToken(), req.GetConsume())
	switch {
	case errors.Is(err, service.ErrLinkTokenInvalid):
		return nil, status.Error(codes.InvalidArgument, "malformed link token")
	case errors.Is(err, service.ErrLinkTokenNotFound):
		return nil, status.Error(codes.NotFound, "link token expired or already used")
	case err != nil:
		log.Println("resolve link token failed:", err)
		return nil, status.Error(codes.Unavailable, "could not look up the link token, try again")
	}
	return &pb.ResolveLinkTokenResponse{Url: feedURL}, nil
}

// validatePageURL accepts absolute http(s) URLs with a host. Private and
// loopback targets are refused later by the safe HTTP client when fetched.
func validatePageURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	switch {
	case s == "":
		return "", errors.New("url is required")
	case len(s) > maxPageURLLen:
		return "", errors.New("url is too long")
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", errors.New("url could not be parsed")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("only http and https urls are supported")
	}
	if u.Hostname() == "" {
		return "", errors.New("url has no host")
	}
	return s, nil
}

func feedTypeName(t rssdetector.FeedType) string {
	switch t {
	case rssdetector.FeedTypeRSS, rssdetector.FeedTypeAtom, rssdetector.FeedTypeJSON:
		return string(t)
	default:
		return string(rssdetector.FeedTypeUnknown)
	}
}
