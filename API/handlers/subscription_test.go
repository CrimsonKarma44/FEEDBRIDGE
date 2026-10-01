package handlers

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/API/internal/testdb"
	pb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/subscription"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/service"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/durationpb"
)

type fakeDetector struct {
	links []service.FeedLink
}

func (f fakeDetector) Detect(context.Context, string) ([]service.FeedLink, error) {
	return f.links, nil
}

func startSubServer(t *testing.T, d service.Detector) pb.SubscriptionServiceClient {
	t.Helper()
	st, err := store.New(testdb.Open(t))
	if err != nil {
		t.Fatal(err)
	}
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	pb.RegisterSubscriptionServiceServer(s, &SubscriptionHandler{
		Subs: service.NewSubscribeService(st, d),
	})
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
	return pb.NewSubscriptionServiceClient(conn)
}

func TestSubscriptionRPC_SubscribeListUnsubscribe(t *testing.T) {
	cli := startSubServer(t, fakeDetector{links: []service.FeedLink{
		{URL: "https://example.com/atom.xml"},
		{URL: "https://example.com/rss.xml"},
	}})
	ctx := context.Background()
	res, err := cli.Subscribe(ctx, &pb.SubscribeRequest{
		Platform: "telegram", ExternalId: "99", Url: "https://example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.GetCreated() || res.GetSubscription().GetUrl() != "https://example.com/atom.xml" {
		t.Fatalf("%v", res)
	}
	list, err := cli.List(ctx, &pb.ListRequest{Platform: "telegram", ExternalId: "99"})
	if err != nil || len(list.GetSubscriptions()) != 1 {
		t.Fatalf("list=%v err=%v", list, err)
	}
	again, err := cli.Subscribe(ctx, &pb.SubscribeRequest{
		Platform: "telegram", ExternalId: "99", Url: "https://example.com",
	})
	if err != nil || again.GetCreated() {
		t.Fatalf("second created=%v err=%v", again.GetCreated(), err)
	}
	_, err = cli.UnsubscribeByURL(ctx, &pb.URLMutation{
		Platform: "telegram", ExternalId: "99", Url: "https://example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	list, err = cli.List(ctx, &pb.ListRequest{Platform: "telegram", ExternalId: "99"})
	if err != nil || len(list.GetSubscriptions()) != 0 {
		t.Fatalf("after unsub n=%d err=%v", len(list.GetSubscriptions()), err)
	}
}

func TestSubscriptionRPC_SetIntervalAndGet(t *testing.T) {
	cli := startSubServer(t, fakeDetector{links: []service.FeedLink{{URL: "https://example.com/feed.xml"}}})
	ctx := context.Background()
	res, err := cli.Subscribe(ctx, &pb.SubscribeRequest{
		Platform: "telegram", ExternalId: "7", Url: "https://example.com/feed.xml",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = cli.SetInterval(ctx, &pb.SetIntervalRequest{
		Platform: "telegram", ExternalId: "7", Interval: durationpb.New(30 * time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := cli.Get(ctx, &pb.SubscriptionID{Id: res.GetSubscription().GetId()})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetIntervalSeconds() != 1800 {
		t.Fatalf("interval=%d", got.GetIntervalSeconds())
	}
}

func TestSubscriptionRPC_UnsubscribeMissing(t *testing.T) {
	cli := startSubServer(t, fakeDetector{})
	_, err := cli.Unsubscribe(context.Background(), &pb.SubscriptionID{Id: 999})
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Fatalf("code=%v err=%v", st.Code(), err)
	}
}
