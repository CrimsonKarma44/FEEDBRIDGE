package handlers

import (
	"context"
	"errors"
	"log"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/config"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/models"
	pb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/service"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/utility"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/youtube"
	rssdetector "github.com/CrimsonKarma44/rss_detector"
	"github.com/mmcdole/gofeed"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

const (
	feedTimeout  = 30 * time.Second
	fetchWorkers = 5
)

type FeedHandler struct {
	pb.UnimplementedFeedHandlerServiceServer
	LinkService *service.LinkRepoService
	RedisClient *config.RedisDB
	YouTube     *youtube.Resolver
}

func (h *FeedHandler) GetFeed(ctx context.Context, req *pb.GetFeedsRequest) (*pb.GetFeedsResponse, error) {
	if req.GetUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "url is required")
	}

	ctx, cancel := context.WithTimeout(ctx, feedTimeout)
	defer cancel()

	linkMap, err := h.resolveLinks(ctx, req.GetUrl())
	if err != nil {
		return nil, err
	}

	items := h.fetchItems(ctx, linkMap)

	sort.SliceStable(items, func(i, j int) bool {
		ti, tj := items[i].item.PublishedParsed, items[j].item.PublishedParsed
		if ti == nil {
			return false
		}
		if tj == nil {
			return true
		}
		return ti.After(*tj)
	})

	var from time.Time
	if req.GetFrom() != nil {
		from = req.GetFrom().AsTime()
	}

	feeds := make([]*pb.GetFeedsResponse_Feed, 0, len(items))
	for _, entry := range items {
		pub := entry.item.PublishedParsed
		if !from.IsZero() && pub != nil && pub.Before(from) {
			continue
		}
		feeds = append(feeds, toItem(entry.srcTitle, entry.item))
	}

	return &pb.GetFeedsResponse{Feeds: feeds}, nil
}

func (h *FeedHandler) resolveLinks(ctx context.Context, url string) (map[string]models.FeedType, error) {
	if h.negativeExists(ctx, url) {
		return nil, status.Errorf(codes.NotFound, "no feeds found for %s (retry later)", url)
	}

	if cached, ok := loadCachedFeedLinks(ctx, h.RedisClient, url); ok {
		return linkMapFrom(cached), nil
	}

	linkRepo, err := h.LinkService.GetFeedLink(url)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, status.Errorf(codes.Internal, "db lookup failed for %s: %v", url, err)
		}

		v, derr, _ := flightGroup.Do(detectFlightKey+url, func() (any, error) {
			return resolveFeeds(ctx, h.YouTube, url)
		})
		var detected []rssdetector.FeedLink
		if v != nil {
			detected, _ = v.([]rssdetector.FeedLink)
		}
		if derr != nil {
			if isPermanentDetectErr(derr) {
				h.markNegative(ctx, url)
			}
			return nil, detectErrorStatus(url, derr)
		}
		if len(detected) == 0 {
			h.markNegative(ctx, url)
			return nil, status.Errorf(codes.NotFound, "no feeds found for %s", url)
		}

		linkMap := linkMapFrom(detected)

		if aerr := h.LinkService.AddLink(models.LinkRepository{Url: url, FeedLinks: linkMap}); aerr != nil {
			log.Println("persist links failed:", aerr)
		}
		if serr := storeCachedFeedLinks(ctx, h.RedisClient, url, detected); serr != nil {
			log.Println("cache links failed:", serr)
		}

		return linkMap, nil
	}

	linkMap := linkRepo.FeedLinks
	if linkMap == nil {
		linkMap = make(map[string]models.FeedType)
	}
	if serr := storeCachedFeedLinks(ctx, h.RedisClient, url, linksFromCached(linkMap)); serr != nil {
		log.Println("cache links failed:", serr)
	}
	return linkMap, nil
}

// itemWithSource pairs a parsed item with the channel title it came from.
type itemWithSource struct {
	srcTitle string
	item     *gofeed.Item
}

func (h *FeedHandler) fetchItems(ctx context.Context, linkMap map[string]models.FeedType) []itemWithSource {
	urls := make([]string, 0, len(linkMap))
	for u := range linkMap {
		urls = append(urls, u)
	}

	cached, misses := h.loadCachedItems(ctx, urls)

	sem := make(chan struct{}, fetchWorkers)
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		items = make([]itemWithSource, 0, len(urls))
	)

	appendItems := func(srcTitle string, list []*gofeed.Item) {
		mu.Lock()
		defer mu.Unlock()
		for _, it := range list {
			items = append(items, itemWithSource{srcTitle: srcTitle, item: it})
		}
	}

	for _, ff := range cached {
		appendItems(ff.title, ff.items)
	}

	for _, feedURL := range misses {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			v, err, _ := flightGroup.Do(fetchFlightKey+u, func() (any, error) {
				feed, ferr := utility.FeedParser(ctx, u)
				if ferr != nil {
					return nil, ferr
				}
				h.storeItems(ctx, u, feed.Title, feed.Items)
				return fetchedFeed{title: feed.Title, items: feed.Items}, nil
			})
			if err != nil {
				log.Printf("parse feed %s: %v", u, err)
				return
			}
			if ff, ok := v.(fetchedFeed); ok {
				appendItems(ff.title, ff.items)
			}
		}(feedURL)
	}

	wg.Wait()
	return items
}

func toItem(srcTitle string, item *gofeed.Item) *pb.GetFeedsResponse_Feed {
	description := item.Description
	if description == "" {
		description = item.Content
	}

	authors := make([]*pb.GetFeedsResponse_Feed_Person, 0, len(item.Authors))
	for _, author := range item.Authors {
		authors = append(authors, &pb.GetFeedsResponse_Feed_Person{Name: author.Name, Email: author.Email})
	}

	images := make([]*pb.GetFeedsResponse_Feed_Image, 0, 1)
	if item.Image != nil && item.Image.URL != "" {
		images = append(images, &pb.GetFeedsResponse_Feed_Image{Url: item.Image.URL})
	}

	enclosures := make([]*pb.GetFeedsResponse_Feed_Enclosure, 0, len(item.Enclosures))
	for _, enc := range item.Enclosures {
		length, _ := strconv.ParseInt(enc.Length, 10, 64)
		enclosures = append(enclosures, &pb.GetFeedsResponse_Feed_Enclosure{
			Url:    enc.URL,
			Type:   enc.Type,
			Length: length,
		})
	}

	var publishedAt *timestamppb.Timestamp
	if item.PublishedParsed != nil {
		publishedAt = timestamppb.New(*item.PublishedParsed)
	}

	return &pb.GetFeedsResponse_Feed{
		Title:       item.Title,
		Description: description,
		Images:      images,
		Authors:     authors,
		Enclosures:  enclosures,
		Links:       item.Links,
		Categories:  item.Categories,
		PublishedAt: publishedAt,
		SourceTitle: srcTitle,
	}
}
