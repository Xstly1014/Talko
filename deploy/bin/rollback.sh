#!/usr/bin/env bash
source "$(dirname -- "$0")/common.sh"
# Roll back only application images after an explicit compatibility assessment.
[[ "${RELEASE:-}" =~ ^[a-f0-9]{40}$ && -n "${IMAGE_PREFIX:-}" ]] || { echo 'tested RELEASE SHA and IMAGE_PREFIX required' >&2; exit 2; }
[[ "${SCHEMA_COMPATIBLE:-}" == yes ]] || { echo 'assess schema compatibility and export SCHEMA_COMPATIBLE=yes' >&2; exit 2; }
mkdir -p "$ROOT_DIR/.tmp/releases"
exec 9>"$ROOT_DIR/.tmp/release.lock"
flock -n 9 || exit 1
apps=(community review worker community-web review-web)
if [[ "${FEISHU_ENABLED:-false}" == true ]]; then apps+=(alertbridge); fi
dc pull "${apps[@]}"
dc up -d --no-build --no-deps --force-recreate "${apps[@]}"
# --no-deps prevents replaying a historical migration job during app rollback.
for attempt in $(seq 1 30); do
 if dc exec -T community-web wget -q -O /dev/null http://community:8080/readyz && dc exec -T review-web wget -q -O /dev/null http://review:8081/readyz; then
  { printf 'rollback=%s\n' "$RELEASE"; dc images --format json; } > "$ROOT_DIR/.tmp/releases/rollback-$(date -u +%Y%m%dT%H%M%SZ).txt"
  echo 'Application rollback healthy; verify publish-review-comment flow. Database unchanged.'; exit 0
 fi
 sleep 2
done
echo 'Rollback health failed; inspect logs. Database unchanged.' >&2
exit 1
