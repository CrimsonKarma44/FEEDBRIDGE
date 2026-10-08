package youtube

import "testing"

func TestChannelIDFromFeedURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://www.youtube.com/feeds/videos.xml?channel_id=UCKy1dAqELo0zrOtPkf0eTMw", "UCKy1dAqELo0zrOtPkf0eTMw"},
		{"https://youtube.com/feeds/videos.xml?channel_id=UCKy1dAqELo0zrOtPkf0eTMw", "UCKy1dAqELo0zrOtPkf0eTMw"},
		{"https://www.youtube.com/feeds/videos.xml?playlist_id=UUabc", ""},
		{"https://news.ycombinator.com/rss", ""},
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", ""},
		{"not a url", ""},
	}
	for _, tc := range cases {
		if got := ChannelIDFromFeedURL(tc.in); got != tc.want {
			t.Errorf("ChannelIDFromFeedURL(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestUploadsPlaylistID(t *testing.T) {
	got := UploadsPlaylistID("UCKy1dAqELo0zrOtPkf0eTMw")
	want := "UUKy1dAqELo0zrOtPkf0eTMw"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if UploadsPlaylistID("nope") != "" {
		t.Fatal("expected empty for bad id")
	}
}
