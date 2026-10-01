package handlers

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	subpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/subscription"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/grpcclient"
	tele "gopkg.in/telebot.v4"
)

const (
	previewCount = 3
	minInterval  = 5 * time.Minute
	maxInterval  = 24 * time.Hour
)

type FeedHandler struct {
	API *grpcclient.Client
}

func telegramDest(chatID int64) (platform, externalID string) {
	return "telegram", strconv.FormatInt(chatID, 10)
}

func esc(s string) string { return html.EscapeString(s) }

func adminGate(c tele.Context) bool {
	if IsAdmin(c) {
		return false
	}
	if c.Callback() != nil {
		c.Respond(&tele.CallbackResponse{Text: "Admins only"})
	} else if c.Chat() != nil {
		c.Send("Only group admins can manage feeds here.")
	}
	return true
}

func (h *FeedHandler) AddFeed(c tele.Context) error {
	if adminGate(c) {
		return nil
	}

	raw := strings.TrimSpace(strings.Join(c.Args(), " "))
	if raw == "" {
		return c.Send("Usage: /addfeed <url>\nExample: /addfeed https://example.com")
	}

	target, err := NormalizeURL(raw)
	if err != nil {
		return c.Send(BadURLHelp(err))
	}

	platform, ext := telegramDest(c.Chat().ID)
	res, err := h.API.Subscribe(context.Background(), platform, ext, target)
	if err != nil {
		log.Printf("Subscribe(%s) failed: %v", target, err)
		return c.Send("Could not add that URL. " + grpcclient.FriendlyError(err))
	}
	if !res.GetCreated() {
		return c.Send("You're already subscribed to this feed.\nManage it with /listfeed.")
	}

	feedURL := res.GetSubscription().GetUrl()
	top, ferr := h.fetchLatest(feedURL)

	confirm := "Added successfully."
	switch {
	case ferr != nil:
		log.Printf("latest-items preview failed for %s: %v", target, ferr)
		confirm += "\nCouldn't load the latest items just now - they will arrive with the next check."
	case len(top) > 0:
		confirm += fmt.Sprintf("\nShowing the latest %d below.", len(top))
	default:
		confirm += "\nNo items published yet."
	}
	if sendErr := c.Send(confirm); sendErr != nil {
		return sendErr
	}

	if len(top) == 0 {
		return nil
	}

	SendItems(c.Bot(), c.Chat().ID, top, sendGap)

	if ts := publishedAt(top[0]); !ts.IsZero() {
		if uerr := h.API.AckCursor(context.Background(), res.GetSubscription().GetId(), ts); uerr != nil {
			log.Println("cursor update failed:", uerr)
		}
	}
	return nil
}

func (h *FeedHandler) fetchLatest(url string) ([]*feedpb.GetFeedsResponse_Feed, error) {
	items, err := h.API.GetFeed(context.Background(), url, time.Time{})
	if err != nil {
		return nil, err
	}
	if len(items) > previewCount {
		items = items[:previewCount]
	}
	return items, nil
}

func (h *FeedHandler) ListFeed(c tele.Context) error {
	_, ext := telegramDest(c.Chat().ID)
	subs, err := h.API.ListSubscriptions(context.Background(), "telegram", ext)
	if err != nil {
		log.Println("list failed:", err)
		return c.Send("Could not load your feeds.")
	}
	if len(subs) == 0 {
		return c.Send("No feeds yet. Add one with /addfeed <url>\nExample: /addfeed https://example.com")
	}
	return renderSubscriptions(c, subs, false)
}

func renderSubscriptions(c tele.Context, subs []*subpb.Subscription, edit bool) error {
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, sub := range subRange(subs) {
		id := strconv.FormatUint(sub.GetId(), 10)

		toggleText, toggleUnique := "Disable", "fd_off"
		if !sub.GetEnabled() {
			toggleText, toggleUnique = "Enable", "fd_on"
		}
		rows = append(rows, menu.Row(
			menu.Data(shortURL(sub.GetUrl()), "fd_view", id),
			menu.Data(toggleText, toggleUnique, id),
			menu.Data("Remove", "fd_rm", id),
		))
	}
	menu.Inline(rows...)

	text := fmt.Sprintf("Your feeds (%d):", len(subs))
	if edit {
		return c.EditOrSend(text, &tele.SendOptions{ReplyMarkup: menu})
	}
	return c.Send(text, &tele.SendOptions{ReplyMarkup: menu})
}

func subRange(subs []*subpb.Subscription) []*subpb.Subscription {
	const maxButtons = 90
	if len(subs) > maxButtons {
		return subs[:maxButtons]
	}
	return subs
}

func shortURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		s := []rune(raw)
		if len(s) > 28 {
			return string(s[:28]) + "..."
		}
		return raw
	}
	s := u.Host + u.Path
	r := []rune(s)
	if len(r) > 28 {
		s = string(r[:25]) + "..."
	}
	return strings.TrimSuffix(s, "/")
}

