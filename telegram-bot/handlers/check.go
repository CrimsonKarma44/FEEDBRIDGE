package handlers

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"unicode/utf8"

	extpb "github.com/CrimsonKarma44/FEEDBRIDGE/backend/protoAPI/extension"
	"github.com/CrimsonKarma44/FEEDBRIDGE/telegram-bot/grpcclient"
	tele "gopkg.in/telebot.v4"
)

const (
	// maxCheckFeeds caps the list and its buttons; GitHub repos alone
	// report three feeds, most blogs one or two.
	maxCheckFeeds   = 6
	checkLabelRunes = 28
)

const checkUsage = "Usage: /check <url>\nExample: /check https://go.dev/blog\n\n" +
	"I'll look for RSS, Atom and JSON feeds there. In a private chat you can also just send me the link."

// Check handles "/check <url>": validate, look for feeds, offer to subscribe.
func (h *FeedHandler) Check(c tele.Context) error {
	raw := strings.TrimSpace(strings.Join(c.Args(), " "))
	if raw == "" {
		return c.Send(checkUsage)
	}
	return h.checkURL(c, raw)
}

// OnText treats a bare link sent in a private chat as /check <link>. Other
// text, and anything in groups (where links are usually just conversation),
// is ignored.
func (h *FeedHandler) OnText(c tele.Context) error {
	chat := c.Chat()
	if chat == nil || chat.Type != tele.ChatPrivate {
		return nil
	}
	text := c.Text()
	if !looksLikeURL(text) {
		return nil
	}
	return h.checkURL(c, text)
}

func (h *FeedHandler) checkURL(c tele.Context, raw string) error {
	target, err := NormalizeURL(raw)
	if err != nil {
		return c.Send("That doesn't look like a valid web address: " + err.Error() + ".\n" +
			"Example: /check https://example.com")
	}
	host := hostOf(target)
	noPreview := &tele.SendOptions{DisableWebPagePreview: true}

	// Detection can take several seconds; show progress, then edit it.
	status, serr := c.Bot().Send(c.Recipient(), fmt.Sprintf("Checking <b>%s</b> for feeds...", esc(host)), noPreview)
	reply := func(text string, opts *tele.SendOptions) error {
		if serr == nil {
			if _, eerr := c.Bot().Edit(status, text, opts); eerr == nil {
				return nil
			}
		}
		return c.Send(text, opts)
	}

	feeds, err := h.API.Detect(context.Background(), target)
	if err != nil {
		if grpcclient.IsNotFound(err) {
			return reply(noFeedsText(host), noPreview)
		}
		log.Printf("Detect(%s) failed: %v", target, err)
		return reply(fmt.Sprintf("Couldn't check <b>%s</b>: %s.", esc(host), grpcclient.FriendlyError(err)), noPreview)
	}
	if len(feeds) == 0 {
		return reply(noFeedsText(host), noPreview)
	}

	shown := feeds
	if len(shown) > maxCheckFeeds {
		shown = shown[:maxCheckFeeds]
	}

	// One single-use token per feed: the feed URL won't fit in Telegram's
	// 64-byte button data, the token does.
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i, f := range shown {
		tok, terr := h.API.CreateLinkToken(context.Background(), f.GetUrl())
		if terr != nil {
			log.Printf("CreateLinkToken(%s) failed: %v", f.GetUrl(), terr)
			continue
		}
		rows = append(rows, menu.Row(menu.Data(checkButtonLabel(i, f, len(shown)), "ck_add", tok)))
	}
	menu.Inline(rows...)

	text := checkResultText(host, shown, len(feeds), len(rows) > 0)
	opts := &tele.SendOptions{DisableWebPagePreview: true}
	if len(rows) > 0 {
		opts.ReplyMarkup = menu
	}
	return reply(text, opts)
}

// OnCheckAddBtn subscribes the chat to the feed behind a /check button.
func (h *FeedHandler) OnCheckAddBtn(c tele.Context) error {
	if adminGate(c) {
		return nil
	}
	tok := firstArg(c)
	if !linkTokenRe.MatchString(tok) {
		respondAlert(c, "Invalid button")
		return nil
	}
	feedURL, err := h.API.ResolveLinkToken(context.Background(), tok, true)
	if err != nil {
		if grpcclient.IsTokenGone(err) {
			respondAlert(c, "This button expired or was already used. Send /check again.")
			return nil
		}
		log.Printf("ResolveLinkToken(check) failed: %v", err)
		respondAlert(c, "Couldn't reach the feed service, try again")
		return nil
	}
	target, err := NormalizeURL(feedURL)
	if err != nil {
		respondAlert(c, "That feed address isn't valid anymore.")
		return nil
	}
	c.Respond(&tele.CallbackResponse{Text: "Subscribing..."})
	return h.subscribeURL(c, target)
}

// looksLikeURL decides whether a bare message is meant as a link: one
// token, not a command, with an http(s) scheme or a dotted host whose last
// label has letters ("example.com", not "3.5" or "e.g").
func looksLikeURL(text string) bool {
	t := strings.TrimSpace(text)
	if t == "" || strings.HasPrefix(t, "/") || strings.ContainsAny(t, " \t\r\n") {
		return false
	}
	lower := strings.ToLower(t)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return true
	}
	if strings.Contains(t, "://") || strings.Contains(t, "@") {
		return false
	}
	host := t
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	dot := strings.LastIndex(host, ".")
	if dot <= 0 || !hostRe.MatchString(host) {
		return false
	}
	tld := host[dot+1:]
	if len(tld) < 2 {
		return false
	}
	for _, r := range tld {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

func noFeedsText(host string) string {
	return fmt.Sprintf("No feeds found on <b>%s</b>.\n"+
		"I couldn't find an RSS, Atom or JSON feed there. Try the site's homepage or blog page.", esc(host))
}

func checkResultText(host string, shown []*extpb.DetectedFeed, total int, buttons bool) string {
	var b strings.Builder
	noun := "feeds"
	if total == 1 {
		noun = "feed"
	}
	fmt.Fprintf(&b, "<b>%d %s found on %s</b>\n", total, noun, esc(host))
	for i, f := range shown {
		title := strings.TrimSpace(f.GetTitle())
		if title == "" {
			title = hostOf(f.GetUrl())
		}
		fmt.Fprintf(&b, "\n%d. <b>%s</b> · %s\n<code>%s</code>\n", i+1, esc(title), feedTypeLabel(f.GetType()), esc(f.GetUrl()))
	}
	if total > len(shown) {
		fmt.Fprintf(&b, "\nShowing the first %d of %d.\n", len(shown), total)
	}
	if buttons {
		b.WriteString("\nTap a button to subscribe this chat. The buttons work for 15 minutes.")
	} else {
		b.WriteString("\nSubscribe with /addfeed &lt;feed url&gt;.")
	}
	return b.String()
}

func checkButtonLabel(i int, f *extpb.DetectedFeed, count int) string {
	if count == 1 {
		return "Subscribe"
	}
	title := strings.TrimSpace(f.GetTitle())
	if title == "" {
		title = shortURL(f.GetUrl())
	}
	return fmt.Sprintf("%d. Subscribe: %s", i+1, truncateRunes(title, checkLabelRunes))
}

func feedTypeLabel(t string) string {
	switch t {
	case "rss":
		return "RSS"
	case "atom":
		return "Atom"
	case "json":
		return "JSON Feed"
	default:
		return "Feed"
	}
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return raw
}
