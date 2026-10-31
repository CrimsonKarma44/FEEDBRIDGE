package handlers

import (
	"context"
	"net"
	"testing"
	"time"

	pb "github.com/CrimsonKarma44/FEEDBRIDGE/backend/protoAPI/extension"
	"github.com/CrimsonKarma44/FEEDBRIDGE/backend/service"
	rssdetector "github.com/CrimsonKarma44/rss_detector"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeFinder struct {
	links []rssdetector.FeedLink
	err   error
	calls []string
}

func (f *fakeFinder) Find(_ context.Context, url string) ([]rssdetector.FeedLink, error) {
	f.calls = append(f.calls, url)
	return f.links, f.err
}

type fakeTokens struct {
	urls map[string]string
}

func (f *fakeTokens) Create(_ context.Context, url string) (string, time.Time, error) {
	tok := "abcdefghijkl"
	f.urls[tok] = url
	return tok, time.Date(2026, 10, 31, 12, 15, 0, 0, time.UTC), nil
}

func (f *fakeTokens) Resolve(_ context.Context, token string, consume bool) (string, error) {
	if len(token) != 12 {
		return "", service.ErrLinkTokenInvalid
	}
	url, ok := f.urls[token]
	if !ok {
		return "", service.ErrLinkTokenNotFound
	}
	if consume {
		delete(f.urls, token)
	}
	return url, nil
}

func startExtServer(t *testing.T, h *ExtensionHandler) pb.ExtensionServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	pb.RegisterExtensionServiceServer(s, h)
	go func() { _ = s.Serve(lis) }()
	t.Cleanup(s.Stop)
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewExtensionServiceClient(conn)
}

func TestExtensionDetect_ReturnsFeeds(t *testing.T) {
	finder := &fakeFinder{links: []rssdetector.FeedLink{
		{URL: "https://example.com/atom.xml", Title: "Example", Type: rssdetector.FeedTypeAtom, Source: rssdetector.SourceHTMLLink},
		{URL: "", Type: rssdetector.FeedTypeRSS}, // dropped
		{URL: "https://example.com/odd", Type: "xml"},
	}}
	cli := startExtServer(t, &ExtensionHandler{Feeds: finder})

	res, err := cli.Detect(context.Background(), &pb.DetectRequest{Url: "  https://example.com/blog  "})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetUrl() != "https://example.com/blog" || len(finder.calls) != 1 || finder.calls[0] != "https://example.com/blog" {
		t.Fatalf("url=%q calls=%v", res.GetUrl(), finder.calls)
	}
	feeds := res.GetFeeds()
	if len(feeds) != 2 {
		t.Fatalf("feeds = %v", feeds)
	}
	if f := feeds[0]; f.GetTitle() != "Example" || f.GetType() != "atom" || f.GetSource() != "html_link" {
		t.Fatalf("first feed = %v", f)
	}
	if feeds[1].GetType() != "unknown" {
		t.Fatalf("unexpected type %q mapped to %q", "xml", feeds[1].GetType())
	}
}

func TestExtensionDetect_Errors(t *testing.T) {
	cases := []struct {
		name string
		url  string
		err  error
		want codes.Code
	}{
		{"empty", "", nil, codes.InvalidArgument},
		{"bad scheme", "ftp://example.com", nil, codes.InvalidArgument},
		{"no host", "https://", nil, codes.InvalidArgument},
		{"no feeds", "https://example.com", rssdetector.ErrNoFeeds, codes.NotFound},
		{"rate limited", "https://example.com", rssdetector.ErrRateLimited, codes.ResourceExhausted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cli := startExtServer(t, &ExtensionHandler{Feeds: &fakeFinder{err: tc.err}})
			_, err := cli.Detect(context.Background(), &pb.DetectRequest{Url: tc.url})
			if status.Code(err) != tc.want {
				t.Fatalf("code = %v, want %v (err %v)", status.Code(err), tc.want, err)
			}
		})
	}
}

func TestExtensionLinkTokens_CreateResolveConsume(t *testing.T) {
	tokens := &fakeTokens{urls: map[string]string{}}
	cli := startExtServer(t, &ExtensionHandler{Tokens: tokens})
	ctx := context.Background()

	tok, err := cli.CreateLinkToken(ctx, &pb.CreateLinkTokenRequest{Url: "https://go.dev/blog/feed.atom"})
	if err != nil {
		t.Fatal(err)
	}
	if tok.GetToken() != "abcdefghijkl" || !tok.GetExpiresAt().AsTime().Equal(time.Date(2026, 10, 31, 12, 15, 0, 0, time.UTC)) {
		t.Fatalf("token = %v", tok)
	}

	peek, err := cli.ResolveLinkToken(ctx, &pb.ResolveLinkTokenRequest{Token: tok.GetToken()})
	if err != nil || peek.GetUrl() != "https://go.dev/blog/feed.atom" {
		t.Fatalf("peek = %v, %v", peek, err)
	}
	used, err := cli.ResolveLinkToken(ctx, &pb.ResolveLinkTokenRequest{Token: tok.GetToken(), Consume: true})
	if err != nil || used.GetUrl() != "https://go.dev/blog/feed.atom" {
		t.Fatalf("consume = %v, %v", used, err)
	}
	_, err = cli.ResolveLinkToken(ctx, &pb.ResolveLinkTokenRequest{Token: tok.GetToken(), Consume: true})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("second consume code = %v, want NotFound", status.Code(err))
	}
	_, err = cli.ResolveLinkToken(ctx, &pb.ResolveLinkTokenRequest{Token: "bad"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("malformed code = %v, want InvalidArgument", status.Code(err))
	}
	_, err = cli.CreateLinkToken(ctx, &pb.CreateLinkTokenRequest{Url: "javascript:alert(1)"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("bad url code = %v, want InvalidArgument", status.Code(err))
	}
}

func TestCachedFinder_NilRedisDoesNotPanic(t *testing.T) {
	// With no Redis the finder must still answer; an invalid URL fails fast
	// in the detector without touching the network.
	_, err := CachedFinder{}.Find(context.Background(), "::not a url")
	if err == nil {
		t.Fatal("expected an error for an unparseable URL")
	}
}
