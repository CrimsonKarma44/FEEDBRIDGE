package handlers

import (
	"time"

	tele "gopkg.in/telebot.v4"
)

type EntryConfig struct{}

func (con *EntryConfig) Start(c tele.Context) error {
	switch c.Chat().Type {
	case tele.ChatGroup:
		return c.Send("Hello, " + c.Chat().Title + "!")
	case tele.ChatSuperGroup:
		return c.Send("Hello from supergroup chat")
	default:
		fullname := c.Sender().FirstName
		if c.Sender().LastName != "" {
			fullname += " " + c.Sender().LastName
		}
		return c.Send("Hello, " + fullname + "!")
	}
}

func (con *EntryConfig) Configure(c tele.Context) error {
	err := c.Send("Configure Bot Environment")
	if err != nil {
		return err
	}

	_ = c.DeleteAfter(time.Second * 2)

	menu := &tele.ReplyMarkup{}

	btnInChat := menu.Data("Current chat", "btn_chat")
	btnInGroup := menu.Data("Group chat", "btn_group")
	btnInCommunity := menu.Data("Community chat", "btn_community")

	menu.Inline(
		menu.Row(btnInChat, btnInGroup),
		menu.Row(btnInCommunity),
	)

	return c.Send("How do you want to use the bot?", &tele.SendOptions{
		ReplyMarkup: menu,
	})
}

func (con *EntryConfig) ConfigBtnChat(c tele.Context) error {
	// Acknowledge the tap so the loading spinner stops on the user's client
	c.Respond(&tele.CallbackResponse{Text: "Thanks for liking!"})

	// Optionally edit or update the existing message
	return c.Edit("You liked this prompt!")
}

func (con *EntryConfig) ConfigBtnGroup(c tele.Context) error {
	c.Respond()
	menu := &tele.ReplyMarkup{}
	addToGroupBtn := menu.URL("Add me to a Group", "https://t.me/feed_bridge_bot?startgroup=true")
	menu.Inline(menu.Row(addToGroupBtn))
	return c.Edit("Click the button below to add me to your group or community.", &tele.SendOptions{
		ReplyMarkup: menu,
	})
}
