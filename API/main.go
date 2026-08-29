package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/CrimsonKarma44/FEEDBRIDGE/API/config"
	handler "github.com/CrimsonKarma44/FEEDBRIDGE/API/handlers"
	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	pb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/setUrl"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/service"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/youtube"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
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

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = "127.0.0.1:50051"
	}
	lis, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logger.Fatalf("failed to listen on %s: %v", listenAddr, err)
	}
	logger.Printf("listening on %s", listenAddr)

	// Initializing link service
	linkService := service.NewLinkRepoService(db.DB) // redis
	ytResolver := youtube.NewResolver(env.YoutubeAPIKey)

	// Registering gRPC services
	srvUrl := &handler.SetUrlHandler{LinkService: linkService, RedisClient: redis, YouTube: ytResolver}
	srvFeed := &handler.FeedHandler{LinkService: linkService, RedisClient: redis, YouTube: ytResolver}
	s := grpc.NewServer(grpc.ChainUnaryInterceptor(
		handler.LoggingInterceptor,
		handler.RecoveryInterceptor,
	))
	pb.RegisterSetUrlHandlerServer(s, srvUrl)
	feedpb.RegisterFeedHandlerServiceServer(s, srvFeed)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, healthSrv)
	reflection.Register(s)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Println("Server running on :50051")
		if err := s.Serve(lis); err != nil {
			logger.Fatalf("failed to serve: %v", err)
		}
	}()

	<-ctx.Done()
	logger.Println("shutdown signal received")

	done := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		logger.Println("graceful stop timed out, forcing shutdown")
		s.Stop()
	}

	if serr := redis.Close(); serr != nil {
		logger.Printf("redis close failed: %v", serr)
	}
	if sqlDB, derr := db.DB.DB(); derr == nil {
		if cerr := sqlDB.Close(); cerr != nil {
			logger.Printf("db close failed: %v", cerr)
		}
	}
	logger.Println("shutdown complete")

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
