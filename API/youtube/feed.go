package youtube

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

const dataAPIMaxItems = 15

// ChannelIDFromFeedURL returns the UC… id when raw is a YouTube channel Atom URL.
func ChannelIDFromFeedURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host != "www.youtube.com" && host != "youtube.com" && host != "m.youtube.com" {
		return ""
	}
	if u.Path != "/feeds/videos.xml" {
		return ""
	}
	id := u.Query().Get("channel_id")
	if !reChannelID.MatchString(id) {
		return ""
	}
	return id
}

// UploadsPlaylistID is the channel's uploads playlist (UC… → UU…).
func UploadsPlaylistID(channelID string) string {
	if !strings.HasPrefix(channelID, "UC") || len(channelID) < 3 {
		return ""
	}
	return "UU" + channelID[2:]
}

// FetchChannelFeed loads recent uploads via Data API v3 when videos.xml is blocked.
func (r *Resolver) FetchChannelFeed(ctx context.Context, feedURL string) (*gofeed.Feed, error) {
	if r == nil || r.APIKey == "" {
		return nil, fmt.Errorf("youtube data api: no API key")
	}
	chID := ChannelIDFromFeedURL(feedURL)
	if chID == "" {
		return nil, fmt.Errorf("youtube data api: not a channel RSS URL")
	}
	playlist := UploadsPlaylistID(chID)
	if playlist == "" {
		return nil, fmt.Errorf("youtube data api: bad channel id %q", chID)
	}

	endpoint := fmt.Sprintf(
		"https://www.googleapis.com/youtube/v3/playlistItems?part=snippet,contentDetails&maxResults=%d&playlistId=%s&key=%s",
		dataAPIMaxItems,
		url.QueryEscape(playlist),
		url.QueryEscape(r.APIKey),
	)

	var resp struct {
		Items []struct {
			Snippet struct {
				PublishedAt  string `json:"publishedAt"`
				ChannelID    string `json:"channelId"`
				Title        string `json:"title"`
				Description  string `json:"description"`
				ChannelTitle string `json:"channelTitle"`
				Thumbnails   struct {
					High struct {
						URL string `json:"url"`
					} `json:"high"`
					Medium struct {
						URL string `json:"url"`
					} `json:"medium"`
				} `json:"thumbnails"`
				ResourceID struct {
					VideoID string `json:"videoId"`
				} `json:"resourceId"`
			} `json:"snippet"`
			ContentDetails struct {
				VideoID string `json:"videoId"`
			} `json:"contentDetails"`
		} `json:"items"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := r.getJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("youtube data api: %s", resp.Error.Message)
	}

	title := "YouTube"
	feed := &gofeed.Feed{
		Title:    title,
		Link:     "https://www.youtube.com/channel/" + chID,
		FeedLink: FeedURL(chID),
		FeedType: "atom",
		Items:    make([]*gofeed.Item, 0, len(resp.Items)),
	}

	for _, it := range resp.Items {
		videoID := it.ContentDetails.VideoID
		if videoID == "" {
			videoID = it.Snippet.ResourceID.VideoID
		}
		if videoID == "" {
			continue
		}
		if it.Snippet.ChannelTitle != "" {
			feed.Title = it.Snippet.ChannelTitle
		}
		watch := "https://www.youtube.com/watch?v=" + videoID
		item := &gofeed.Item{
			Title:       it.Snippet.Title,
			Description: it.Snippet.Description,
			Link:        watch,
			Links:       []string{watch},
			GUID:        watch,
		}
		if ts, err := time.Parse(time.RFC3339, it.Snippet.PublishedAt); err == nil {
			t := ts.UTC()
			item.PublishedParsed = &t
			item.Published = it.Snippet.PublishedAt
		}
		thumb := it.Snippet.Thumbnails.High.URL
		if thumb == "" {
			thumb = it.Snippet.Thumbnails.Medium.URL
		}
		if thumb != "" {
			item.Image = &gofeed.Image{URL: thumb}
		}
		feed.Items = append(feed.Items, item)
	}
	if len(feed.Items) == 0 {
		return nil, fmt.Errorf("youtube data api: no uploads for %s", chID)
	}
	return feed, nil
}
