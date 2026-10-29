#!/usr/bin/env bash
set -euo pipefail
DEPLOY_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
ROOT_DIR="$(cd -- "$DEPLOY_DIR/.." && pwd)"
# Parse credentials as Compose data, never source them as shell code.
compose=(docker compose --project-directory "$DEPLOY_DIR" -f "$DEPLOY_DIR/compose.yaml")
if [[ -n "${TALKO_ENV_FILE:-}" ]]; then compose+=(--env-file "$TALKO_ENV_FILE"); fi
if [[ "${FEISHU_ENABLED:-false}" == true ]]; then compose+=(-f "$DEPLOY_DIR/compose.alerts.yaml"); fi
dc(){ "${compose[@]}" "$@"; }
mysql_root(){ dc exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot --batch --skip-column-names "$@"' sh "$@"; }
