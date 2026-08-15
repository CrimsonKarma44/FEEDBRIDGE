// Package youtube resolves YouTube URLs to channel IDs and RSS feed URLs
// without scraping full HTML watch pages (which Google rate-limits heavily).
//
// Strategy (most reliable first):
//  1. Parse the URL — channel IDs already in the path need no network call.
//  2. Video URL  → Innertube player API  → videoDetails.channelId
//  3. Handle URL → Innertube resolve_url  → browseEndpoint.browseId
//  4. Optional: YouTube Data API v3 if YOUTUBE_API_KEY is set
//  5. Last resort: HTML scrape (often 429) — see html_scrape.go
//
// Once you have a channel ID, the public Atom feed is stable and rarely blocked:
//
//	https://www.youtube.com/feeds/videos.xml?channel_id=UC...
package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ---- Public types -----------------------------------------------------------

// Kind classifies what a YouTube URL points at.
type Kind int

const (
	KindUnknown Kind = iota
	KindVideo
	KindChannelID // /channel/UC...
	KindHandle    // /@name
	KindLegacyUser
	KindCustom // /c/name (old vanity)
)

func (k Kind) String() string {
	switch k {
	case KindVideo:
		return "video"
	case KindChannelID:
		return "channel_id"
	case KindHandle:
		return "handle"
	case KindLegacyUser:
		return "user"
	case KindCustom:
		return "custom"
	default:
		return "unknown"
	}
}

// Parsed is the result of inspecting a YouTube URL without hitting the network.
type Parsed struct {
	Kind      Kind
	RawURL    string
	VideoID   string // 11-char id when KindVideo
	ChannelID string // UC... when already present
	Handle    string // without leading @
	User      string // legacy /user/ name
}

// Channel holds a resolved channel and the feed URL you actually want for RSS.
type Channel struct {
	ID      string // UC...
	Title   string // optional; filled when available
	RSSURL  string
	Source  string // how we got it (for learning / debugging)
	VideoID string // if resolution started from a video
}

// ---- Resolver ---------------------------------------------------------------

// Resolver looks up channel IDs with caching and clear rate-limit errors.
type Resolver struct {
	HTTP      *http.Client
	APIKey    string // optional YouTube Data API v3 key
	UserAgent string

	mu    sync.RWMutex
	cache map[string]Channel // keyed by normalized input (video id, handle, …)
}

// NewResolver builds a resolver with sensible defaults.
// If apiKey is empty, YOUTUBE_API_KEY from the environment is used when set.
func NewResolver(apiKey string) *Resolver {
	if apiKey == "" {
		apiKey = os.Getenv("YOUTUBE_API_KEY")
	}
	return &Resolver{
		HTTP:   &http.Client{Timeout: 20 * time.Second},
		APIKey: apiKey,
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
			"(KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		cache: make(map[string]Channel),
	}
}

// Resolve turns any common YouTube URL into a Channel (ID + RSS feed URL).
func (r *Resolver) Resolve(ctx context.Context, rawURL string) (*Channel, error) {
	parsed, err := ParseURL(rawURL)
	if err != nil {
		return nil, err
	}

	cacheKey := parsed.cacheKey()
	if ch, ok := r.getCache(cacheKey); ok {
		ch.Source = ch.Source + " (cache)"
		return &ch, nil
	}

	var ch *Channel

	switch parsed.Kind {
	case KindChannelID:
		ch = &Channel{
			ID:     parsed.ChannelID,
			RSSURL: FeedURL(parsed.ChannelID),
			Source: "url path (/channel/UC...)",
		}

	case KindVideo:
		ch, err = r.resolveVideo(ctx, parsed.VideoID)
		if err != nil {
			return nil, err
		}

	case KindHandle:
		ch, err = r.resolveHandle(ctx, parsed.Handle)
		if err != nil {
			return nil, err
		}

	case KindLegacyUser, KindCustom:
		// Treat as vanity URL — resolve_url handles /user/ and /c/ too.
		ch, err = r.resolveVanityURL(ctx, parsed.RawURL)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported or unrecognised YouTube URL: %q", rawURL)
	}

	r.putCache(cacheKey, *ch)
	return ch, nil
}

// ---- URL parsing (no network) -----------------------------------------------

var (
	reVideoID   = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)
	reChannelID = regexp.MustCompile(`^UC[\w-]{22}$`)
)

