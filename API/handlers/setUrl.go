package handlers

import (
	"context"
	"log"
	"strconv"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/config"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/models"
	pb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/setUrl"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/utility"
	rssdetector "github.com/CrimsonKarma44/rss_detector"

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
		return nil, err
	case 1:
		err := h.RedisClient.Set(ctx, req.Url, links[0].URL)
		if err != nil {
			return nil, err
		}
		return &pb.SetUrlResponse{
			Message: "url set successfully",
			FeedLinks: []string{links[0].URL},
		}, nil
	default:
		err := h.RedisClient.HSet(ctx, req.Url, func() map[string]string {
			linker := make(map[string]models.FeedType, len(links))
			for _, link := range links {
				linker[link.URL] = models.FeedType(link.Type)
			}
			return utility.ToStringMap(linker)
		}())
		if err != nil {
			return nil, err
		}
		return &pb.SetUrlResponse{
			Message: "multiple links detected: " + strconv.Itoa(len(links)),
			FeedLinks: func() []string {
				names := make([]string, len(links))
				for i, link := range links {
					names[i] = link.URL
				}
				return names
			}(),
		}, nil
	}
}
