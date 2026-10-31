module github.com/CrimsonKarma44/FEEDBRIDGE/telegram-bot

go 1.26.5

require (
	github.com/CrimsonKarma44/FEEDBRIDGE/backend v0.0.0
	github.com/joho/godotenv v1.5.1
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/telebot.v4 v4.0.0-beta.10
)

require (
	github.com/stretchr/testify v1.11.1 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261005182115-fad411399dd8 // indirect
)

replace github.com/CrimsonKarma44/FEEDBRIDGE/backend => ../backend

replace github.com/CrimsonKarma44/rss_detector => /home/deus/Documents/code/project/rss_detector