// ParseURL classifies a YouTube link without making HTTP requests.
func ParseURL(raw string) (*Parsed, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty URL")
	}

	// Bare video id or channel id
	if reVideoID.MatchString(raw) {
		return &Parsed{Kind: KindVideo, RawURL: raw, VideoID: raw}, nil
	}
	if reChannelID.MatchString(raw) {
		return &Parsed{Kind: KindChannelID, RawURL: raw, ChannelID: raw}, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse URL: %w", err)
	}
	if u.Scheme == "" {
		// Allow youtube.com/... without scheme
		u, err = url.Parse("https://" + raw)
		if err != nil {
			return nil, fmt.Errorf("parse URL: %w", err)
		}
	}

	host := strings.ToLower(u.Hostname())
	if host != "youtube.com" && host != "www.youtube.com" &&
		host != "m.youtube.com" && host != "youtu.be" && host != "www.youtu.be" {
		return nil, fmt.Errorf("not a YouTube host: %s", host)
	}

	p := &Parsed{RawURL: raw}
	path := strings.Trim(u.Path, "/")
	parts := splitPath(path)

	// youtu.be/<videoId>
	if host == "youtu.be" || host == "www.youtu.be" {
		if len(parts) >= 1 && reVideoID.MatchString(parts[0]) {
			p.Kind = KindVideo
			p.VideoID = parts[0]
			return p, nil
		}
		return nil, fmt.Errorf("invalid youtu.be URL: %s", raw)
	}

	// ?v=VIDEO_ID
	if v := u.Query().Get("v"); reVideoID.MatchString(v) {
		p.Kind = KindVideo
		p.VideoID = v
		return p, nil
	}

	if len(parts) == 0 {
		return nil, fmt.Errorf("no path in URL: %s", raw)
	}

	switch {
	case parts[0] == "watch":
		return nil, fmt.Errorf("watch URL missing v= parameter: %s", raw)

	case parts[0] == "shorts" && len(parts) >= 2 && reVideoID.MatchString(parts[1]):
		p.Kind = KindVideo
		p.VideoID = parts[1]

	case parts[0] == "embed" && len(parts) >= 2 && reVideoID.MatchString(parts[1]):
		p.Kind = KindVideo
		p.VideoID = parts[1]

	case parts[0] == "live" && len(parts) >= 2 && reVideoID.MatchString(parts[1]):
		p.Kind = KindVideo
		p.VideoID = parts[1]

	case parts[0] == "channel" && len(parts) >= 2:
		id := parts[1]
		if !reChannelID.MatchString(id) {
			return nil, fmt.Errorf("invalid channel id in URL: %s", id)
		}
		p.Kind = KindChannelID
		p.ChannelID = id

	case strings.HasPrefix(parts[0], "@"):
		p.Kind = KindHandle
		p.Handle = strings.TrimPrefix(parts[0], "@")

	case parts[0] == "user" && len(parts) >= 2:
		p.Kind = KindLegacyUser
		p.User = parts[1]

	case parts[0] == "c" && len(parts) >= 2:
		p.Kind = KindCustom
		p.Handle = parts[1] // reuse Handle field for vanity segment

	default:
		return nil, fmt.Errorf("unrecognised YouTube path /%s", path)
	}

	return p, nil
}

func (p *Parsed) cacheKey() string {
	switch p.Kind {
	case KindVideo:
		return "v:" + p.VideoID
	case KindChannelID:
		return "c:" + p.ChannelID
	case KindHandle:
		return "h:" + strings.ToLower(p.Handle)
	case KindLegacyUser:
		return "u:" + strings.ToLower(p.User)
	case KindCustom:
		return "custom:" + strings.ToLower(p.Handle)
	default:
		return "raw:" + p.RawURL
	}
}

