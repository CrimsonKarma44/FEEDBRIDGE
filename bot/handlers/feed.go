package handlers

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/grpcclient"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/store"
	tele "gopkg.in/telebot.v4"
	"gorm.io/gorm"
)

const previewCount = 3

type FeedHandler struct {
	Store *store.Store
	API   *grpcclient.Client
}

// esc makes user-derived text safe under Telegram HTML parse mode.
func esc(s string) string { return html.EscapeString(s) }

// adminGate returns true when the action must be blocked, responding to the
// user appropriately. Never returns an error so OnError stays quiet.
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

	count, err := h.API.SetUrl(context.Background(), target)
	if err != nil {
		log.Printf("SetUrl(%s) failed: %v", target, err)
		return c.Send("Could not add that URL. " + grpcclient.FriendlyError(err))
	}

	sub, err := h.Store.Add(c.Chat().ID, target, store.DefaultInterval)
	if err != nil {
		log.Println("subscribe failed:", err)
		return c.Send(fmt.Sprintf("Detected %d feed(s), but saving the subscription failed.", count))
	}

	top, ferr := h.fetchLatest(target)

	confirm := fmt.Sprintf("Subscribed to %s\n%d feed(s) detected.\nChecked every %s.", esc(target), count, store.DefaultInterval)
	if target != raw {
		confirm += fmt.Sprintf("\n<i>Normalized to %s</i>", esc(target))
	}
	switch {
	case ferr != nil:
		log.Printf("latest-items preview failed for %s: %v", target, ferr)
		confirm += "\nCouldn't load the latest items just now - they will arrive with the next check."
	case len(top) > 0:
		confirm += fmt.Sprintf("\nShowing the latest %d item(s) below.", len(top))
	default:
		confirm += "\nNo items published yet."
	}
	if sendErr := c.Send(confirm); sendErr != nil {
		return sendErr
	}

	if len(top) == 0 {
		return nil
	}

	SendItems(c.Bot(), sub.ChatID, top, sendGap)

	if ts := publishedAt(top[0]); !ts.IsZero() {
		if uerr := h.Store.UpdateCursor(sub.ID, ts); uerr != nil {
			log.Println("cursor update failed:", uerr)
		} else {
			sub.LastSeenPublished = ts
		}
	}
	return nil
}

// fetchLatest returns the most recent items for a URL (newest first).
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
	subs, err := h.Store.ListByChat(c.Chat().ID)
	if err != nil {
		log.Println("list failed:", err)
		return c.Send("Could not load your feeds.")
	}
	if len(subs) == 0 {
		return c.Send("No feeds yet. Add one with /addfeed <url>\nExample: /addfeed https://example.com")
	}
	return renderSubscriptions(c, subs, false)
}

func renderSubscriptions(c tele.Context, subs []store.Subscription, edit bool) error {
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, sub := range subRange(subs) {
		id := strconv.FormatUint(uint64(sub.ID), 10)

		toggleText, toggleUnique := "Disable", "fd_off"
		if !sub.Enabled {
			toggleText, toggleUnique = "Enable", "fd_on"
		}
		rows = append(rows, menu.Row(
			menu.Data(shortURL(sub.URL), "fd_view", id),
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

func subRange(subs []store.Subscription) []store.Subscription {
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

func callbackID(c tele.Context) (uint, bool) {
	args := c.Args()
	if len(args) == 0 {
		return 0, false
	}
	id, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return uint(id), true
}

// ownsSubscription guards callbacks: the subscription must belong to the chat
// the button was pressed in. Always acknowledges the callback.
func (h *FeedHandler) ownsSubscription(c tele.Context, id uint) (*store.Subscription, bool) {
	sub, err := h.Store.Get(id)
	if err != nil || sub.ChatID != c.Chat().ID {
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

	status := fmt.Sprintf("%s\nStatus: %s", esc(sub.URL), map[bool]string{true: "enabled", false: "disabled"}[sub.Enabled])
	if sub.Enabled {
		status += fmt.Sprintf("\nChecked every %d min", sub.IntervalSeconds/60)
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
	if err := h.Store.SetEnabled(id, enable); err != nil {
		log.Println("toggle failed:", err)
		respondAlert(c, "Failed, try again")
		return nil
	}
	c.Respond()

	subs, err := h.Store.ListByChat(c.Chat().ID)
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
	sub, ok := h.ownsSubscription(c, id)
	if !ok {
		return nil
	}
	if err := h.Store.Remove(sub.ID); err != nil {
		log.Println("remove failed:", err)
		respondAlert(c, "Failed, try again")
		return nil
	}
	respondAlert(c, "Removed")

	subs, lerr := h.Store.ListByChat(c.Chat().ID)
	if lerr != nil || len(subs) == 0 {
		return c.EditOrSend("Feed removed.")
	}
	return renderSubscriptions(c, subs, true)
}

// urlCommand backs /removefeed, /enablefeed and /disablefeed. With no argument
// it shows the interactive list instead of a bare usage line.
func (h *FeedHandler) urlCommand(c tele.Context, cmd string, fn func(chatID int64, target string) error, success string) error {
	if adminGate(c) {
		return nil
	}

	raw := strings.TrimSpace(strings.Join(c.Args(), " "))
	if raw == "" {
		subs, lerr := h.Store.ListByChat(c.Chat().ID)
		if lerr != nil || len(subs) == 0 {
			return c.Send("Usage: /" + cmd + " <url>\nYou have no subscriptions yet - add one with /addfeed <url>")
		}
		return renderSubscriptions(c, subs, false)
	}

	target, err := NormalizeURL(raw)
	if err != nil {
		return c.Send(BadURLHelp(err))
	}

	if err := fn(c.Chat().ID, target); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Send("You are not subscribed to that URL.\nSee /listfeed for what you follow.")
		}
		log.Println(cmd, "failed:", err)
		return c.Send("Something went wrong, try again.")
	}
	return c.Send(success + "\n" + esc(target))
}

func (h *FeedHandler) RemoveFeed(c tele.Context) error {
	return h.urlCommand(c, "removefeed",
		func(chatID int64, target string) error { return h.Store.RemoveByURL(chatID, target) },
		"Unsubscribed from:")
}

func (h *FeedHandler) EnableFeed(c tele.Context) error {
	return h.urlCommand(c, "enablefeed",
		func(chatID int64, target string) error { return h.Store.SetEnabledByURL(chatID, target, true) },
		"Feed enabled:")
}

func (h *FeedHandler) DisableFeed(c tele.Context) error {
	return h.urlCommand(c, "disablefeed",
		func(chatID int64, target string) error { return h.Store.SetEnabledByURL(chatID, target, false) },
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
	if applied < store.MinInterval {
		applied = store.MinInterval
	}
	if applied > store.MaxInterval {
		applied = store.MaxInterval
	}
	if err := h.Store.SetChatInterval(c.Chat().ID, applied); err != nil {
		log.Println("interval update failed:", err)
		return c.Send("Something went wrong, try again.")
	}
	note := ""
	if applied != d {
		note = fmt.Sprintf("\n<i>Clamped to the allowed range (%s - %s).</i>", store.MinInterval, store.MaxInterval)
	}
	return c.Send(fmt.Sprintf("All feeds in this chat now refresh every %s.%s", applied, note))
}
