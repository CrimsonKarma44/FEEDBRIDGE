package main

import (
	// "context"
	// "fmt"
	// "log"

	// "net"
	// "net/http"
	// "time"

	// "github.com/CrimsonKarma44/FEEDBRIDGE/API/models"
	// rssdetector "github.com/CrimsonKarma44/rss_detector"
	// "github.com/mmcdole/gofeed"
	// "google.golang.org/grpc"
	// "github.com/CrimsonKarma44/FEEDBRIDGE/API/config"
)

func main() {
	// env := config.LoadEnv()
	// db, err := config.NewDB(env)
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	// fmt.Printf("connection established: %v\n", db)

	// lis, err := net.Listen("tcp", ":50051")
	// if err != nil {
	// 	log.Fatalf("failed to listen: %v", err)
	// }
	// s := grpc.NewServer()
	// // pb.RegisterGreeterServer(s, &server{})
	// // pb.RegisterChatServer(s, &server{})
	// log.Println("Server running on :50051")
	// if err := s.Serve(lis); err != nil {
	// 	log.Fatalf("failed to serve: %v", err)
	// }
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