func splitPath(path string) []string {
	if path == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(path, "/") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// FeedURL is the public Atom feed for a channel — prefer this over scraping.
func FeedURL(channelID string) string {
	return "https://www.youtube.com/feeds/videos.xml?channel_id=" + url.QueryEscape(channelID)
}

// ---- Resolution backends ----------------------------------------------------

func (r *Resolver) resolveVideo(ctx context.Context, videoID string) (*Channel, error) {
	// 1) Official API when a key is available (best long-term choice).
	if r.APIKey != "" {
		ch, err := r.resolveVideoDataAPI(ctx, videoID)
		if err == nil {
			return ch, nil
		}
		// Fall through to Innertube; keep API error only if Innertube also fails.
		if ch, err2 := r.resolveVideoInnertube(ctx, videoID); err2 == nil {
			return ch, nil
		}
		return nil, fmt.Errorf("data api: %w", err)
	}
	return r.resolveVideoInnertube(ctx, videoID)
}

func (r *Resolver) resolveVideoInnertube(ctx context.Context, videoID string) (*Channel, error) {
	// Same JSON body the web player uses. No API key required for this public endpoint.
	body := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB",
				"clientVersion": "2.20240701.00.00",
				"hl":            "en",
				"gl":            "US",
			},
		},
		"videoId": videoID,
	}

	var resp struct {
		VideoDetails struct {
			VideoID   string `json:"videoId"`
			Title     string `json:"title"`
			ChannelID string `json:"channelId"`
			Author    string `json:"author"`
		} `json:"videoDetails"`
		PlayabilityStatus struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		} `json:"playabilityStatus"`
	}

	if err := r.postJSON(ctx, "https://www.youtube.com/youtubei/v1/player?prettyPrint=false", body, &resp); err != nil {
		return nil, fmt.Errorf("innertube player: %w", err)
	}
	if resp.VideoDetails.ChannelID == "" {
		reason := resp.PlayabilityStatus.Reason
		if reason == "" {
			reason = resp.PlayabilityStatus.Status
		}
		if reason == "" {
			reason = "empty channelId"
		}
		return nil, fmt.Errorf("innertube player: no channelId for video %s (%s)", videoID, reason)
	}

	return &Channel{
		ID:      resp.VideoDetails.ChannelID,
		Title:   resp.VideoDetails.Author,
		RSSURL:  FeedURL(resp.VideoDetails.ChannelID),
		Source:  "innertube /youtubei/v1/player",
		VideoID: videoID,
	}, nil
}

func (r *Resolver) resolveVideoDataAPI(ctx context.Context, videoID string) (*Channel, error) {
	u := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/videos?part=snippet&id=%s&key=%s",
		url.QueryEscape(videoID),
		url.QueryEscape(r.APIKey),
	)

	var resp struct {
		Items []struct {
			Snippet struct {
				ChannelID    string `json:"channelId"`
				ChannelTitle string `json:"channelTitle"`
				Title        string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := r.getJSON(ctx, u, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("youtube data api: %s", resp.Error.Message)
	}
	if len(resp.Items) == 0 || resp.Items[0].Snippet.ChannelID == "" {
		return nil, fmt.Errorf("youtube data api: video not found: %s", videoID)
	}

	sn := resp.Items[0].Snippet
	return &Channel{
		ID:      sn.ChannelID,
		Title:   sn.ChannelTitle,
		RSSURL:  FeedURL(sn.ChannelID),
		Source:  "YouTube Data API v3 videos.list",
		VideoID: videoID,
	}, nil
}

func (r *Resolver) resolveHandle(ctx context.Context, handle string) (*Channel, error) {
	// Data API forHandle when available
	if r.APIKey != "" {
		ch, err := r.resolveHandleDataAPI(ctx, handle)
		if err == nil {
			return ch, nil
		}
	}
	return r.resolveVanityURL(ctx, "https://www.youtube.com/@"+handle)
}

func (r *Resolver) resolveHandleDataAPI(ctx context.Context, handle string) (*Channel, error) {
	u := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/channels?part=snippet&forHandle=%s&key=%s",
		url.QueryEscape(handle),
		url.QueryEscape(r.APIKey),
	)

	var resp struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := r.getJSON(ctx, u, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("youtube data api: %s", resp.Error.Message)
	}
	if len(resp.Items) == 0 {
		return nil, fmt.Errorf("youtube data api: handle not found: @%s", handle)
	}

	return &Channel{
		ID:     resp.Items[0].ID,
		Title:  resp.Items[0].Snippet.Title,
		RSSURL: FeedURL(resp.Items[0].ID),
		Source: "YouTube Data API v3 channels.list forHandle",
	}, nil
}

func (r *Resolver) resolveVanityURL(ctx context.Context, pageURL string) (*Channel, error) {
	// navigation/resolve_url maps @handles, /c/, /user/ → browseId (UC...)
	body := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB",
				"clientVersion": "2.20240701.00.00",
				"hl":            "en",
				"gl":            "US",
			},
		},
		"url": pageURL,
	}

	var resp struct {
		Endpoint struct {
			BrowseEndpoint struct {
				BrowseID string `json:"browseId"`
			} `json:"browseEndpoint"`
		} `json:"endpoint"`
	}

	if err := r.postJSON(ctx, "https://www.youtube.com/youtubei/v1/navigation/resolve_url?prettyPrint=false", body, &resp); err != nil {
		return nil, fmt.Errorf("innertube resolve_url: %w", err)
	}

	id := resp.Endpoint.BrowseEndpoint.BrowseID
	if !reChannelID.MatchString(id) {
		return nil, fmt.Errorf("innertube resolve_url: no channel browseId for %s (got %q)", pageURL, id)
	}

	// Optional metadata (title + official rssUrl) via browse — non-fatal if it fails.
	title := ""
	if meta, err := r.browseChannel(ctx, id); err == nil {
		title = meta.Title
	}

	return &Channel{
		ID:     id,
		Title:  title,
		RSSURL: FeedURL(id),
		Source: "innertube /youtubei/v1/navigation/resolve_url",
	}, nil
}

