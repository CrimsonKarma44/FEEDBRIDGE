HOST ?= USER@your-vm-ip
REMOTE_DIR := /opt/feedbridge
LDFLAGS := -s -w

.PHONY: run build-linux deploy clean

run : feedbridge-bot
	./feedbridge-bot

feedbridge-bot : main.go
	go build -o feedbridge-bot main.go
	echo "Compiled feedbridge-bot"

# Fully static binary for the GCE e2-micro (linux/amd64).
build-linux :
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o feedbridge-bot-linux-amd64 main.go
	echo "Compiled static feedbridge-bot-linux-amd64"

deploy : build-linux
	scp feedbridge-bot-linux-amd64 $(HOST):/tmp/
	ssh $(HOST) 'sudo install -m 0755 -o feedbridge -g feedbridge /tmp/feedbridge-bot-linux-amd64 $(REMOTE_DIR)/feedbridge-bot && sudo systemctl restart feedbridge-bot'
	echo "Deployed and restarted feedbridge-bot"

clean :
	rm -f feedbridge-bot feedbridge-bot-linux-amd64
