package handlers

import (
	"context"
	"log"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ChainUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		h := handler
		for i := len(interceptors) - 1; i >= 0; i-- {
			current, next := interceptors[i], h
			h = func(ctx context.Context, req any) (any, error) {
				return current(ctx, req, info, next)
			}
		}
		return h(ctx, req)
	}
}

func LoggingInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	code := codes.OK
	if err != nil {
		code = status.Code(err)
	}
	log.Printf("%s -> %s (%s)", info.FullMethod, code, time.Since(start))
	return resp, err
}

func RecoveryInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic recovered in %s: %v\n%s", info.FullMethod, r, debug.Stack())
			err = status.Error(codes.Internal, "internal server error")
		}
	}()
	return handler(ctx, req)
}
