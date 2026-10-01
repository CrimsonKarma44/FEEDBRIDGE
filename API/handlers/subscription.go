package handlers

import (
	"context"
	"errors"
	"time"

	pb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/subscription"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/service"
	"github.com/CrimsonKarma44/FEEDBRIDGE/API/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"
)

type SubscriptionHandler struct {
	pb.UnimplementedSubscriptionServiceServer
	Subs *service.SubscribeService
}

func (h *SubscriptionHandler) Subscribe(ctx context.Context, req *pb.SubscribeRequest) (*pb.SubscribeResponse, error) {
	res, err := h.Subs.Subscribe(ctx, service.SubscribeRequest{
		Platform:   req.GetPlatform(),
		ExternalID: req.GetExternalId(),
		URL:        req.GetUrl(),
	})
	if err != nil {
		return nil, subscribeStatus(err)
	}
	return &pb.SubscribeResponse{
		Subscription: toProtoSub(res.Subscription),
		Created:      res.Created,
	}, nil
}

func (h *SubscriptionHandler) Unsubscribe(_ context.Context, req *pb.SubscriptionID) (*pb.Empty, error) {
	if err := h.Subs.Store().Remove(uint(req.GetId())); err != nil {
		return nil, subscribeStatus(err)
	}
	return &pb.Empty{}, nil
}

func (h *SubscriptionHandler) Get(_ context.Context, req *pb.SubscriptionID) (*pb.Subscription, error) {
	sub, err := h.Subs.Store().Get(uint(req.GetId()))
	if err != nil {
		return nil, subscribeStatus(err)
	}
	return toProtoSub(sub), nil
}

func (h *SubscriptionHandler) SetEnabled(_ context.Context, req *pb.SetEnabledRequest) (*pb.Empty, error) {
	if err := h.Subs.Store().SetEnabled(uint(req.GetId()), req.GetEnabled()); err != nil {
		return nil, subscribeStatus(err)
	}
	return &pb.Empty{}, nil
}

func (h *SubscriptionHandler) SetEnabledByURL(ctx context.Context, req *pb.URLMutation) (*pb.Empty, error) {
	url, err := h.Subs.ResolveURL(ctx, req.GetPlatform(), req.GetExternalId(), req.GetUrl())
	if err != nil {
		return nil, subscribeStatus(err)
	}
	if err := h.Subs.Store().SetEnabledByURL(req.GetPlatform(), req.GetExternalId(), url, req.GetEnabled()); err != nil {
		return nil, subscribeStatus(err)
	}
	return &pb.Empty{}, nil
}

func (h *SubscriptionHandler) UnsubscribeByURL(ctx context.Context, req *pb.URLMutation) (*pb.Empty, error) {
	url, err := h.Subs.ResolveURL(ctx, req.GetPlatform(), req.GetExternalId(), req.GetUrl())
	if err != nil {
		return nil, subscribeStatus(err)
	}
	if err := h.Subs.Store().RemoveByURL(req.GetPlatform(), req.GetExternalId(), url); err != nil {
		return nil, subscribeStatus(err)
	}
	return &pb.Empty{}, nil
}

func (h *SubscriptionHandler) SetInterval(_ context.Context, req *pb.SetIntervalRequest) (*pb.Empty, error) {
	d := req.GetInterval().AsDuration()
	if err := h.Subs.Store().SetInterval(req.GetPlatform(), req.GetExternalId(), d, time.Now().UTC()); err != nil {
		return nil, subscribeStatus(err)
	}
	return &pb.Empty{}, nil
}

func (h *SubscriptionHandler) List(_ context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	subs, err := h.Subs.Store().List(req.GetPlatform(), req.GetExternalId())
	if err != nil {
		return nil, subscribeStatus(err)
	}
	return toProtoList(subs), nil
}

func (h *SubscriptionHandler) ListDue(_ context.Context, req *pb.ListDueRequest) (*pb.ListResponse, error) {
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 25
	}
	now := time.Now().UTC()
	subs, err := h.Subs.Store().DueFor(req.GetPlatform(), now, limit)
	if err != nil {
		return nil, subscribeStatus(err)
	}
	if err := h.Subs.Store().MarkDue(subs, now); err != nil {
		return nil, subscribeStatus(err)
	}
	return toProtoList(subs), nil
}

func (h *SubscriptionHandler) AckCursor(_ context.Context, req *pb.AckCursorRequest) (*pb.Empty, error) {
	var pub time.Time
	if req.GetPublishedAt() != nil {
		pub = req.GetPublishedAt().AsTime()
	}
	if err := h.Subs.Store().AckCursor(uint(req.GetId()), pub, time.Now().UTC()); err != nil {
		return nil, subscribeStatus(err)
	}
	return &pb.Empty{}, nil
}

func toProtoList(subs []store.Subscription) *pb.ListResponse {
	out := make([]*pb.Subscription, 0, len(subs))
	for i := range subs {
		out = append(out, toProtoSub(&subs[i]))
	}
	return &pb.ListResponse{Subscriptions: out}
}

func toProtoSub(sub *store.Subscription) *pb.Subscription {
	if sub == nil {
		return nil
	}
	return &pb.Subscription{
		Id:                uint64(sub.ID),
		Platform:          sub.Platform,
		ExternalId:        sub.ExternalID,
		Url:               sub.URL,
		IntervalSeconds:   int32(sub.IntervalSeconds),
		Enabled:           sub.Enabled,
		LastSeenPublished: timestamppb.New(sub.LastSeenPublished),
		NextCheckAt:       timestamppb.New(sub.NextCheckAt),
	}
}

func subscribeStatus(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, service.ErrNoFeeds) {
		return status.Error(codes.NotFound, err.Error())
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	return status.Errorf(codes.Internal, "%v", err)
}
