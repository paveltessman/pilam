#!/usr/bin/env bash
#
# Release one image tag. Runs on the server, started by CI or by hand:
#
#   ./deploy.sh 3f2a1c9e...    roll forward, or back, to one commit
#   ./deploy.sh latest         whatever the last build pushed
#
# The steps are: record the tag, pull, migrate, restart, wait for health. A
# migration that fails stops the script and leaves the old container serving.

set -euo pipefail

TAG="${1:?usage: deploy.sh <image-tag>}"
DIR="${PILAM_DIR:-/srv/pilam}"
REGISTRY="${PILAM_REGISTRY:-ghcr.io/paveltessman/pilam}"
IMAGE="${REGISTRY}:${TAG}"

COMPOSE=(docker compose -f "${DIR}/docker-compose.prod.yml")

cd "$DIR"

# CI renders this file from deploy/env.template and the repository secrets, and
# installs it just before this script runs. To deploy by hand, render it once
# yourself. docs/v1/deploy.md says how.
if [ ! -f .env ]; then
	echo "deploy: ${DIR}/.env is missing. See docs/v1/deploy.md." >&2
	exit 1
fi

echo ">> deploying ${IMAGE}"

# Record the tag before the pull, so that a later `docker compose up -d` typed
# by hand starts the same image this run started.
if grep -q '^PILAM_IMAGE=' .env; then
	sed -i "s|^PILAM_IMAGE=.*|PILAM_IMAGE=${IMAGE}|" .env
else
	printf 'PILAM_IMAGE=%s\n' "$IMAGE" >> .env
fi

"${COMPOSE[@]}" pull

# Migrations run here, as a dependency of app. Compose stops on a non-zero exit.
"${COMPOSE[@]}" up -d

echo ">> waiting for the health check"
container="$("${COMPOSE[@]}" ps -q app)"
for _ in $(seq 1 30); do
	state="$(docker inspect -f '{{.State.Health.Status}}' "$container")"
	if [ "$state" = "healthy" ]; then
		echo ">> ${IMAGE} is serving"
		exit 0
	fi
	sleep 2
done

echo "deploy: the app did not report healthy within 60 seconds" >&2
"${COMPOSE[@]}" logs --tail 50 app >&2
exit 1
