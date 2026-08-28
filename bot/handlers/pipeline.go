package handlers

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/grpcclient"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/model"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/store"
	tele "gopkg.in/telebot.v4"
)

// Sender is the slice of telebot both *tele.Bot and Context.Bot() satisfy.
type Sender interface {
	Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error)
}

func isParseEntitiesErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "can't parse entities")
}

func sendHTML(bot Sender, chatID int64, text string, opts *tele.SendOptions) error {
	if opts == nil {
		opts = &tele.SendOptions{}
	}
	htmlOpts := *opts
	htmlOpts.ParseMode = tele.ModeHTML
	_, err := bot.Send(tele.ChatID(chatID), text, &htmlOpts)
	if err == nil {
		return nil
	}
	if !isParseEntitiesErr(err) {
		return err
	}
	log.Printf("html parse failed for chat %d, retrying plain text: %v", chatID, err)
	plainOpts := *opts
	plainOpts.ParseMode = ""
	_, err = bot.Send(tele.ChatID(chatID), htmlToPlain(text), &plainOpts)
	return err
}

// SendItems delivers items as individual HTML messages with throttling.
// Link previews are left enabled so Telegram renders its native card for each
// item's article link. Returns how many items were successfully delivered;
// stops at first failure.
func SendItems(bot Sender, chatID int64, items []*feedpb.GetFeedsResponse_Feed, gap time.Duration) int {
	delivered := 0
	for _, item := range items {
		if err := sendHTML(bot, chatID, FormatItem(item), &tele.SendOptions{}); err != nil {
			log.Printf("deliver to chat %d failed: %v", chatID, err)
			break
		}
		delivered++
		time.Sleep(gap)
	}
	return delivered
}

// SendDigest delivers many new items collapsed into one or more messages,
// splitting on complete item lines so Telegram HTML tags stay balanced.
func SendDigest(bot Sender, chatID int64, items []*feedpb.GetFeedsResponse_Feed) error {
	pages := FormatDigestPages(items)
	for i, page := range pages {
		if err := sendHTML(bot, chatID, page, &tele.SendOptions{
			DisableWebPagePreview: true,
		}); err != nil {
			return err
		}
		if i < len(pages)-1 {
			time.Sleep(sendGap)
		}
	}
	return nil
}

// FetchTask builds the worker job for one subscription: fetch items newer than
// the cursor, deliver them, then advance the cursor. Undated items are skipped
// so they can never repeat forever.
func FetchTask(bot *tele.Bot, api *grpcclient.Client, st *store.Store, sub *store.Subscription) *model.Task {
	return model.NewTask(
		"fetch:"+strconv.FormatUint(uint64(sub.ID), 10),
		func(ctx context.Context) error {
			items, err := api.GetFeed(ctx, sub.URL, sub.LastSeenPublished)
			if err != nil {
				return err
			}

			var fresh []*feedpb.GetFeedsResponse_Feed
			var newest time.Time
			for _, item := range items {
				ts := publishedAt(item)
				if ts.IsZero() || !ts.After(sub.LastSeenPublished) {
					continue
				}
				fresh = append(fresh, item)
				if ts.After(newest) {
					newest = ts
				}
			}
			if len(fresh) == 0 {
				return nil
			}

			delivered := 0
			if len(fresh) > digestThreshold {
				if err := SendDigest(bot, sub.ChatID, fresh); err == nil {
					delivered = len(fresh)
				} else {
					log.Printf("digest to chat %d failed: %v", sub.ChatID, err)
				}
			} else {
				delivered = SendItems(bot, sub.ChatID, fresh, sendGap)
			}

			if delivered == 0 {
				log.Printf("delivery to chat %d failed entirely, keeping cursor", sub.ChatID)
				return nil
			}

			cursor := newest
			if delivered < len(fresh) && publishedAt(fresh[delivered-1]).After(time.Time{}) {
				cursor = publishedAt(fresh[delivered-1])
			}
			if err := st.UpdateCursor(sub.ID, cursor); err != nil {
				return err
			}
			sub.LastSeenPublished = cursor
			return nil
		},
	)
}

func publishedAt(item *feedpb.GetFeedsResponse_Feed) time.Time {
	if item.GetPublishedAt() == nil {
		return time.Time{}
	}
	return item.GetPublishedAt().AsTime()
}
