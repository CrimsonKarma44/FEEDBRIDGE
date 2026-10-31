package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/CrimsonKarma44/FEEDBRIDGE/backend/config"
	"github.com/CrimsonKarma44/FEEDBRIDGE/backend/gateway"
	handler "github.com/CrimsonKarma44/FEEDBRIDGE/backend/handlers"
	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/backend/protoAPI/Feed"
	extpb "github.com/CrimsonKarma44/FEEDBRIDGE/backend/protoAPI/extension"
	pb "github.com/CrimsonKarma44/FEEDBRIDGE/backend/protoAPI/setUrl"
	subpb "github.com/CrimsonKarma44/FEEDBRIDGE/backend/protoAPI/subscription"
	"github.com/CrimsonKarma44/FEEDBRIDGE/backend/service"
	apistore "github.com/CrimsonKarma44/FEEDBRIDGE/backend/store"
	"github.com/CrimsonKarma44/FEEDBRIDGE/backend/youtube"
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
	subStore, err := apistore.New(db.DB)
	if err != nil {
		logger.Fatalf("subscription store: %v", err)
	}
	subSvc := service.NewSubscribeService(subStore, handler.ResolverDetector{YouTube: ytResolver})

	// Registering gRPC services
	srvUrl := &handler.SetUrlHandler{LinkService: linkService, RedisClient: redis, YouTube: ytResolver}
	srvFeed := &handler.FeedHandler{LinkService: linkService, RedisClient: redis, YouTube: ytResolver}
	s := grpc.NewServer(grpc.ChainUnaryInterceptor(
		handler.LoggingInterceptor,
		handler.RecoveryInterceptor,
	))
	pb.RegisterSetUrlHandlerServer(s, srvUrl)
	feedpb.RegisterFeedHandlerServiceServer(s, srvFeed)
	subpb.RegisterSubscriptionServiceServer(s, &handler.SubscriptionHandler{Subs: subSvc})
	extpb.RegisterExtensionServiceServer(s, &handler.ExtensionHandler{
		Feeds:  handler.CachedFinder{Redis: redis, YouTube: ytResolver},
		Tokens: service.NewLinkTokens(redis),
	})

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(s, healthSrv)
	reflection.Register(s)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := s.Serve(lis); err != nil {
			logger.Fatalf("failed to serve: %v", err)
		}
	}()

	// Optional HTTP/JSON gateway for the browser extension. Off unless
	// HTTP_LISTEN_ADDR is set; on the VM it binds loopback behind Caddy.
	var httpSrv *http.Server
	if httpAddr := os.Getenv("HTTP_LISTEN_ADDR"); httpAddr != "" {
		gw, gerr := gateway.New(ctx, gateway.DialTarget(listenAddr), gateway.DefaultConfig())
		if gerr != nil {
			logger.Fatalf("http gateway: %v", gerr)
		}
		httpSrv = &http.Server{
			Addr:              httpAddr,
			Handler:           gw,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      40 * time.Second, // detection can take up to 30s
			IdleTimeout:       60 * time.Second,
		}
		go func() {
			logger.Printf("http gateway listening on %s", httpAddr)
			if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Fatalf("http gateway failed: %v", err)
			}
		}()
	}

	<-ctx.Done()
	logger.Println("shutdown signal received")

	if httpSrv != nil {
		sctx, scancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := httpSrv.Shutdown(sctx); err != nil {
			logger.Printf("http gateway shutdown: %v", err)
		}
		scancel()
	}

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
}
