#!/usr/bin/env bash
# Bootstrap a fresh Ubuntu (amd64) VM for FEEDBRIDGE.
# Tuned for Google Cloud Always Free e2-micro (1 GB RAM).
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
    # Ubuntu's useradd has no --group (that is Debian adduser). -U makes a
    # matching group; -d sets the home path without requiring it to exist yet.
    useradd --system -U -d /opt/feedbridge -s /usr/sbin/nologin feedbridge
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

# Keep the 1 GB e2-micro from swapping under Postgres defaults (~128MB+).
sudo -u postgres psql -c "ALTER SYSTEM SET shared_buffers = '64MB';"
sudo -u postgres psql -c "ALTER SYSTEM SET work_mem = '4MB';"
sudo -u postgres psql -c "ALTER SYSTEM SET maintenance_work_mem = '32MB';"
sudo -u postgres psql -c "ALTER SYSTEM SET effective_cache_size = '256MB';"

echo "==> redis"
REDIS_CONF=/etc/redis/redis.conf
if grep -q '^requirepass ' "$REDIS_CONF"; then
    REDIS_PASS="$(awk '/^requirepass /{print $2; exit}' "$REDIS_CONF")"
else
    REDIS_PASS="$(openssl rand -hex 16)"
    echo "requirepass ${REDIS_PASS}" >> "$REDIS_CONF"
fi
grep -q '^bind 127.0.0.1' "$REDIS_CONF" || sed -i 's/^bind .*/bind 127.0.0.1 ::1/' "$REDIS_CONF"
if ! grep -q '^maxmemory ' "$REDIS_CONF"; then
    echo "maxmemory 64mb" >> "$REDIS_CONF"
    echo "maxmemory-policy allkeys-lru" >> "$REDIS_CONF"
fi

systemctl enable --now redis-server postgresql
systemctl restart postgresql redis-server

echo "==> firewall"
ufw default deny incoming
ufw default allow outgoing
ufw allow OpenSSH
yes | ufw enable

cat <<EOF

============================================================
 Server bootstrap complete.

 REDIS_PASSWORD=${REDIS_PASS}

 Next steps:
   1) Copy env examples to /etc/feedbridge and fill secrets:
        sudo cp ~/deploy/env/api.env.example /etc/feedbridge/api.env
        sudo cp ~/deploy/env/bot.env.example /etc/feedbridge/bot.env
        sudo chmod 600 /etc/feedbridge/*.env
        sudoedit /etc/feedbridge/api.env /etc/feedbridge/bot.env
      Use the REDIS_PASSWORD printed above and the Postgres password
      you just set. Set TELEGRAM_BOT_TOKEN in bot.env.
      On the e2-micro keep WORKER_COUNT=3.
   2) Install units:
        sudo cp ~/deploy/feedbridge-*.service /etc/systemd/system/
        sudo systemctl daemon-reload
   3) From your laptop (do not compile on this VM):
        make -C API deploy HOST=\$USER@\$(curl -s ifconfig.me)
        make -C bot deploy HOST=\$USER@\$(curl -s ifconfig.me)
   4) sudo systemctl enable --now feedbridge-api feedbridge-bot
============================================================
EOF
