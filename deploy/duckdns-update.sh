#!/usr/bin/env bash
# Keep the DuckDNS name pointed at this VM's current public IP.
# Reads DUCKDNS_DOMAIN and DUCKDNS_TOKEN from /etc/feedbridge/duckdns.env.
# Runs from /etc/cron.d/feedbridge every 5 minutes; prints only on failure.
set -euo pipefail

ENV_FILE=/etc/feedbridge/duckdns.env
[[ -r "$ENV_FILE" ]] || { echo "$(date -Is) missing $ENV_FILE" >&2; exit 0; }
# shellcheck disable=SC1090
. "$ENV_FILE"
if [[ -z "${DUCKDNS_TOKEN:-}" ]]; then
    exit 0 # not configured yet
fi

# Pass the URL on stdin so the token never shows up in the process list.
# An empty ip= makes DuckDNS use the address the request comes from.
resp="$(printf 'url = "https://www.duckdns.org/update?domains=%s&token=%s&ip="\n' \
    "${DUCKDNS_DOMAIN:-feedbridge}" "$DUCKDNS_TOKEN" \
    | curl -fsS --max-time 20 -K -)"
if [[ "$resp" != "OK" ]]; then
    echo "$(date -Is) duckdns update failed: ${resp}" >&2
    exit 1
fi
