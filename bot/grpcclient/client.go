package grpcclient

import (
	"context"
	"time"

	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	urlpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/setUrl"
	subpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/subscription"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const callTimeout = 30 * time.Second

type Client struct {
	conn   *grpc.ClientConn
	feed   feedpb.FeedHandlerServiceClient
	setURL urlpb.SetUrlHandlerClient
	sub    subpb.SubscriptionServiceClient
}

func New(addr string) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &Client{
		conn:   conn,
		feed:   feedpb.NewFeedHandlerServiceClient(conn),
		setURL: urlpb.NewSetUrlHandlerClient(conn),
		sub:    subpb.NewSubscriptionServiceClient(conn),
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// SetUrl registers the URL with the API and returns the detected feed links,
// best-ranked first.
func (c *Client) SetUrl(ctx context.Context, url string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	resp, err := c.setURL.SetUrl(ctx, &urlpb.SetUrlRequest{Url: url})
	if err != nil {
		return nil, err
	}
	return resp.GetFeedLinks(), nil
}

// GetFeed returns feed items for url published strictly after from.
func (c *Client) GetFeed(ctx context.Context, url string, from time.Time) ([]*feedpb.GetFeedsResponse_Feed, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	resp, err := c.feed.GetFeed(ctx, &feedpb.GetFeedsRequest{
		Url:  url,
		From: timestamppb.New(from),
	})
	if err != nil {
		return nil, err
	}
	return resp.GetFeeds(), nil
}

func (c *Client) Subscribe(ctx context.Context, platform, externalID, url string) (*subpb.SubscribeResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	return c.sub.Subscribe(ctx, &subpb.SubscribeRequest{
		Platform: platform, ExternalId: externalID, Url: url,
	})
}

func (c *Client) ListSubscriptions(ctx context.Context, platform, externalID string) ([]*subpb.Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := c.sub.List(ctx, &subpb.ListRequest{Platform: platform, ExternalId: externalID})
	if err != nil {
		return nil, err
	}
	return resp.GetSubscriptions(), nil
}

func (c *Client) UnsubscribeByURL(ctx context.Context, platform, externalID, url string) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	_, err := c.sub.UnsubscribeByURL(ctx, &subpb.URLMutation{
		Platform: platform, ExternalId: externalID, Url: url,
	})
	return err
}

func (c *Client) SetEnabledByURL(ctx context.Context, platform, externalID, url string, enabled bool) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	_, err := c.sub.SetEnabledByURL(ctx, &subpb.URLMutation{
		Platform: platform, ExternalId: externalID, Url: url, Enabled: enabled,
	})
	return err
}

func (c *Client) SetInterval(ctx context.Context, platform, externalID string, d time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	_, err := c.sub.SetInterval(ctx, &subpb.SetIntervalRequest{
		Platform: platform, ExternalId: externalID, Interval: durationpb.New(d),
	})
	return err
}

func (c *Client) ListDue(ctx context.Context, platform string, limit int32) ([]*subpb.Subscription, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	resp, err := c.sub.ListDue(ctx, &subpb.ListDueRequest{Platform: platform, Limit: limit})
	if err != nil {
		return nil, err
	}
	return resp.GetSubscriptions(), nil
}

func (c *Client) AckCursor(ctx context.Context, id uint64, publishedAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	_, err := c.sub.AckCursor(ctx, &subpb.AckCursorRequest{
		Id: id, PublishedAt: timestamppb.New(publishedAt),
	})
	return err
}

func (c *Client) SetEnabled(ctx context.Context, id uint64, enabled bool) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	_, err := c.sub.SetEnabled(ctx, &subpb.SetEnabledRequest{Id: id, Enabled: enabled})
	return err
}

func (c *Client) Unsubscribe(ctx context.Context, id uint64) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	_, err := c.sub.Unsubscribe(ctx, &subpb.SubscriptionID{Id: id})
	return err
}

// FriendlyError converts gRPC errors into short user-facing messages.
func FriendlyError(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return "the feed service is unreachable, try again later"
	}
	switch st.Code() {
	case codes.InvalidArgument:
		return "no feeds were found for that URL"
	case codes.NotFound:
		return "no feeds were found for that URL"
	case codes.AlreadyExists:
		return "you're already subscribed to this feed"
	case codes.ResourceExhausted:
		return "that website is temporarily limiting automated requests - try again in a few minutes"
	case codes.PermissionDenied:
		return "that website is blocking automated access"
	case codes.Unavailable:
		return "the feed service is temporarily unavailable, try again later"
	default:
		return "something went wrong while contacting the feed service"
	}
}
