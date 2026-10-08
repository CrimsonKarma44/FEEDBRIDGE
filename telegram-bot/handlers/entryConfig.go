package handlers

import (
	"fmt"
	"html"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"
)

type EntryConfig struct{}

const privateStartTpl = `<b>Welcome to FEEDBRIDGE, %s!</b>

Get new posts from any website delivered straight into this chat - RSS, Atom and more, checked automatically.

<b>Quick start</b>
<code>/addfeed https://example.com</code>

<b>Commands</b>
/addfeed &lt;url&gt; - subscribe this chat
/listfeed - manage your subscriptions
/interval 30m - delivery frequency
/disablefeed &lt;url&gt; / /enablefeed &lt;url&gt;
/removefeed &lt;url&gt;

<i>New subscriptions show their latest 3 items right away.</i>`

const groupStartTpl = `<b>Hi %s! I deliver feed updates into this chat.</b>

Anyone can read along, admins manage:

/addfeed &lt;url&gt; - add a feed for everyone here
/listfeed - see what this group follows
/interval 30m - adjust frequency

<i>Add a feed to try it out.</i>`

func (con *EntryConfig) Start(c tele.Context) error {
	chat := c.Chat()
	if chat == nil {
		return nil
	}

	switch chat.Type {
	case tele.ChatGroup, tele.ChatSuperGroup:
		return c.Send(fmt.Sprintf(groupStartTpl, esc(chat.Title)))
	case tele.ChatChannel:
		return c.Send("Feed updates are delivered in groups and private chats. Add me there and use /start.")
	default:
		name := ""
		if sender := c.Sender(); sender != nil {
			name = html.EscapeString(strings.TrimSpace(sender.FirstName + " " + sender.LastName))
		}
		if name == "" {
			name = "friend"
		}
		return c.Send(fmt.Sprintf(privateStartTpl, name))
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
	c.Respond()
	return c.Edit("Feeds will be delivered right here in this chat. Add your first one with /addfeed <url>.")
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

func (con *EntryConfig) ConfigBtnCommunity(c tele.Context) error {
	c.Respond()
	menu := &tele.ReplyMarkup{}
	addBtn := menu.URL("Add me to a Community", "https://t.me/feed_bridge_bot?startgroup=true")
	menu.Inline(menu.Row(addBtn))
	return c.Edit(
		"Communities are supergroups in Telegram, so the same flow applies.\n"+
			"Add the bot, then use /addfeed inside the community.",
		&tele.SendOptions{ReplyMarkup: menu},
	)
}
