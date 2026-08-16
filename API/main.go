package main

import (
	// "context"
	"fmt"
	"log"

	"net"

	config "github.com/CrimsonKarma44/FEEDBRIDGE/API/config"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/service"

	// "time"
	// rssdetector "github.com/CrimsonKarma44/rss_detector"
	// "github.com/mmcdole/gofeed"
	"os"

	handler "github.com/CrimsonKarma44/FEEDBRIDGE/API/handlers"
	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	pb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/setUrl"
	"google.golang.org/grpc"
)

func main() {
	// initializing logger
	logger := log.New(
		os.Stdout,                          // output destination
		"FEEDBRIDGE: ",                     // prefix
		log.Ldate|log.Ltime|log.Lshortfile, // flags
	)

	// Loading environment variables
	env := config.LoadENV()
	logger.Printf("Environment variables loaded: %v\n", env)

	// Loading database
	db, err := config.NewDB(env)
	if err != nil {
		fmt.Println(err)
		return
	}
	logger.Printf("DB connection established: %v\n", db.Name())

	// Loading Redis
	redis := config.NewRedisDB(env)
	logger.Printf("Redis connection established: %v\n", redis)

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}

	// Initializing link service
	linkService := service.NewLinkRepoService(db.DB) // redis

	// Registering gRPC services
	srvUrl := &handler.SetUrlHandler{LinkService: linkService, RedisClient: redis}
	srvFeed := &handler.FeedHandler{LinkService: linkService, RedisClient: redis}
	s := grpc.NewServer()
	pb.RegisterSetUrlHandlerServer(s, srvUrl)
	feedpb.RegisterFeedHandlerServiceServer(s, srvFeed)

	logger.Println("Server running on :50051")

	if err := s.Serve(lis); err != nil {
		logger.Fatalf("failed to serve: %v", err)
	}

	// demoURL := "https://feeds.transistor.fm/cup-o-go"
	// demoURL := "https://feeds.transistor.fm/cup-o-go"
	// demoURL := "https://www.youtube.com/watch?v=kD0w3YQxV6E"

	// ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// defer cancel()

	// links, err := rssdetector.Detect(ctx, demoURL)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// last_week := time.Now().Add(-121 * 24 * time.Hour)

	// for _, l := range links {
	// 	feed, err := utility.FeedParser(ctx, l.URL)
	// 	if err != nil {
	// 		log.Println(err)
	// 		continue
	// 	}
	// 	fmt.Printf("%s  (%s, via %s)\n", feed.Title, l.Type, l.Source)
	// 	for count, item := range feed.Items {
	// 		newItem := models.NewFeedItem(item)
	// 		if newItem.PublishedAt != nil && newItem.PublishedAt.Before(last_week) {
	// 			continue
	// 		}
	// 		if newItem.Description == "" {
	// 			fmt.Println("no description")
	// 		} else {
	// 			fmt.Println(newItem.Description)
	// 		}
	// 		if newItem.Image != nil {
	// 			fmt.Printf("%#v\n", newItem.Image)
	// 		} else {
	// 			fmt.Println("no image")
	// 		}
	// 		if len(newItem.Enclosures) > 0 {
	// 			fmt.Printf("%#v\n", newItem.Enclosures[0])
	// 		} else {
	// 			fmt.Println("no enclosure")
	// 		}
	// 		for _, author := range newItem.Authors {
	// 			fmt.Printf("%#v\n", author)
	// 		}
	// 		fmt.Println(newItem.Title, newItem.Links, newItem.PublishedAt, count)
	// 		fmt.Println("=================")
	// 	}
	// }

}
