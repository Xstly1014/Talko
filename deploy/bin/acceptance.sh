#!/usr/bin/env bash
source "$(dirname -- "$0")/common.sh"
# Dedicated staging stack only. This script stops worker temporarily to exercise recovery.
dc config --quiet
dc run --rm --no-deps --entrypoint promtool prometheus check config /etc/prometheus/prometheus.yml
dc run --rm --no-deps --entrypoint amtool alertmanager check-config /etc/alertmanager/alertmanager.yml
dc up -d --build
ready=false
for attempt in $(seq 1 90); do
 if curl -fsS http://127.0.0.1:5183/api/v1/feed >/dev/null && dc exec -T review-web wget -q -O /dev/null http://review:8081/readyz && dc exec -T community-web wget -q -O /dev/null http://worker:8085/readyz; then ready=true; break; fi
 sleep 2
done
[[ "$ready" == true ]] || exit 1
# Metrics unavailable must be observable, then recover after worker restart.
trap 'dc start worker >/dev/null' EXIT
dc stop worker
sleep 20
curl -fsS --get --data-urlencode 'query=up{job="worker"}' http://127.0.0.1:9090/api/v1/query | python3 -c 'import json,sys; r=json.load(sys.stdin)["data"]["result"]; assert r and all(x["value"][1]=="0" for x in r),r'
# Wait until the configured alert actually fires, not merely until up becomes zero.
fired=false
for attempt in $(seq 1 60); do
 if curl -fsS http://127.0.0.1:9090/api/v1/alerts | python3 -c 'import json,sys; a=json.load(sys.stdin)["data"]["alerts"]; assert any(x["labels"].get("alertname")=="MetricsUnavailable" and x["labels"].get("job")=="worker" and x["state"]=="firing" for x in a)'; then fired=true; break; fi
 sleep 2
done
[[ "$fired" == true ]] || exit 1
dc start worker
sleep 20
curl -fsS --get --data-urlencode 'query=up{job="worker"}' http://127.0.0.1:9090/api/v1/query | python3 -c 'import json,sys; r=json.load(sys.stdin)["data"]["result"]; assert r and all(x["value"][1]=="1" for x in r),r'
recovered=false
for attempt in $(seq 1 30); do
 if curl -fsS http://127.0.0.1:9090/api/v1/alerts | python3 -c 'import json,sys; a=json.load(sys.stdin)["data"]["alerts"]; assert not any(x["labels"].get("alertname")=="MetricsUnavailable" and x["labels"].get("job")=="worker" for x in a)'; then recovered=true; break; fi
 sleep 2
done
[[ "$recovered" == true ]] || exit 1
# Independent canary verifies BLOB content survived the logical backup, not just table creation.
mysql_root talko -e "CREATE TABLE IF NOT EXISTS restore_probe(id INT PRIMARY KEY,body BLOB); INSERT INTO restore_probe VALUES(1,UNHEX(SHA2('talko-restore',256))) ON DUPLICATE KEY UPDATE body=VALUES(body);"
backup="$(bash "$DEPLOY_DIR/bin/backup.sh" "$ROOT_DIR/.tmp/acceptance-backups")"
evidence="$(bash "$DEPLOY_DIR/bin/restore-check.sh" "$backup")"
printf '%s\n' "$evidence"
restored="$(printf '%s\n' "$evidence" | sed -n 's/^isolated_restore_database=//p')"
[[ "$restored" =~ ^talko_restore_[0-9_]+$ ]] || exit 1
mysql_root "$restored" -e "SELECT body=UNHEX(SHA2('talko-restore',256)) FROM restore_probe WHERE id=1" | grep -qx 1
echo 'Container startup, scrape failure/recovery and isolated restore passed.'
