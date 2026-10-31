package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// memKV is an in-memory KV with expiry, standing in for Redis.
type memKV struct {
	now  func() time.Time
	vals map[string]string
	exp  map[string]time.Time
}

func newMemKV(now func() time.Time) *memKV {
	return &memKV{now: now, vals: map[string]string{}, exp: map[string]time.Time{}}
}

func (m *memKV) Set(_ context.Context, key, value string, ttl time.Duration) error {
	m.vals[key] = value
	m.exp[key] = m.now().Add(ttl)
	return nil
}

func (m *memKV) Get(_ context.Context, key string) (string, error) {
	v, ok := m.vals[key]
	if !ok || !m.now().Before(m.exp[key]) {
		return "", redis.Nil
	}
	return v, nil
}

func (m *memKV) GetDel(ctx context.Context, key string) (string, error) {
	v, err := m.Get(ctx, key)
	delete(m.vals, key)
	delete(m.exp, key)
	return v, err
}

func TestLinkTokens_CreatePeekConsume(t *testing.T) {
	clock := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	kv := newMemKV(func() time.Time { return clock })
	s := NewLinkTokens(kv)
	s.now = func() time.Time { return clock }
	ctx := context.Background()

	tok, exp, err := s.Create(ctx, "https://go.dev/blog/feed.atom")
	if err != nil {
		t.Fatal(err)
	}
	if !linkTokenRe.MatchString(tok) {
		t.Fatalf("token %q has the wrong shape", tok)
	}
	if len("add_"+tok) > 64 {
		t.Fatalf("deep-link payload too long for Telegram: %d", len("add_"+tok))
	}
	if !exp.Equal(clock.Add(LinkTokenTTL)) {
		t.Fatalf("expires %v", exp)
	}

	for i := 0; i < 2; i++ { // peeking never uses the token up
		url, err := s.Resolve(ctx, tok, false)
		if err != nil || url != "https://go.dev/blog/feed.atom" {
			t.Fatalf("peek %d: url=%q err=%v", i, url, err)
		}
	}
	if url, err := s.Resolve(ctx, tok, true); err != nil || url != "https://go.dev/blog/feed.atom" {
		t.Fatalf("consume: url=%q err=%v", url, err)
	}
	if _, err := s.Resolve(ctx, tok, true); !errors.Is(err, ErrLinkTokenNotFound) {
		t.Fatalf("second consume err = %v, want ErrLinkTokenNotFound", err)
	}
}

func TestLinkTokens_Expires(t *testing.T) {
	clock := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	kv := newMemKV(func() time.Time { return clock })
	s := NewLinkTokens(kv)
	ctx := context.Background()

	tok, _, err := s.Create(ctx, "https://example.com/rss.xml")
	if err != nil {
		t.Fatal(err)
	}
	clock = clock.Add(LinkTokenTTL + time.Second)
	if _, err := s.Resolve(ctx, tok, false); !errors.Is(err, ErrLinkTokenNotFound) {
		t.Fatalf("err = %v, want ErrLinkTokenNotFound", err)
	}
}

func TestLinkTokens_RejectsMalformed(t *testing.T) {
	s := NewLinkTokens(newMemKV(time.Now))
	for _, tok := range []string{"", "short", "has space in it", "aaaaaaaaaaaaa", "aaaaaaaaaaa$"} {
		if _, err := s.Resolve(context.Background(), tok, false); !errors.Is(err, ErrLinkTokenInvalid) {
			t.Errorf("Resolve(%q) err = %v, want ErrLinkTokenInvalid", tok, err)
		}
	}
}

func TestLinkTokens_Unique(t *testing.T) {
	s := NewLinkTokens(newMemKV(time.Now))
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		tok, _, err := s.Create(context.Background(), "https://example.com/feed")
		if err != nil {
			t.Fatal(err)
		}
		if seen[tok] {
			t.Fatalf("duplicate token %q", tok)
		}
		seen[tok] = true
	}
}
