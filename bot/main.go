package main

import (
	"log"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/handlers"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/model"
	tele "gopkg.in/telebot.v4"
)

func main() {
	env, err := model.UpdateEnv()
	if err != nil {
		log.Fatal(err)
		// return
	}

	pref := tele.Settings{
		Token:     env.Token,
		Poller:    &tele.LongPoller{Timeout: 10 * time.Second},
		ParseMode: tele.ModeHTML,
		// Verbose:   true,
		// Offline: false,
		OnError: func(err error, c tele.Context) {
			if c != nil {
				log.Printf("Error in update %d: %v", c.Update().ID, err)
			} else {
				log.Printf("Error: %v", err)
			}
		},
	}

	worker := model.NewWorkerPool(10)
	worker.Start()
	defer worker.Stop()
	scheduler := model.NewTaskScheduler(worker.TaskQueue)
	scheduler.Start()
	defer scheduler.Stop()

	task := model.NewTask("task1", time.Hour * 2, nil)
	scheduler.AddTask(task)
	

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	// b.Handle(tele.OnText, func(ctx tele.Context) error {
	// 	return ctx.Send("this is your message: " + ctx.Message().Text)
	// })

	// b.Handle(tele.OnMedia, func(ctx tele.Context) error {
	// 	photo := &telebot.Photo{
	// 		File:    ctx.Message().Photo.File,
	// 		Caption: "Check out this photo!\n" + ctx.Message().Caption,
	// 	}
	// 	return ctx.Send(photo)
	// })

	envConf := &handlers.EntryConfig{}
	bot.Handle("/start", envConf.Start)
	bot.Handle("/configure", envConf.Configure)
	bot.Handle("\fbtn_chat", envConf.ConfigBtnChat)
	bot.Handle("\fbtn_group", envConf.ConfigBtnGroup)

	feedHandler := &handlers.FeedHandler{}
	bot.Handle("/addfeed", feedHandler.AddFeed)
	bot.Handle("/listfeed", feedHandler.ListFeed)
	bot.Handle("/removefeed", feedHandler.RemoveFeed)
	bot.Handle("/disablefeed", feedHandler.DisableFeed)
	bot.Handle("\fbtn_disable_single_feed", feedHandler.DisableSingleFeed)
	bot.Handle("\fbtn_disable_all_feed", feedHandler.DisableAllFeeds)
	bot.Handle("/enablefeed", feedHandler.EnableFeed)
	bot.Handle("\fbtn_enable_single_feed", feedHandler.EnableSingleFeed)
	bot.Handle("\fbtn_enable_all_feed", feedHandler.EnableAllFeeds)

	log.Println("Bot started")
	bot.Start()
}
