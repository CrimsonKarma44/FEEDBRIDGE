package handlers

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

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

// FormatItem renders one feed item as a Telegram HTML message.
func FormatItem(item *feedpb.GetFeedsResponse_Feed) string {
	var b strings.Builder

	title := cleanText(item.GetTitle(), 200)
	link := firstLink(item)

	b.WriteString("<b>")
	if link != "" {
		b.WriteString(fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(link), html.EscapeString(title)))
	} else {
		b.WriteString(html.EscapeString(title))
	}
	b.WriteString("</b>\n")

	if desc := item.GetDescription(); desc != "" {
		fmt.Fprintf(&b, "%s\n", html.EscapeString(cleanText(desc, snippetLen)))
	}

	if item.GetPublishedAt() != nil {
		ts := item.GetPublishedAt().AsTime().UTC()
		fmt.Fprintf(&b, "\n<i>%s</i>", ts.Format("02 Jan 2006, 15:04 UTC"))
	}

	msg := b.String()
	if len(msg) > maxMessageLen {
		msg = msg[:maxMessageLen]
	}
	return msg
}

// FormatDigest renders many new items into one message.
func FormatDigest(items []*feedpb.GetFeedsResponse_Feed) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<b>%d new items</b>\n", len(items))
	for i, item := range items {
		title := cleanText(item.GetTitle(), 120)
		date := ""
		if item.GetPublishedAt() != nil {
			date = item.GetPublishedAt().AsTime().UTC().Format("02 Jan")
		}
		line := fmt.Sprintf("%d. %s", i+1, html.EscapeString(title))
		if date != "" {
			line += fmt.Sprintf(" <i>(%s)</i>", date)
		}
		if link := firstLink(item); link != "" {
			line = fmt.Sprintf("%d. <a href=\"%s\">%s</a>", i+1, html.EscapeString(link), html.EscapeString(title))
			if date != "" {
				line += fmt.Sprintf(" <i>(%s)</i>", date)
			}
		}
		fmt.Fprintf(&b, "%s\n", line)
	}

	msg := b.String()
	if len(msg) > maxMessageLen {
		msg = msg[:maxMessageLen]
	}
	return msg
}

func firstLink(item *feedpb.GetFeedsResponse_Feed) string {
	for _, l := range item.GetLinks() {
		if l != "" {
			return l
		}
	}
	return ""
}
