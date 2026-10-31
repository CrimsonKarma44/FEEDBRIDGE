package gateway

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	extpb "github.com/CrimsonKarma44/FEEDBRIDGE/backend/protoAPI/extension"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeExt struct {
	extpb.UnimplementedExtensionServiceServer
	gotURL string
}

func (f *fakeExt) Detect(_ context.Context, req *extpb.DetectRequest) (*extpb.DetectResponse, error) {
	f.gotURL = req.GetUrl()
	if req.GetUrl() == "https://nofeeds.example" {
		return nil, status.Error(codes.NotFound, "no feeds found")
	}
	return &extpb.DetectResponse{Url: req.GetUrl(), Feeds: []*extpb.DetectedFeed{
		{Url: "https://example.com/atom.xml", Title: "Example", Type: "atom", Source: "html_link"},
	}}, nil
}

func (f *fakeExt) CreateLinkToken(_ context.Context, req *extpb.CreateLinkTokenRequest) (*extpb.LinkToken, error) {
	f.gotURL = req.GetUrl()
	return &extpb.LinkToken{Token: "abcdefghijkl", ExpiresAt: timestamppb.New(time.Date(2026, 10, 31, 12, 15, 0, 0, time.UTC))}, nil
}

func (f *fakeExt) ResolveLinkToken(context.Context, *extpb.ResolveLinkTokenRequest) (*extpb.ResolveLinkTokenResponse, error) {
	return &extpb.ResolveLinkTokenResponse{Url: "https://secret.example"}, nil
}

func startGateway(t *testing.T, cfg Config) (*httptest.Server, *fakeExt) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	fake := &fakeExt{}
	extpb.RegisterExtensionServiceServer(s, fake)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h, err := New(ctx, "passthrough:///bufnet", cfg,
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, fake
}

func roomyConfig() Config {
	cfg := DefaultConfig()
	cfg.Rate, cfg.Burst = rate.Inf, 1
	return cfg
}

func TestGateway_Detect(t *testing.T) {
	srv, fake := startGateway(t, roomyConfig())
	res, err := http.Get(srv.URL + "/v1/detect?url=https%3A%2F%2Fexample.com%2Fblog")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	var body struct {
		URL   string `json:"url"`
		Feeds []struct {
			URL, Title, Type, Source string
		} `json:"feeds"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if fake.gotURL != "https://example.com/blog" || len(body.Feeds) != 1 || body.Feeds[0].Type != "atom" || body.Feeds[0].Title != "Example" {
		t.Fatalf("got %q, body %+v", fake.gotURL, body)
	}
}

func TestGateway_DetectNotFoundIs404(t *testing.T) {
	srv, _ := startGateway(t, roomyConfig())
	res, err := http.Get(srv.URL + "/v1/detect?url=https%3A%2F%2Fnofeeds.example")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status %d, want 404", res.StatusCode)
	}
}

func TestGateway_CreateLinkToken(t *testing.T) {
	srv, fake := startGateway(t, roomyConfig())
	res, err := http.Post(srv.URL+"/v1/link-tokens", "application/json",
		strings.NewReader(`{"url":"https://go.dev/blog/feed.atom"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", res.StatusCode, raw)
	}
	var body struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expiresAt"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body.Token != "abcdefghijkl" || body.ExpiresAt != "2026-10-31T12:15:00Z" || fake.gotURL != "https://go.dev/blog/feed.atom" {
		t.Fatalf("body %s, got url %q", raw, fake.gotURL)
	}
}

func TestGateway_ResolveIsNotExposed(t *testing.T) {
	srv, _ := startGateway(t, roomyConfig())
	for _, path := range []string{"/v1/link-tokens/abcdefghijkl", "/v1/resolve?token=abcdefghijkl"} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode == http.StatusOK || strings.Contains(string(raw), "secret.example") {
			t.Fatalf("%s reachable over HTTP: %d %s", path, res.StatusCode, raw)
		}
	}
}

func TestGateway_RateLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Rate, cfg.Burst = rate.Every(time.Hour), 2
	srv, _ := startGateway(t, cfg)
	codes := []int{}
	for i := 0; i < 3; i++ {
		res, err := http.Get(srv.URL + "/v1/detect?url=https%3A%2F%2Fexample.com")
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		codes = append(codes, res.StatusCode)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests {
		t.Fatalf("codes = %v, want [200 200 429]", codes)
	}
	// Health checks are never limited.
	res, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("healthz status %d", res.StatusCode)
	}
}

func TestGateway_BodyLimit(t *testing.T) {
	cfg := roomyConfig()
	cfg.MaxBodyBytes = 64
	srv, fake := startGateway(t, cfg)
	big := `{"url":"https://example.com/` + strings.Repeat("a", 200) + `"}`
	res, err := http.Post(srv.URL+"/v1/link-tokens", "application/json", strings.NewReader(big))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode < 400 || res.StatusCode >= 500 || fake.gotURL != "" {
		t.Fatalf("status %d, backend saw %q", res.StatusCode, fake.gotURL)
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		remote, xff, want string
	}{
		{"203.0.113.9:5000", "", "203.0.113.9"},
		{"203.0.113.9:5000", "1.2.3.4", "203.0.113.9"},              // untrusted proxy header ignored
		{"127.0.0.1:5000", "198.51.100.7", "198.51.100.7"},          // from Caddy
		{"127.0.0.1:5000", "6.6.6.6, 198.51.100.7", "198.51.100.7"}, // spoofed first entry ignored
		{"127.0.0.1:5000", "garbage", "127.0.0.1"},
		{"[::1]:5000", "198.51.100.8", "198.51.100.8"},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(http.MethodGet, "/v1/detect", nil)
		r.RemoteAddr = tc.remote
		if tc.xff != "" {
			r.Header.Set("X-Forwarded-For", tc.xff)
		}
		if got := clientIP(r); got != tc.want {
			t.Errorf("clientIP(%s, %q) = %q, want %q", tc.remote, tc.xff, got, tc.want)
		}
	}
}

func TestLimiterPrunesIdleClients(t *testing.T) {
	clock := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	l := newIPLimiter(rate.Inf, 1, 10*time.Minute)
	l.now = func() time.Time { return clock }
	l.allow("198.51.100.1")
	clock = clock.Add(11 * time.Minute)
	l.allow("198.51.100.2")
	l.prune()
	if _, ok := l.clients["198.51.100.1"]; ok || len(l.clients) != 1 {
		t.Fatalf("clients = %v", l.clients)
	}
}

func TestDialTarget(t *testing.T) {
	for in, want := range map[string]string{
		":50051":          "127.0.0.1:50051",
		"0.0.0.0:50051":   "127.0.0.1:50051",
		"127.0.0.1:50051": "127.0.0.1:50051",
		"[::]:50051":      "127.0.0.1:50051",
		"10.0.0.5:50051":  "10.0.0.5:50051",
	} {
		if got := DialTarget(in); got != want {
			t.Errorf("DialTarget(%q) = %q, want %q", in, got, want)
		}
	}
}
