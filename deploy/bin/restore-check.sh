#!/usr/bin/env bash
source "$(dirname -- "$0")/common.sh"
umask 077
[[ $# == 1 ]] || { echo 'usage: restore-check.sh backup.sql.gz' >&2; exit 2; }
file="$(realpath -- "$1")"
[[ -f "$file" && -f "$file.sha256" ]] || { echo 'backup or checksum missing' >&2; exit 2; }
(cd -- "$(dirname -- "$file")" && sha256sum -c -- "$(basename -- "$file").sha256")
gzip -t "$file"
db="talko_restore_$(date -u +%Y%m%d_%H%M%S)_$$"
[[ "$db" =~ ^talko_restore_[0-9_]+$ ]] || exit 2
# Fresh isolated database only, never caller-supplied production target.
printf 'CREATE DATABASE %s CHARACTER SET utf8mb4;\n' "$db" | mysql_root
printf 'isolated_restore_database=%s\n' "$db"
zcat -- "$file" | mysql_root "$db"
dc exec -T mysql sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysqlcheck -uroot "$1"' sh "$db"
mysql_root "$db" <<'SQL'
SELECT 'schema_version',MAX(version) FROM schema_migrations;
SELECT 'users',COUNT(*) FROM users;
SELECT 'posts',COUNT(*) FROM posts;
SELECT 'revisions',COUNT(*) FROM revisions;
SELECT 'media',COUNT(*),COALESCE(SUM(OCTET_LENGTH(body)),0) FROM media;
SELECT 'outbox',COUNT(*) FROM outbox;
SELECT 'audit_log',COUNT(*) FROM audit_log;
SQL
orphans="$(mysql_root "$db" <<'SQL'
SELECT (SELECT COUNT(*) FROM revisions r LEFT JOIN posts p ON p.id=r.post_id WHERE p.id IS NULL)+(SELECT COUNT(*) FROM comments c LEFT JOIN posts p ON p.id=c.post_id WHERE p.id IS NULL)+(SELECT COUNT(*) FROM outbox_delivery d LEFT JOIN outbox o ON o.id=d.event_id WHERE o.id IS NULL);
SQL
)"
[[ "$orphans" == 0 ]] || { echo 'restore failed: orphan rows found' >&2; exit 1; }
echo 'Integrity checks passed; isolated database retained for business verification. No production data replaced.'
