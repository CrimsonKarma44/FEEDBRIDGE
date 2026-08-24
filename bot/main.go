package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/grpcclient"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/handlers"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/model"
	"github.com/CrimsonKarma44/FEEDBRIDGE/bot/store"
	tele "gopkg.in/telebot.v4"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	env, err := model.UpdateEnv()
	if err != nil {
		log.Fatal(err)
	}
	if env.Token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := gorm.Open(postgres.Open(env.Database.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatal("database connection failed: ", err)
	}
	st, err := store.New(db)
	if err != nil {
		log.Fatal("store migration failed: ", err)
	}

	api, err := grpcclient.New(env.APIAddr)
	if err != nil {
		log.Fatal("feed service client failed: ", err)
	}

	pref := tele.Settings{
		Token:     env.Token,
		Poller:    &tele.LongPoller{Timeout: 10 * time.Second},
		ParseMode: tele.ModeHTML,
		OnError: func(err error, c tele.Context) {
			if c != nil {
				log.Printf("Error in update %d: %v", c.Update().ID, err)
			} else {
				log.Printf("Error: %v", err)
			}
		},
	}

	bot, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
	}

	wp := model.NewWorkerPool(model.WorkerCount())
	wp.Start(ctx)

	scheduler := model.NewTaskScheduler(wp.TaskQueue, st, func(sub *store.Subscription) *model.Task {
		return handlers.FetchTask(bot, api, st, sub)
	})
	scheduler.Start(ctx)

	envConf := &handlers.EntryConfig{}
	feedHandler := &handlers.FeedHandler{Store: st, API: api}

	bot.Handle("/start", envConf.Start)
	bot.Handle("/configure", envConf.Configure)
	bot.Handle("\fbtn_chat", envConf.ConfigBtnChat)
	bot.Handle("\fbtn_group", envConf.ConfigBtnGroup)
	bot.Handle("\fbtn_community", envConf.ConfigBtnCommunity)

	bot.Handle("/addfeed", feedHandler.AddFeed)
	bot.Handle("/listfeed", feedHandler.ListFeed)
	bot.Handle("/removefeed", feedHandler.RemoveFeed)
	bot.Handle("/disablefeed", feedHandler.DisableFeed)
	bot.Handle("/enablefeed", feedHandler.EnableFeed)
	bot.Handle("/interval", feedHandler.SetInterval)

	bot.Handle("\ffd_view", feedHandler.OnViewBtn)
	bot.Handle("\ffd_on", feedHandler.OnEnableBtn)
	bot.Handle("\ffd_off", feedHandler.OnDisableBtn)
	bot.Handle("\ffd_rm", feedHandler.OnRemoveBtn)

	go func() {
		log.Println("Bot started")
		bot.Start()
	}()

	<-ctx.Done()
	log.Println("shutdown signal received")

	bot.Stop()
	scheduler.Stop()
	wp.Stop()

	if err := api.Close(); err != nil {
		log.Printf("api client close: %v", err)
	}
	if sqlDB, derr := db.DB(); derr == nil {
		if cerr := sqlDB.Close(); cerr != nil {
			log.Printf("db close: %v", cerr)
		}
	}
	log.Println("shutdown complete")
}
