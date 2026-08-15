package handlers

import (
	"context"
	"log"
	"strconv"
	"time"

	pb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/setUrl"
	rssdetector "github.com/CrimsonKarma44/rss_detector"
)

type SetUrlHandler struct {
	pb.UnimplementedSetUrlHandlerServer
}

func (h *SetUrlHandler) HandleSetUrl(ctx context.Context, req *pb.SetUrlRequest) (*pb.SetUrlResponse, error) {
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
		return &pb.SetUrlResponse{
			Message: "url set successfully",
		}, nil
	default:
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
