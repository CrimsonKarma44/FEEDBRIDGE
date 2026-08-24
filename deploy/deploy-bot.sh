#!/usr/bin/env bash
# Cross-compile the bot and redeploy to the VM.
# Usage: HOST=ubuntu@your-vm-ip ./deploy-bot.sh
set -euo pipefail

HOST="${HOST:?usage: HOST=ubuntu@your-vm-ip $0}"
REMOTE_DIR=/opt/feedbridge

cd "$(dirname "$0")/../bot"

echo "==> building (linux/arm64, static)"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" \
    -o feedbridge-bot-linux-arm64 main.go

echo "==> shipping"
scp feedbridge-bot-linux-arm64 "${HOST}:/tmp/"

echo "==> installing + restarting"
ssh "${HOST}" "sudo install -m 0755 -o feedbridge -g feedbridge /tmp/feedbridge-bot-linux-arm64 ${REMOTE_DIR}/feedbridge-bot && sudo systemctl restart feedbridge-bot"

echo "==> done. journalctl -u feedbridge-bot -f"
rm feedbridge-bot-linux-arm64
