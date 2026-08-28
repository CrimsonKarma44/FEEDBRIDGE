package handlers

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
)

const (
	maxMessageLen   = 4000
	snippetLen      = 300
	digestThreshold = 5
	sendGap         = 50 * time.Millisecond
)

var tagRe = regexp.MustCompile(`<[^>]*>`)
var spaceRe = regexp.MustCompile(`\s+`)

func cleanText(s string, max int) string {
	s = tagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.TrimSpace(spaceRe.ReplaceAllString(s, " "))
	runes := []rune(s)
	if len(runes) > max {
		return string(runes[:max]) + "..."
	}
	return s
}

// hrefURL returns a Telegram-safe http(s) URL, or empty if the link must not
// be placed in an <a href>. Telegram's HTML parser is not a full HTML parser:
// a quote, angle bracket, or truncated href is enough to reject the message.
func hrefURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" || len(s) > maxURLLen {
		return ""
	}
	for _, r := range s {
		if r < 32 || r == 127 || unicode.IsControl(r) {
			return ""
		}
	}
	if strings.ContainsAny(s, `<>"'`) {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	if u.Host == "" {
		return ""
	}
	return s
}

func firstHref(item *feedpb.GetFeedsResponse_Feed) string {
	for _, l := range item.GetLinks() {
		if h := hrefURL(l); h != "" {
			return h
		}
	}
	return ""
}

func itemTitle(item *feedpb.GetFeedsResponse_Feed, max int) string {
	t := cleanText(item.GetTitle(), max)
	if t == "" {
		return "(untitled)"
	}
	return t
}

func formatAnchor(link, text string) string {
	return fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(link), html.EscapeString(text))
}

// FormatItem renders one feed item as a Telegram HTML message. Tags are never
// byte-sliced: if the full card is too long, description then link are dropped.
func FormatItem(item *feedpb.GetFeedsResponse_Feed) string {
	msg := formatItem(item, true, true)
	if utf8.RuneCountInString(msg) <= maxMessageLen {
		return msg
	}
	msg = formatItem(item, true, false)
	if utf8.RuneCountInString(msg) <= maxMessageLen {
		return msg
	}
	return formatItem(item, false, false)
}

func formatItem(item *feedpb.GetFeedsResponse_Feed, withLink, withDesc bool) string {
	var b strings.Builder

	title := itemTitle(item, 200)
	link := ""
	if withLink {
		link = firstHref(item)
	}

	b.WriteString("<b>")
	if link != "" {
		b.WriteString(formatAnchor(link, title))
	} else {
		b.WriteString(html.EscapeString(title))
	}
	b.WriteString("</b>\n")

	if withDesc {
		if desc := item.GetDescription(); desc != "" {
			fmt.Fprintf(&b, "%s\n", html.EscapeString(cleanText(desc, snippetLen)))
		}
	}

	var meta []string
	if src := item.GetSourceTitle(); src != "" {
		meta = append(meta, "via "+cleanText(src, 60))
	}
	if item.GetPublishedAt() != nil {
		ts := item.GetPublishedAt().AsTime().UTC()
		meta = append(meta, ts.Format("02 Jan 2006, 15:04 UTC"))
	}
	if len(meta) > 0 {
		fmt.Fprintf(&b, "\n<i>%s</i>", html.EscapeString(strings.Join(meta, " · ")))
	}
	return b.String()
}

// FormatDigest renders many new items into one message. Prefer
// FormatDigestPages when sending so long batches are split instead of sliced.
func FormatDigest(items []*feedpb.GetFeedsResponse_Feed) string {
	pages := FormatDigestPages(items)
	if len(pages) == 0 {
		return ""
	}
	return pages[0]
}

func digestMixed(items []*feedpb.GetFeedsResponse_Feed) bool {
	sources := make(map[string]struct{})
	for _, item := range items {
		if src := item.GetSourceTitle(); src != "" {
			sources[src] = struct{}{}
		}
	}
	return len(sources) > 1
}

func digestHeader(n int, continued bool) string {
	if continued {
		return fmt.Sprintf("<b>%d new items (continued)</b>\n", n)
	}
	return fmt.Sprintf("<b>%d new items</b>\n", n)
}

func digestLine(i int, item *feedpb.GetFeedsResponse_Feed, mixed, withLink bool) string {
	title := itemTitle(item, 120)
	var line string
	if withLink {
		if link := firstHref(item); link != "" {
			line = fmt.Sprintf("%d. %s", i+1, formatAnchor(link, title))
		}
	}
	if line == "" {
		line = fmt.Sprintf("%d. %s", i+1, html.EscapeString(title))
	}
	if item.GetPublishedAt() != nil {
		date := item.GetPublishedAt().AsTime().UTC().Format("02 Jan")
		line += fmt.Sprintf(" <i>(%s)</i>", date)
	}
	if mixed {
		if src := cleanText(item.GetSourceTitle(), 40); src != "" {
			line += fmt.Sprintf(" <i>[%s]</i>", html.EscapeString(src))
		}
	}
	return line
}

// FormatDigestPages packs complete digest lines into messages that each stay
// under maxMessageLen. Numbering is global across pages.
func FormatDigestPages(items []*feedpb.GetFeedsResponse_Feed) []string {
	if len(items) == 0 {
		return nil
	}
	mixed := digestMixed(items)
	n := len(items)

	fits := func(page, line string) bool {
		return utf8.RuneCountInString(page)+utf8.RuneCountInString(line)+1 <= maxMessageLen
	}

	var pages []string
	var b strings.Builder
	continued := false
	flush := func() {
		if b.Len() == 0 {
			return
		}
		pages = append(pages, b.String())
		b.Reset()
		continued = true
	}

	b.WriteString(digestHeader(n, false))
	for i, item := range items {
		line := digestLine(i, item, mixed, true)
		if !fits(b.String(), line) {
			if utf8.RuneCountInString(b.String()) > utf8.RuneCountInString(digestHeader(n, continued)) {
				flush()
				b.WriteString(digestHeader(n, true))
			}
			if !fits(b.String(), line) {
				line = digestLine(i, item, mixed, false)
			}
		}
		fmt.Fprintf(&b, "%s\n", line)
	}
	flush()
	return pages
}

// htmlToPlain strips Telegram HTML tags so a message can be retried without
// parse mode after Telegram rejects the markup.
func htmlToPlain(s string) string {
	s = tagRe.ReplaceAllString(s, "")
	return html.UnescapeString(s)
}
