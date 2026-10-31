// Package gateway serves the browser extension's HTTP/JSON API. It is a
// grpc-gateway reverse proxy: each request is translated into a call on the
// local gRPC server, so the gRPC handlers (and their interceptors) stay the
// only implementation. Routes come from proto/extension_http.yaml.
package gateway

import (
	"context"
	"net"
	"net/http"
	"time"

	extpb "github.com/CrimsonKarma44/FEEDBRIDGE/backend/protoAPI/extension"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config tunes the HTTP front of the gateway.
type Config struct {
	// Requests per second allowed per client IP, and the burst on top.
	Rate  rate.Limit
	Burst int
	// MaxBodyBytes caps request bodies.
	MaxBodyBytes int64
	// IdleTTL is how long an unused per-IP limiter is kept.
	IdleTTL time.Duration
}

// DefaultConfig allows 30 requests a minute per IP with bursts of 10.
func DefaultConfig() Config {
	return Config{
		Rate:         rate.Every(2 * time.Second),
		Burst:        10,
		MaxBodyBytes: 4 << 10,
		IdleTTL:      10 * time.Minute,
	}
}

// New returns the gateway handler. grpcTarget is the gRPC server to forward
// to; extra dial options are for tests.
func New(ctx context.Context, grpcTarget string, cfg Config, dialOpts ...grpc.DialOption) (http.Handler, error) {
	gw := runtime.NewServeMux()
	opts := append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, dialOpts...)
	if err := extpb.RegisterExtensionServiceHandlerFromEndpoint(ctx, gw, grpcTarget, opts); err != nil {
		return nil, err
	}

	limiter := newIPLimiter(cfg.Rate, cfg.Burst, cfg.IdleTTL)
	go limiter.janitor(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("/v1/", limitBody(cfg.MaxBodyBytes, limiter.middleware(gw)))
	return mux, nil
}

// DialTarget turns a gRPC listen address into one the gateway can dial:
// ":50051" and "0.0.0.0:50051" become "127.0.0.1:50051".
func DialTarget(listenAddr string) string {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return listenAddr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func limitBody(n int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, n)
		}
		next.ServeHTTP(w, r)
	})
}
