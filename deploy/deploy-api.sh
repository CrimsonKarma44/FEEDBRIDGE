#!/usr/bin/env bash
# Cross-compile the API and redeploy to the VM.
# Usage: HOST=ubuntu@your-vm-ip ./deploy-api.sh
set -euo pipefail

HOST="${HOST:?usage: HOST=ubuntu@your-vm-ip $0}"
REMOTE_DIR=/opt/feedbridge

cd "$(dirname "$0")/../API"

echo "==> building (linux/arm64, static)"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" \
    -o feedbridge-linux-arm64 main.go

echo "==> shipping"
scp feedbridge-linux-arm64 "${HOST}:/tmp/"

echo "==> installing + restarting"
ssh "${HOST}" "sudo install -m 0755 -o feedbridge -g feedbridge /tmp/feedbridge-linux-arm64 ${REMOTE_DIR}/feedbridge && sudo systemctl restart feedbridge-api"

echo "==> done. journalctl -u feedbridge-api -f"
rm feedbridge-linux-arm64
