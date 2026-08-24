package grpcclient

import (
	"context"
	"time"

	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	urlpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/setUrl"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const callTimeout = 30 * time.Second

type Client struct {
	conn   *grpc.ClientConn
	feed   feedpb.FeedHandlerServiceClient
	setURL urlpb.SetUrlHandlerClient
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
