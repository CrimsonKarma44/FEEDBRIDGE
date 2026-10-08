HOST ?= USER@your-vm-ip
REMOTE_DIR := /opt/feedbridge
LDFLAGS := -s -w

.PHONY: update run build-linux deploy clean proto

proto :
	PATH="$(PATH):$(shell go env GOPATH)/bin" protoc \
		--go_out=protoAPI --go-grpc_out=protoAPI \
		--proto_path=proto \
		--proto_path=/usr/include \
		proto/subscription.proto proto/feed.proto proto/setUrl.proto
	echo "Generated protoAPI"

update :
	"Updating dependencies"
	go get -u

run : feedbridge
	./feedbridge

feedbridge : main.go
	go build -o feedbridge main.go
	echo "Compiled feedbridge"

# Fully static binary for the GCE e2-micro (linux/amd64).
build-linux :
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o feedbridge-linux-amd64 main.go
	echo "Compiled static feedbridge-linux-amd64"

deploy : build-linux
	scp feedbridge-linux-amd64 $(HOST):/tmp/
	ssh $(HOST) 'sudo install -m 0755 -o feedbridge -g feedbridge /tmp/feedbridge-linux-amd64 $(REMOTE_DIR)/feedbridge && sudo systemctl restart feedbridge-api'
	echo "Deployed and restarted feedbridge-api"

clean :
	rm -f feedbridge feedbridge-linux-amd64
