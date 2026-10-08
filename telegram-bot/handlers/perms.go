package handlers

import (
	"log"

	tele "gopkg.in/telebot.v4"
)

// IsAdmin allows everyone in private chats; in groups only admins may manage
// subscriptions. Fails closed with a notice when the admin lookup errors.
func IsAdmin(c tele.Context) bool {
	chat := c.Chat()
	if chat == nil {
		return false
	}
	if chat.Type == tele.ChatPrivate {
		return true
	}

	sender := c.Sender()
	if sender == nil {
		return false
	}

	admins, err := c.Bot().AdminsOf(chat)
	if err != nil {
		log.Printf("admin lookup failed for chat %d: %v", chat.ID, err)
		return false
	}
	for _, member := range admins {
		if member.User != nil && member.User.ID == sender.ID {
			return true
		}
	}
	return false
}
