package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
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

// safe wraps handlers with panic recovery. Telebot runs each handler in a
// bare goroutine, so an unrecovered panic would kill the whole bot silently.
func safe(h tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic recovered in handler: %v\n%s", r, debug.Stack())
				if c.Callback() != nil {
					c.Respond(&tele.CallbackResponse{Text: "Something went wrong"})
					return
				}
				if c.Chat() != nil {
					_ = c.Send("Something went wrong handling that command.")
				}
			}
		}()
		return h(c)
	}
}

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

	handle := func(endpoint string, h tele.HandlerFunc) {
		bot.Handle(endpoint, safe(h))
	}

	handle("/start", envConf.Start)
	handle("/configure", envConf.Configure)
	handle("\fbtn_chat", envConf.ConfigBtnChat)
	handle("\fbtn_group", envConf.ConfigBtnGroup)
	handle("\fbtn_community", envConf.ConfigBtnCommunity)

	handle("/addfeed", feedHandler.AddFeed)
	handle("/listfeed", feedHandler.ListFeed)
	handle("/removefeed", feedHandler.RemoveFeed)
	handle("/disablefeed", feedHandler.DisableFeed)
	handle("/enablefeed", feedHandler.EnableFeed)
	handle("/interval", feedHandler.SetInterval)

	handle("\ffd_view", feedHandler.OnViewBtn)
	handle("\ffd_on", feedHandler.OnEnableBtn)
	handle("\ffd_off", feedHandler.OnDisableBtn)
	handle("\ffd_rm", feedHandler.OnRemoveBtn)

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
