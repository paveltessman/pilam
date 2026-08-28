#!/usr/bin/env bash
#
# Dump the database and the uploaded photos, then drop what is older than
# KEEP_DAYS.
#
# Restore instructions are in docs/v1/deploy.md.

set -euo pipefail

DIR="${PILAM_DIR:-/srv/pilam}"
OUT="${PILAM_BACKUP_DIR:-${DIR}/backups}"
KEEP_DAYS="${KEEP_DAYS:-180}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"

COMPOSE=(docker compose -f "${DIR}/docker-compose.prod.yml")

mkdir -p "$OUT"

"${COMPOSE[@]}" exec -T db sh -c \
	'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --format=custom' \
	> "${OUT}/pilam-${STAMP}.dump"

# A throwaway container, because the app image runs as an unprivileged user that
# cannot write to a host directory owned by root.
docker run --rm \
	-v pilam_media:/media:ro \
	-v "${OUT}:/backup" \
	alpine:3.22 tar -czf "/backup/media-${STAMP}.tar.gz" -C /media .

find "$OUT" -type f -mtime "+${KEEP_DAYS}" -delete

# Old image layers pile up, one release at a time, on a small disk.
docker image prune -f >/dev/null

echo "backup: wrote pilam-${STAMP}.dump and media-${STAMP}.tar.gz"
