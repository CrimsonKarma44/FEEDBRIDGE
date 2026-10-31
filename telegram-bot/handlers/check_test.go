package handlers

import (
	"strings"
	"testing"

	extpb "github.com/CrimsonKarma44/FEEDBRIDGE/backend/protoAPI/extension"
)

func TestLooksLikeURL(t *testing.T) {
	yes := []string{
		"https://go.dev/blog",
		"http://example.com",
		"HTTPS://EXAMPLE.COM/feed",
		"example.com",
		"go.dev/blog/",
		"www.example.co.uk/news?x=1",
		"blog.example.com:8443/rss",
		"  example.org  ",
	}
	no := []string{
		"",
		"hello",
		"/check example.com",
		"see example.com",   // more than one word
		"3.5",               // number
		"e.g",               // abbreviation
		"user@example.com",  // email
		"ftp://example.com", // other scheme
		"example.c0m",       // TLD with a digit
		".com",
		"thanks!",
	}
	for _, s := range yes {
		if !looksLikeURL(s) {
			t.Errorf("looksLikeURL(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if looksLikeURL(s) {
			t.Errorf("looksLikeURL(%q) = true, want false", s)
		}
	}
}

func TestCheckResultText(t *testing.T) {
	feeds := []*extpb.DetectedFeed{
		{Url: "https://go.dev/blog/feed.atom", Title: "The Go <Blog>", Type: "atom"},
		{Url: "https://go.dev/blog/rss?a=1&b=2", Type: "rss"},
	}
	text := checkResultText("go.dev", feeds, 2, true)
	for _, want := range []string{
		"<b>2 feeds found on go.dev</b>",
		"1. <b>The Go &lt;Blog&gt;</b> · Atom",
		"2. <b>go.dev</b> · RSS", // untitled feed falls back to its host
		"<code>https://go.dev/blog/rss?a=1&amp;b=2</code>",
		"buttons work for 15 minutes",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q:\n%s", want, text)
		}
	}

	one := checkResultText("example.com", feeds[:1], 1, false)
	if !strings.Contains(one, "1 feed found") || !strings.Contains(one, "/addfeed &lt;feed url&gt;") {
		t.Errorf("single feed without buttons:\n%s", one)
	}
	capped := checkResultText("github.com", feeds, 9, true)
	if !strings.Contains(capped, "Showing the first 2 of 9.") {
		t.Errorf("capped list:\n%s", capped)
	}
}

func TestCheckButtonLabel(t *testing.T) {
	f := &extpb.DetectedFeed{Url: "https://example.com/feed", Title: "A very long feed title that keeps going on and on"}
	if got := checkButtonLabel(0, f, 1); got != "Subscribe" {
		t.Errorf("single feed label = %q", got)
	}
	got := checkButtonLabel(1, f, 3)
	if !strings.HasPrefix(got, "2. Subscribe: A very long") || !strings.HasSuffix(got, "…") {
		t.Errorf("label = %q", got)
	}
	untitled := checkButtonLabel(0, &extpb.DetectedFeed{Url: "https://example.com/rss"}, 2)
	if !strings.Contains(untitled, "example.com/rss") {
		t.Errorf("untitled label = %q", untitled)
	}
}

func TestCheckCallbackFitsTelegram(t *testing.T) {
	if n := len("\fck_add|" + "130ps112PTd1"); n > 64 {
		t.Fatalf("callback data is %d bytes", n)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("Блог о Go и вебе", 8); got != "Блог о …" {
		t.Errorf("got %q", got)
	}
	if got := truncateRunes("short", 8); got != "short" {
		t.Errorf("got %q", got)
	}
}
