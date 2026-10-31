package gateway

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipLimiter keeps one token bucket per client IP and forgets idle ones, so
// memory stays bounded on the 1 GB VM.
type ipLimiter struct {
	mu      sync.Mutex
	clients map[string]*client
	rate    rate.Limit
	burst   int
	idle    time.Duration
	now     func() time.Time
}

type client struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

func newIPLimiter(r rate.Limit, burst int, idle time.Duration) *ipLimiter {
	return &ipLimiter{
		clients: map[string]*client{},
		rate:    r,
		burst:   burst,
		idle:    idle,
		now:     time.Now,
	}
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.clients[ip]
	if !ok {
		c = &client{lim: rate.NewLimiter(l.rate, l.burst)}
		l.clients[ip] = c
	}
	c.lastSeen = l.now()
	return c.lim.AllowN(c.lastSeen, 1)
}

func (l *ipLimiter) prune() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := l.now().Add(-l.idle)
	for ip, c := range l.clients {
		if c.lastSeen.Before(cutoff) {
			delete(l.clients, ip)
		}
	}
}

func (l *ipLimiter) janitor(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			l.prune()
		}
	}
}

func (l *ipLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r)) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "10")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"code":8,"message":"too many requests, try again shortly"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP returns the caller's IP. X-Forwarded-For is trusted only when the
// request comes from loopback (Caddy on the same VM); its last entry is the
// address Caddy saw. Anyone else's header is ignored, so clients can't dodge
// the limit by sending their own X-Forwarded-For.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if last := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(last) != nil {
				return last
			}
		}
	}
	return host
}
