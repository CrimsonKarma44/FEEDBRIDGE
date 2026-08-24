module github.com/CrimsonKarma44/FEEDBRIDGE/bot

go 1.26.5

require (
	github.com/CrimsonKarma44/FEEDBRIDGE/API v0.0.0
	github.com/joho/godotenv v1.5.1
	google.golang.org/grpc v1.83.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/telebot.v4 v4.0.0-beta.10
	gorm.io/driver/postgres v1.6.0
	gorm.io/gorm v1.31.2
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260818201246-1b0934165a6f // indirect
)

replace github.com/CrimsonKarma44/FEEDBRIDGE/API => ../API

replace github.com/CrimsonKarma44/rss_detector => /home/deus/Documents/code/project/rss_detector
