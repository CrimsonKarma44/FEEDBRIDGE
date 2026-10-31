package handlers

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/CrimsonKarma44/FEEDBRIDGE/telegram-bot/grpcclient"
	tele "gopkg.in/telebot.v4"
)

// Deep links from the browser extension look like
// t.me/<bot>?start=add_<token> (or ?startgroup=add_<token>). The token maps
// to a feed URL in the API's Redis for 15 minutes and works once.
const linkPayloadPrefix = "add_"

var linkTokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{12}$`)

const linkGoneText = "That subscribe link has expired or was already used.\n" +
	"Open it again from the FeedBridge extension, or send /addfeed &lt;url&gt;."

// linkToken reads a /start payload. isLink is true for any add_ payload;
// token is empty when the rest isn't a well-formed token.
func linkToken(payload string) (token string, isLink bool) {
	p := strings.TrimSpace(payload)
	if !strings.HasPrefix(p, linkPayloadPrefix) {
		return "", false
	}
	tok := strings.TrimPrefix(p, linkPayloadPrefix)
	if !linkTokenRe.MatchString(tok) {
		return "", true
	}
	return tok, true
}

func linkConfirmText(feedURL string, group bool) string {
	text := fmt.Sprintf("<b>Subscribe this chat to this feed?</b>\n<code>%s</code>", esc(feedURL))
	if group {
		text += "\n\n<i>Only group admins can confirm.</i>"
	}
	return text
}

// StartWithLink handles /start add_<token> from the browser extension by
// asking the chat to confirm the subscription. Any other /start goes to next.
func (h *FeedHandler) StartWithLink(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		msg, chat := c.Message(), c.Chat()
		if msg == nil || chat == nil || chat.Type == tele.ChatChannel {
			return next(c)
		}
		tok, isLink := linkToken(msg.Payload)
		if !isLink {
			return next(c)
		}
		if tok == "" {
			return c.Send(linkGoneText)
		}

		feedURL, err := h.API.ResolveLinkToken(context.Background(), tok, false)
		if err != nil {
			if grpcclient.IsTokenGone(err) {
				return c.Send(linkGoneText)
			}
			log.Printf("ResolveLinkToken(peek) failed: %v", err)
			return c.Send("Couldn't open that subscribe link: " + grpcclient.FriendlyError(err) + ".")
		}

		menu := &tele.ReplyMarkup{}
		menu.Inline(menu.Row(
			menu.Data("Subscribe", "lk_add", tok),
			menu.Data("Cancel", "lk_no", tok),
		))
		return c.Send(linkConfirmText(feedURL, chat.Type != tele.ChatPrivate), &tele.SendOptions{
			ReplyMarkup:           menu,
			DisableWebPagePreview: true,
		})
	}
}

// OnLinkAddBtn confirms a deep-link subscription. The token is used up
// first, so a second tap (or a forwarded link) can't subscribe twice.
func (h *FeedHandler) OnLinkAddBtn(c tele.Context) error {
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
			c.Respond()
			return c.Edit(linkGoneText)
		}
		log.Printf("ResolveLinkToken(consume) failed: %v", err)
		respondAlert(c, "Couldn't reach the feed service, try again")
		return nil // keep the buttons so the user can retry
	}

	target, err := NormalizeURL(feedURL)
	if err != nil {
		c.Respond()
		return c.Edit(BadURLHelp(err))
	}

	c.Respond(&tele.CallbackResponse{Text: "Subscribing..."})
	if eerr := c.Edit(fmt.Sprintf("Subscribing to <code>%s</code>...", esc(target)),
		&tele.SendOptions{DisableWebPagePreview: true}); eerr != nil {
		log.Printf("edit confirm message failed: %v", eerr)
	}
	return h.subscribeURL(c, target)
}

// OnLinkCancelBtn drops the token and removes the buttons.
func (h *FeedHandler) OnLinkCancelBtn(c tele.Context) error {
	if adminGate(c) {
		return nil
	}
	if tok := firstArg(c); linkTokenRe.MatchString(tok) {
		if _, err := h.API.ResolveLinkToken(context.Background(), tok, true); err != nil && !grpcclient.IsTokenGone(err) {
			log.Printf("ResolveLinkToken(cancel) failed: %v", err)
		}
	}
	c.Respond()
	return c.Edit("Cancelled. Nothing was added.")
}

func firstArg(c tele.Context) string {
	if args := c.Args(); len(args) > 0 {
		return args[0]
	}
	return ""
}
