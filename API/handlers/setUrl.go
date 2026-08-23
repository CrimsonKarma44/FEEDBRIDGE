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
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/utility"
	rssdetector "github.com/CrimsonKarma44/rss_detector"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const setTimeout = 30 * time.Second

type SetUrlHandler struct {
	pb.UnimplementedSetUrlHandlerServer
	LinkService *service.LinkRepoService
	RedisClient *config.RedisDB
}

func (h *SetUrlHandler) SetUrl(ctx context.Context, req *pb.SetUrlRequest) (*pb.SetUrlResponse, error) {
	if req.GetUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "url is required")
	}

	ctx, cancel := context.WithTimeout(ctx, setTimeout)
	defer cancel()

	links, err := rssdetector.Detect(ctx, req.GetUrl())
	if err != nil {
		log.Println("detect failed:", err)
		return nil, status.Errorf(codes.Unavailable,
			"feed detection failed for %s: %v", req.GetUrl(), err)
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

	if serr := h.RedisClient.HSet(ctx, redisKeyPrefix+req.GetUrl(), utility.ToStringMap(linkMap)); serr != nil {
		return nil, status.Errorf(codes.Internal,
			"failed to cache feed links for %s: %v", req.GetUrl(), serr)
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
