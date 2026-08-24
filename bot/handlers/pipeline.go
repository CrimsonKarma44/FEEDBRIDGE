package handlers

import (
	"context"
	"log"
	"strconv"
	"time"

	feedpb "github.com/CrimsonKarma44/FEEDBRIDGE/API/protoAPI/Feed"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/grpcclient"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/model"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/store"
	tele "gopkg.in/telebot.v4"
)

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

			send := func(text string) error {
				_, serr := bot.Send(tele.ChatID(sub.ChatID), text, &tele.SendOptions{
					ParseMode:             tele.ModeHTML,
					DisableWebPagePreview: true,
				})
				time.Sleep(sendGap)
				return serr
			}

			delivered := 0
			if len(fresh) > digestThreshold {
				if err := send(FormatDigest(fresh)); err == nil {
					delivered = len(fresh)
				}
			} else {
				for _, item := range fresh {
					if err := send(FormatItem(item)); err != nil {
						break
					}
					delivered++
				}
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
