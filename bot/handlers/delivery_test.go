package handlers

import (
	"fmt"
	"html"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func feedItem(title string, links ...string) *feedpb.GetFeedsResponse_Feed {
	return &feedpb.GetFeedsResponse_Feed{
		Title:       title,
		Links:       links,
		PublishedAt: timestamppb.New(time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)),
		SourceTitle: "Example",
	}
}

func assertBalancedHTML(t *testing.T, s string) {
	t.Helper()
	if utf8.RuneCountInString(s) > maxMessageLen {
		t.Fatalf("message length %d exceeds %d", utf8.RuneCountInString(s), maxMessageLen)
	}
	opens := strings.Count(s, "<a href=")
	closes := strings.Count(s, "</a>")
	if opens != closes {
		t.Fatalf("unbalanced <a> tags: %d open, %d close\n%s", opens, closes, s)
	}
	if lastOpen := strings.LastIndex(s, "<a href="); lastOpen >= 0 {
		if strings.LastIndex(s, "</a>") < lastOpen {
			t.Fatalf("last <a> is unclosed:\n%s", s)
		}
	}
	for _, tag := range []string{"<b>", "<i>"} {
		end := "</" + tag[1:]
		if strings.Count(s, tag) != strings.Count(s, end) {
			t.Fatalf("unbalanced %s tags\n%s", tag, s)
		}
	}
}

func TestFormatDigestPages_SplitsWithoutBreakingTags(t *testing.T) {
	longURL := "https://example.com/article?" + strings.Repeat("x=1&", 400)
	if len(longURL) > maxURLLen {
		longURL = longURL[:maxURLLen]
	}
	items := make([]*feedpb.GetFeedsResponse_Feed, 30)
	for i := range items {
		items[i] = feedItem(fmt.Sprintf("Headline %d <b>bold</b>", i), longURL)
	}

	pages := FormatDigestPages(items)
	if len(pages) < 2 {
		t.Fatalf("expected multiple pages for long URLs, got %d", len(pages))
	}

	var combined strings.Builder
	for i, page := range pages {
		assertBalancedHTML(t, page)
		if !strings.Contains(page, "new items") {
			t.Fatalf("page %d missing header", i)
		}
		combined.WriteString(page)
	}
	for i := range items {
		if !strings.Contains(combined.String(), fmt.Sprintf("%d. ", i+1)) {
			t.Fatalf("item %d missing from digest pages", i+1)
		}
	}
}

func TestFormatDigest_EscapesHTMLInTitle(t *testing.T) {
	item := feedItem(`Read <a href="http://evil.example">this</a> <b>now</b>`, "https://example.com/post")
	out := FormatDigest([]*feedpb.GetFeedsResponse_Feed{item})
	assertBalancedHTML(t, out)
	if strings.Count(out, "<a href=") != 1 {
		t.Fatalf("expected a single digest link, got:\n%s", out)
	}
	if strings.Contains(out, `<a href="http://evil.example">`) {
		t.Fatalf("title HTML leaked into digest:\n%s", out)
	}
	if !strings.Contains(out, "Read") || !strings.Contains(out, "this") {
		t.Fatalf("stripped title missing from digest:\n%s", out)
	}
}

func TestFormatItem_EscapesHTMLInTitle(t *testing.T) {
	item := feedItem(`Click <a href="http://evil.example">here</a>`, "https://example.com/post")
	item.Description = `See <a href="http://evil.example">more</a>`
	out := FormatItem(item)
	assertBalancedHTML(t, out)
	if strings.Count(out, "<a href=") != 1 {
		t.Fatalf("expected a single item link, got:\n%s", out)
	}
	if strings.Contains(out, "evil.example") {
		t.Fatalf("unescaped title/description HTML leaked:\n%s", out)
	}
}

func TestHrefURL_RejectsUnsafeAndNonHTTP(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"https://example.com/x?a=1&b=2", true},
		{"http://example.com/x", true},
		{"javascript:alert(1)", false},
		{"", false},
		{"/relative", false},
		{"https://example.com/foo\"bar", false},
		{"https://example.com/<script>", false},
		{"https://exa\nmple.com", false},
		{"ftp://example.com/file", false},
		{"https://example.com/" + strings.Repeat("a", maxURLLen), false},
	}
	for _, tc := range cases {
		got := hrefURL(tc.in)
		if tc.want && got == "" {
			t.Errorf("hrefURL(%q) rejected, want keep", tc.in)
		}
		if !tc.want && got != "" {
			t.Errorf("hrefURL(%q) = %q, want reject", tc.in, got)
		}
	}
}

func TestFormatDigest_OmitsUnsafeLinks(t *testing.T) {
	items := []*feedpb.GetFeedsResponse_Feed{
		feedItem("JS", "javascript:alert(1)"),
		feedItem("Quoted", `https://example.com/foo"bar`),
		feedItem("Amp", "https://example.com/x?a=1&b=2"),
	}
	out := strings.Join(FormatDigestPages(items), "")
	assertBalancedHTML(t, out)
	if strings.Contains(out, "javascript:") {
		t.Fatalf("javascript URL was wrapped:\n%s", out)
	}
	if strings.Contains(out, `href="https://example.com/foo`) && strings.Contains(out, "Quoted") {
		t.Fatalf("quoted URL was wrapped:\n%s", out)
	}
	if !strings.Contains(out, `href="https://example.com/x?a=1&amp;b=2"`) {
		t.Fatalf("ampersand in URL was not escaped:\n%s", out)
	}
}

func TestFormatItem_NeverTruncatesOpenTag(t *testing.T) {
	item := feedItem(strings.Repeat("标题", 80), "https://example.com/"+strings.Repeat("p", 400))
	item.Description = strings.Repeat("desc <b>x</b> ", 80)
	out := FormatItem(item)
	assertBalancedHTML(t, out)
	if strings.Contains(out, "<b>") && !strings.Contains(out, "</b>") {
		t.Fatalf("truncated <b>:\n%s", out)
	}
}

func TestHtmlToPlain_StripsTags(t *testing.T) {
	in := `1. <a href="https://example.com">Hello &amp; Co</a> <i>(28 Aug)</i>`
	got := htmlToPlain(in)
	if strings.ContainsAny(got, "<>") {
		t.Fatalf("tags remain: %q", got)
	}
	if !strings.Contains(got, "Hello & Co") {
		t.Fatalf("entities not unescaped: %q", got)
	}
}

func TestFormatDigestPages_Empty(t *testing.T) {
	if pages := FormatDigestPages(nil); pages != nil {
		t.Fatalf("got %v, want nil", pages)
	}
}

func TestFormatAnchor_EscapesHref(t *testing.T) {
	got := formatAnchor("https://example.com/x?a=1&b=2", `A & B`)
	if !strings.Contains(got, "&amp;b=2") {
		t.Fatalf("href not escaped: %s", got)
	}
	if !strings.Contains(got, "A &amp; B") {
		t.Fatalf("text not escaped: %s", got)
	}
	if html.UnescapeString(got) == got {
		t.Fatalf("expected escaped HTML, got %s", got)
	}
}
