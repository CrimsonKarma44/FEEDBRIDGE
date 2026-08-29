#!/usr/bin/env bash
# Cross-compile the bot (linux/amd64) and redeploy to the GCE VM.
# Usage: HOST=you@EXTERNAL_IP ./deploy-bot.sh
set -euo pipefail

HOST="${HOST:?usage: HOST=you@EXTERNAL_IP $0}"
REMOTE_DIR=/opt/feedbridge

cd "$(dirname "$0")/../bot"

echo "==> building (linux/amd64, static)"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" \
    -o feedbridge-bot-linux-amd64 main.go

echo "==> shipping"
scp feedbridge-bot-linux-amd64 "${HOST}:/tmp/"

echo "==> installing + restarting"
ssh "${HOST}" "sudo install -m 0755 -o feedbridge -g feedbridge /tmp/feedbridge-bot-linux-amd64 ${REMOTE_DIR}/feedbridge-bot && sudo systemctl restart feedbridge-bot"

echo "==> done. journalctl -u feedbridge-bot -f"
rm feedbridge-bot-linux-amd64
