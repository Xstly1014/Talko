#!/usr/bin/env bash
source "$(dirname -- "$0")/common.sh"
umask 077
out="${1:-$ROOT_DIR/backups}"
mkdir -p -- "$out"
out="$(cd -- "$out" && pwd)"
file="$out/talko-$(date -u +%Y%m%dT%H%M%SZ)-$$.sql.gz"
trap 'rm -f -- "$file.partial"' EXIT
# Includes media BLOBs. pipefail ensures a failed dump is never marked successful.
dc exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqldump -uroot --single-transaction --quick --hex-blob --set-gtid-purged=OFF --no-tablespaces talko' | gzip > "$file.partial"
gzip -t "$file.partial"
mv -- "$file.partial" "$file"
(cd -- "$out" && sha256sum -- "$(basename -- "$file")" > "$(basename -- "$file").sha256")
printf '%s\n' "$file"
