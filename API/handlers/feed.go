package handlers

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/config"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/models"
	pb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/service"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/utility"
	rssdetector "github.com/CrimsonKarma44/rss_detector"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type FeedHandler struct {
	pb.UnimplementedFeedHandlerServiceServer
	LinkService *service.LinkRepoService
	RedisClient *config.RedisDB
}

func (h *FeedHandler) GetFeed(ctx context.Context, req *pb.GetFeedsRequest) (*pb.GetFeedsResponse, error) {
	var linkRepo models.LinkRepository

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check cache first
	feedLinks, err := h.RedisClient.HGetAll(ctx, req.Url)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// Cache miss, fetch from database
			linkRepo, err := h.LinkService.GetFeedLink(req.Url)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// Link not found, fetch from RSS detector
					links, err := rssdetector.Detect(ctx, req.Url)
					if err != nil {
						log.Println(err)
					}

					// LinkService
					linkRepo.Url = req.Url
					linkRepo.FeedLinks = func() map[string]models.FeedType {
						result := make(map[string]models.FeedType, len(links))
						for _, v := range links {
							result[v.URL] = models.FeedType(v.Type)
						}
						return result
					}()
					// adding to the database
					err = h.LinkService.AddLink(linkRepo)
					if err != nil {
						log.Println(err)
					}
					
					// Cache the result
					err = h.RedisClient.HSet(ctx, req.Url, )
					if err != nil {
						return nil, err
					}
					return &pb.GetFeedsResponse{
						Feeds: utility.ToFeedTypeProto(links),
					}, nil
				}
				// Cache the result
				err = h.RedisClient.HSet(ctx, req.Url, utility.ToStringMap(utility.ToFeedTypeMap(links)))
				if err != nil {
					return nil, err
				}
				return &pb.GetFeedsResponse{
					Feeds: utility.ToFeedTypeProto(links),
				}, nil
			}
			// Cache the result
			err = h.RedisClient.HSet(ctx, req.Url, utility.ToStringMap(linkRepo.FeedLinks))
			if err != nil {
				return nil, err
			}
		}
		return nil, err
	} else {
		// Cache hit, use cached data
		linkRepo.Url = req.Url
		linkRepo.FeedLinks = utility.ToFeedTypeMap(feedLinks)
	}

	// links, err := rssdetector.Detect(ctx, link.Url)
	// if err != nil {
	// 	log.Println(err)
	// }

	// feed, err := utility.FeedParser(ctx, l.URL)
	// if err != nil {
	// 	log.Println(err)
	// 	continue
	// }

	// from := time.Now()
	// if req.From != nil {
	// 	from = req.From.AsTime()
	// }

	// filteredLinks := make([]*models.FeedItem, 0, len(links))
	// for _, link := range links {
	// 	if link..After(from) {
	// 		filteredLinks = append(filteredLinks, link)
	// 	}
	// }

	// Return the cached or fetched data
	// return &pb.GetFeedsResponse{
	// 	Feeds: []*pb.GetFeedsResponse_Feed{
	// 		{
	// 			Title:       feedLinks["title"],
	// 	Description: feedLinks["description"],
	// }, nil
	return nil, nil
}
