package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"regexp"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// LinkTokenTTL is how long a Telegram deep-link token stays valid.
	LinkTokenTTL = 15 * time.Minute

	linkTokenPrefix = "linktoken:"
	linkTokenBytes  = 9 // 12 base64url characters
)

// ErrLinkTokenNotFound means the token never existed, expired, or was used.
var ErrLinkTokenNotFound = errors.New("link token not found or expired")

// ErrLinkTokenInvalid means the token is not in the format CreateLinkToken makes.
var ErrLinkTokenInvalid = errors.New("link token is malformed")

var linkTokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{12}$`)

// KV is the subset of Redis the token store needs. *config.RedisDB satisfies it.
type KV interface {
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	GetDel(ctx context.Context, key string) (string, error)
}

// LinkTokens maps short tokens to feed URLs for Telegram deep links
// (t.me/<bot>?start=add_<token>). Tokens live only in Redis and expire, so
// nothing about them reaches Postgres.
type LinkTokens struct {
	kv  KV
	now func() time.Time
}

func NewLinkTokens(kv KV) *LinkTokens {
	return &LinkTokens{kv: kv, now: time.Now}
}

// Create stores url under a new random token and returns it with its expiry.
func (s *LinkTokens) Create(ctx context.Context, url string) (string, time.Time, error) {
	buf := make([]byte, linkTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", time.Time{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	if err := s.kv.Set(ctx, linkTokenPrefix+token, url, LinkTokenTTL); err != nil {
		return "", time.Time{}, err
	}
	return token, s.now().Add(LinkTokenTTL), nil
}

// Resolve returns the URL behind token. With consume it also deletes the
// token, so a deep link subscribes at most once.
func (s *LinkTokens) Resolve(ctx context.Context, token string, consume bool) (string, error) {
	if !linkTokenRe.MatchString(token) {
		return "", ErrLinkTokenInvalid
	}
	key := linkTokenPrefix + token
	var (
		url string
		err error
	)
	if consume {
		url, err = s.kv.GetDel(ctx, key)
	} else {
		url, err = s.kv.Get(ctx, key)
	}
	if errors.Is(err, redis.Nil) {
		return "", ErrLinkTokenNotFound
	}
	return url, err
}
