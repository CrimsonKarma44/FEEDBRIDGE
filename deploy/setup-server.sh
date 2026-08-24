#!/usr/bin/env bash
# Bootstrap a fresh Ubuntu (arm64) VM for FEEDBRIDGE.
# Usage: sudo ./setup-server.sh
# Run ONCE on the server. Idempotent where practical.
set -euo pipefail

[[ $EUID -eq 0 ]] || { echo "must run as root (sudo)"; exit 1; }

echo "==> packages"
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get upgrade -y
apt-get install -y postgresql redis-server ufw openssl curl

echo "==> service user"
if ! id -u feedbridge &>/dev/null; then
    useradd --system --group --home /opt/feedbridge --shell /usr/sbin/nologin feedbridge
fi
install -d -m 0750 -o feedbridge -g feedbridge /opt/feedbridge
install -d -m 0700 -o root -g root /etc/feedbridge

echo "==> postgres"
read -rp "Postgres password for DB user 'deus': " DBPASS
sudo -u postgres psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='deus'" | grep -q 1 ||
    sudo -u postgres psql -c "CREATE USER deus WITH PASSWORD '${DBPASS}';"
sudo -u postgres psql -tAc "SELECT 1 FROM pg_database WHERE datname='feedbridge'" | grep -q 1 ||
    sudo -u postgres psql -c "CREATE DATABASE feedbridge OWNER deus;"
sudo -u postgres psql -c "ALTER USER deus WITH PASSWORD '${DBPASS}';"

echo "==> redis"
REDIS_PASS="$(openssl rand -hex 16)"
if ! grep -q '^requirepass' /etc/redis/redis.conf; then
    echo "requirepass ${REDIS_PASS}" >> /etc/redis/redis.conf
fi
# Loopback-only is the default; enforce anyway.
grep -q '^bind 127.0.0.1' /etc/redis/redis.conf || sed -i 's/^bind .*/bind 127.0.0.1/' /etc/redis/redis.conf
systemctl enable --now redis-server postgresql
systemctl restart redis-server

echo "==> firewall"
ufw default deny incoming
ufw default allow outgoing
ufw allow OpenSSH
yes | ufw enable

cat <<'EOF'

============================================================
 Server bootstrap complete.

 Next steps:
   1) Copy deploy/env/*.env.example to /etc/feedbridge/api.env
      and /etc/feedbridge/bot.env, fill in real values.
      REDIS_PASSWORD (if newly generated): ${REDIS_PASS shown above}
   2) Install units: cp deploy/*.service /etc/systemd/system/
      systemctl daemon-reload
   3) From your machine: make deploy-api / make deploy-bot
      (or deploy/deploy-api.sh etc.)
============================================================
EOF
