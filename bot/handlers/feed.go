package handlers

import (
	tele "gopkg.in/telebot.v4"
)

type FeedHandler struct{}

func (h *FeedHandler) AddFeed(c tele.Context) error {
	return c.Send("Feed added")
}

func (h *FeedHandler) ListFeed(c tele.Context) error {
	return c.Send("Feeds listed")
}

func (h *FeedHandler) DisableFeed(c tele.Context) error {
	menu := &tele.ReplyMarkup{}

	single_btn := tele.Btn{Text: "Disable Single Feed", Unique: "btn_disable_single_feed"}
	all_btn := tele.Btn{Text: "Disable All Feeds", Unique: "btn_disable_all_feed"}

	menu.Inline(
		menu.Row(single_btn, all_btn),
	)

	return c.Send("Feed disabled", &tele.SendOptions{
		ReplyMarkup: menu,
	})
}

func (h *FeedHandler) DisableSingleFeed(c tele.Context) error {
	return c.Edit("Single feed disabled")
}

func (h *FeedHandler) DisableAllFeeds(c tele.Context) error {
	return c.Edit("All feeds disabled")
}

func (h *FeedHandler) EnableFeed(c tele.Context) error {
	menu := &tele.ReplyMarkup{}

	single_btn := tele.Btn{Text: "Enable Single Feed", Unique: "btn_enable_single_feed"}
	all_btn := tele.Btn{Text: "Enable All Feeds", Unique: "btn_enable_all_feed"}

	menu.Inline(
		menu.Row(single_btn, all_btn),
	)

	return c.Send("Feed enabled", &tele.SendOptions{
		ReplyMarkup: menu,
	})
}

func (h *FeedHandler) EnableSingleFeed(c tele.Context) error {
	return c.Edit("Single feed enabled")
}

func (h *FeedHandler) EnableAllFeeds(c tele.Context) error {
	return c.Edit("All feeds enabled")
}
func (h *FeedHandler) RemoveFeed(c tele.Context) error {
	return c.Send("Feed removed")
}
