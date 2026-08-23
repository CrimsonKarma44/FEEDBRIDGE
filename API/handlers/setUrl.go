package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/config"
	pb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/setUrl"
	rssdetector "github.com/CrimsonKarma44/rss_detector"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/service"
)

type SetUrlHandler struct {
	pb.UnimplementedSetUrlHandlerServer
	LinkService *service.LinkRepoService
	RedisClient *config.RedisDB
}

func (h *SetUrlHandler) SetUrl(ctx context.Context, req *pb.SetUrlRequest) (*pb.SetUrlResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	links, err := rssdetector.Detect(ctx, req.Url)
	if err != nil {
		log.Fatal(err)
	}

	switch len(links) {
	case 0:
		return nil, status.Error(codes.InvalidArgument, "no feed links provided")
	default:
		// Build the map for Redis HSET
		linker := make(map[string]string, len(links))
		for _, link := range links {
			linker[link.URL] = string(link.Type) // direct cast, no utility needed
		}

		// Use a prefixed key for namespace isolation
		redisKey := fmt.Sprintf("feed:%s", req.Url)

		if err := h.RedisClient.HSet(ctx, redisKey, linker); err != nil {
			return nil, status.Errorf(codes.Internal,
				"failed to cache feed links for %s: %v", req.Url, err)
		}

		// Build response
		names := make([]string, len(links))
		for i, link := range links {
			names[i] = link.URL
		}

		return &pb.SetUrlResponse{
			Message:   fmt.Sprintf("URL set successfully: %d feeds", len(links)),
			FeedLinks: names,
		}, nil
	}
}