type channelMeta struct {
	Title string
}

func (r *Resolver) browseChannel(ctx context.Context, channelID string) (*channelMeta, error) {
	body := map[string]any{
		"context": map[string]any{
			"client": map[string]any{
				"clientName":    "WEB",
				"clientVersion": "2.20240701.00.00",
				"hl":            "en",
				"gl":            "US",
			},
		},
		"browseId": channelID,
	}

	var resp struct {
		Metadata struct {
			ChannelMetadataRenderer struct {
				Title      string `json:"title"`
				ExternalID string `json:"externalId"`
				RSSURL     string `json:"rssUrl"`
			} `json:"channelMetadataRenderer"`
		} `json:"metadata"`
	}

	if err := r.postJSON(ctx, "https://www.youtube.com/youtubei/v1/browse?prettyPrint=false", body, &resp); err != nil {
		return nil, err
	}
	return &channelMeta{Title: resp.Metadata.ChannelMetadataRenderer.Title}, nil
}

// ---- HTTP helpers -----------------------------------------------------------

func (r *Resolver) postJSON(ctx context.Context, endpoint string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", r.UserAgent)
	req.Header.Set("Accept", "application/json")
	// Origin/referer make the client look a bit more like the real web app.
	req.Header.Set("Origin", "https://www.youtube.com")
	req.Header.Set("Referer", "https://www.youtube.com/")

	return r.doJSON(req, out)
}

func (r *Resolver) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", r.UserAgent)
	req.Header.Set("Accept", "application/json")
	return r.doJSON(req, out)
}

func (r *Resolver) doJSON(req *http.Request, out any) error {
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8 MiB cap
	if err != nil {
		return err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// continue
	case http.StatusTooManyRequests:
		return fmt.Errorf("%w: YouTube rate-limited this IP (HTTP 429) — back off and retry later", ErrRateLimited)
	case http.StatusForbidden:
		return fmt.Errorf("%w: forbidden (HTTP 403) — may need cookies, different IP, or Data API key", ErrBlocked)
	default:
		if resp.StatusCode >= 400 {
			// Captcha / sorry pages sometimes arrive as 302 followed by HTML;
			// clients that don't follow redirects still see 3xx here rarely.
			snippet := string(data)
			if len(snippet) > 200 {
				snippet = snippet[:200]
			}
			return fmt.Errorf("HTTP %s from %s: %s", resp.Status, req.URL.Host, snippet)
		}
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	return nil
}

// ---- Cache ------------------------------------------------------------------

func (r *Resolver) getCache(key string) (Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, ok := r.cache[key]
	return ch, ok
}

func (r *Resolver) putCache(key string, ch Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache[key] = ch
}

// ---- Sentinel errors --------------------------------------------------------

var (
	// ErrRateLimited means HTTP 429 / captcha pressure — wait, don't spam.
	ErrRateLimited = fmt.Errorf("rate limited")
	// ErrBlocked means the request was refused for bot/policy reasons.
	ErrBlocked = fmt.Errorf("blocked")
)
