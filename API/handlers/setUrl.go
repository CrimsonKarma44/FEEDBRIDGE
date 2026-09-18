package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/config"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/models"
	pb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/setUrl"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/service"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/youtube"
	rssdetector "github.com/CrimsonKarma44/rss_detector"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const setTimeout = 30 * time.Second

type SetUrlHandler struct {
	pb.UnimplementedSetUrlHandlerServer
	LinkService *service.LinkRepoService
	RedisClient *config.RedisDB
	YouTube     *youtube.Resolver
}

func (h *SetUrlHandler) SetUrl(ctx context.Context, req *pb.SetUrlRequest) (*pb.SetUrlResponse, error) {
	if req.GetUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "url is required")
	}

	ctx, cancel := context.WithTimeout(ctx, setTimeout)
	defer cancel()

	v, err, _ := flightGroup.Do(detectFlightKey+req.GetUrl(), func() (any, error) {
		if cached, ok := loadCachedFeedLinks(ctx, h.RedisClient, req.GetUrl()); ok {
			log.Printf("cache hit for %s, skipping detection", req.GetUrl())
			return cached, nil
		}
		return resolveFeeds(ctx, h.YouTube, req.GetUrl())
	})
	var links []rssdetector.FeedLink
	if v != nil {
		links, _ = v.([]rssdetector.FeedLink)
	}
	if err != nil {
		return nil, detectErrorStatus(req.GetUrl(), err)
	}

	if len(links) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "no feed links found for %s", req.GetUrl())
	}

	linkMap := make(map[string]models.FeedType, len(links))
	for _, link := range links {
		linkMap[link.URL] = models.FeedType(link.Type)
	}

	if aerr := h.LinkService.AddLink(models.LinkRepository{Url: req.GetUrl(), FeedLinks: linkMap}); aerr != nil {
		log.Println("persist links failed:", aerr)
	}

	if serr := storeCachedFeedLinks(ctx, h.RedisClient, req.GetUrl(), links); serr != nil {
		return nil, status.Errorf(codes.Internal,
			"failed to cache feed links for %s: %v", req.GetUrl(), serr)
	}

	if derr := h.RedisClient.Del(ctx, negativeKeyPrefix+req.GetUrl()); derr != nil {
		log.Println("negative cache clear failed:", derr)
	}

	names := make([]string, len(links))
	for i, link := range links {
		names[i] = link.URL
	}

	return &pb.SetUrlResponse{
		Message:   fmt.Sprintf("URL set successfully: %d feeds", len(links)),
		FeedLinks: names,
	}, nil
}