func callbackID(c tele.Context) (uint64, bool) {
	args := c.Args()
	if len(args) == 0 {
		return 0, false
	}
	id, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func (h *FeedHandler) ownsSubscription(c tele.Context, id uint64) (*subpb.Subscription, bool) {
	sub, err := h.API.GetSubscription(context.Background(), id)
	_, ext := telegramDest(c.Chat().ID)
	if err != nil || sub.GetPlatform() != "telegram" || sub.GetExternalId() != ext {
		c.Respond(&tele.CallbackResponse{Text: "This button no longer works."})
		return nil, false
	}
	return sub, true
}

func respondAlert(c tele.Context, text string) {
	c.Respond(&tele.CallbackResponse{Text: text})
}

func (h *FeedHandler) OnViewBtn(c tele.Context) error {
	if adminGate(c) {
		return nil
	}
	id, ok := callbackID(c)
	if !ok {
		respondAlert(c, "Invalid button")
		return nil
	}
	sub, ok := h.ownsSubscription(c, id)
	if !ok {
		return nil
	}

	status := fmt.Sprintf("%s\nStatus: %s", esc(sub.GetUrl()), map[bool]string{true: "enabled", false: "disabled"}[sub.GetEnabled()])
	if sub.GetEnabled() {
		status += fmt.Sprintf("\nChecked every %d min", sub.GetIntervalSeconds()/60)
	}
	c.Respond()
	return c.EditOrSend(status)
}

func (h *FeedHandler) OnEnableBtn(c tele.Context) error  { return h.toggleBtn(c, true) }
func (h *FeedHandler) OnDisableBtn(c tele.Context) error { return h.toggleBtn(c, false) }

func (h *FeedHandler) toggleBtn(c tele.Context, enable bool) error {
	if adminGate(c) {
		return nil
	}
	id, ok := callbackID(c)
	if !ok {
		respondAlert(c, "Invalid button")
		return nil
	}
	if _, ok := h.ownsSubscription(c, id); !ok {
		return nil
	}
	if err := h.API.SetEnabled(context.Background(), id, enable); err != nil {
		log.Println("toggle failed:", err)
		respondAlert(c, "Failed, try again")
		return nil
	}
	c.Respond()

	_, ext := telegramDest(c.Chat().ID)
	subs, err := h.API.ListSubscriptions(context.Background(), "telegram", ext)
	if err != nil {
		return nil
	}
	return renderSubscriptions(c, subs, true)
}

func (h *FeedHandler) OnRemoveBtn(c tele.Context) error {
	if adminGate(c) {
		return nil
	}
	id, ok := callbackID(c)
	if !ok {
		respondAlert(c, "Invalid button")
		return nil
	}
	if _, ok := h.ownsSubscription(c, id); !ok {
		return nil
	}
	if err := h.API.Unsubscribe(context.Background(), id); err != nil {
		log.Println("remove failed:", err)
		respondAlert(c, "Failed, try again")
		return nil
	}
	respondAlert(c, "Removed")

	_, ext := telegramDest(c.Chat().ID)
	subs, lerr := h.API.ListSubscriptions(context.Background(), "telegram", ext)
	if lerr != nil || len(subs) == 0 {
		return c.EditOrSend("Feed removed.")
	}
	return renderSubscriptions(c, subs, true)
}

func (h *FeedHandler) urlCommand(c tele.Context, cmd string, fn func(platform, ext, url string) error, success string) error {
	if adminGate(c) {
		return nil
	}

	raw := strings.TrimSpace(strings.Join(c.Args(), " "))
	platform, ext := telegramDest(c.Chat().ID)
	if raw == "" {
		subs, lerr := h.API.ListSubscriptions(context.Background(), platform, ext)
		if lerr != nil || len(subs) == 0 {
			return c.Send("Usage: /" + cmd + " <url>\nYou have no subscriptions yet - add one with /addfeed <url>")
		}
		return renderSubscriptions(c, subs, false)
	}

	target, err := NormalizeURL(raw)
	if err != nil {
		return c.Send(BadURLHelp(err))
	}

	if err := fn(platform, ext, target); err != nil {
		if grpcclient.IsNotFound(err) {
			return c.Send("You are not subscribed to that URL.\nSee /listfeed for what you follow.")
		}
		log.Println(cmd, "failed:", err)
		return c.Send("Could not look that URL up. " + grpcclient.FriendlyError(err))
	}
	return c.Send(success + "\n" + esc(target))
}

func (h *FeedHandler) RemoveFeed(c tele.Context) error {
	return h.urlCommand(c, "removefeed",
		func(platform, ext, target string) error {
			return h.API.UnsubscribeByURL(context.Background(), platform, ext, target)
		},
		"Unsubscribed from:")
}

func (h *FeedHandler) EnableFeed(c tele.Context) error {
	return h.urlCommand(c, "enablefeed",
		func(platform, ext, target string) error {
			return h.API.SetEnabledByURL(context.Background(), platform, ext, target, true)
		},
		"Feed enabled:")
}

func (h *FeedHandler) DisableFeed(c tele.Context) error {
	return h.urlCommand(c, "disablefeed",
		func(platform, ext, target string) error {
			return h.API.SetEnabledByURL(context.Background(), platform, ext, target, false)
		},
		"Feed disabled:")
}

func (h *FeedHandler) SetInterval(c tele.Context) error {
	if adminGate(c) {
		return nil
	}
	args := c.Args()
	if len(args) == 0 {
		return c.Send("Usage: /interval <duration>\nExamples: 15m, 30m, 1h, 6h\nAllowed range: 5m - 24h")
	}
	d, err := time.ParseDuration(args[0])
	if err != nil {
		return c.Send("Could not parse that duration.\nUse forms like 15m, 30m, 1h or 1h30m.")
	}
	applied := d
	if applied < minInterval {
		applied = minInterval
	}
	if applied > maxInterval {
		applied = maxInterval
	}
	platform, ext := telegramDest(c.Chat().ID)
	if err := h.API.SetInterval(context.Background(), platform, ext, applied); err != nil {
		if grpcclient.IsNotFound(err) {
			return c.Send("You have no subscriptions yet - add one with /addfeed <url>")
		}
		log.Println("interval update failed:", err)
		return c.Send("Something went wrong, try again.")
	}
	note := ""
	if applied != d {
		note = fmt.Sprintf("\n<i>Clamped to the allowed range (%s - %s).</i>", minInterval, maxInterval)
	}
	return c.Send(fmt.Sprintf("All feeds in this chat now refresh every %s.%s", applied, note))
}
