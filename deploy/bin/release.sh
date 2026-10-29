#!/usr/bin/env bash
source "$(dirname -- "$0")/common.sh"
umask 077
[[ "${RELEASE:-}" =~ ^[a-f0-9]{40}$ ]] || { echo 'export RELEASE as the tested full commit SHA' >&2; exit 2; }
[[ -n "${IMAGE_PREFIX:-}" ]] || { echo 'export IMAGE_PREFIX, e.g. ghcr.io/xstly1014/talko' >&2; exit 2; }
mkdir -p "$ROOT_DIR/.tmp/releases"
exec 9>"$ROOT_DIR/.tmp/release.lock"
flock -n 9 || { echo 'another release is running' >&2; exit 1; }
dc config --quiet
apps=(community review worker community-web review-web)
if [[ "${FEISHU_ENABLED:-false}" == true ]]; then apps+=(alertbridge); fi
dc pull "${apps[@]}" migrate
dc up -d mysql
# Wait for database before checking whether a pre-migration backup is needed.
ready=false
for attempt in $(seq 1 60); do
 if mysql_root -e 'SELECT 1' > /dev/null 2>&1; then ready=true; break; fi
 sleep 2
done
[[ "$ready" == true ]] || { echo 'database not ready' >&2; exit 1; }
exists="$(mysql_root -e 'SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=CHAR(116,97,108,107,111) AND table_name=CHAR(117,115,101,114,115)')"
if [[ "$exists" == 1 ]]; then bash "$DEPLOY_DIR/bin/backup.sh"; fi
# Fresh migration container every release; never reuse an old exited job.
dc run --rm migrate
dc up -d --no-build --force-recreate "${apps[@]}" prometheus grafana alertmanager
ok=false
for attempt in $(seq 1 30); do
 if dc exec -T community-web wget -q -O /dev/null http://community:8080/readyz && dc exec -T review-web wget -q -O /dev/null http://review:8081/readyz && dc exec -T community-web wget -q -O /dev/null http://worker:8085/readyz; then ok=true; break; fi
 sleep 2
done
[[ "$ok" == true ]] || { echo 'Health checks failed; inspect logs and choose compatible rollback. No database downgrade.' >&2; exit 1; }
curl --fail --silent --show-error http://127.0.0.1:5183/api/v1/feed > /dev/null
curl --silent --show-error http://127.0.0.1:5184/review-api/v1/auth/login -H 'Content-Type: application/json' -d '{}' -o /dev/null -w '%{http_code}' | grep -qx 401
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
{ printf 'release=%s\ncompleted_at=%s\n' "$RELEASE" "$stamp"; dc images --format json; } > "$ROOT_DIR/.tmp/releases/$stamp-$RELEASE.txt"
echo 'Infrastructure smoke passed. Complete authenticated publish-review-comment acceptance before inviting users.'
