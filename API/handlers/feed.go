package handlers

import (
	"context"
	"errors"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/config"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/models"
	pb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/service"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/utility"
	"github.com/redis/go-redis/v9"
)

type FeedHandler struct {
	pb.UnimplementedFeedHandlerServiceServer
	LinkService *service.LinkRepoService
	RedisClient *config.RedisDB
}

func (h *FeedHandler) GetFeed(ctx context.Context, req *pb.GetFeedRequest) (*pb.GetFeedResponse, error) {
	var link models.LinkRepository

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check cache first
	feedLinks, err := h.RedisClient.HGetAll(ctx, req.Url)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Cache miss, fetch from database
			link, err := h.LinkService.GetFeedLink(req.Url)
			if err != nil {
				return nil, err
			}
			// Cache the result
			err = h.RedisClient.HSet(ctx, req.Url, utility.ToStringMap(link.FeedLinks))
			if err != nil {
				return nil, err
			}
		}
		return nil, err
	} else {
		// Cache hit, use cached data
		link.Url = req.Url
		link.FeedLinks = utility.ToFeedTypeMap(feedLinks)
	}

	// Return the cached or fetched data
	return &pb.GetFeedResponse{
		Title:       feedLinks["title"],
		Description: feedLinks["description"],
	}, nil
}
