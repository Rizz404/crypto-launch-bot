#!/usr/bin/env bash
# Build the bot image on this server from a pushed commit and point .env.vps at it.
# The build context is `git archive` of that commit, so untracked configs and
# .env.vps never enter it. CI still runs on every push as the quality gate.
#
# Usage (from /opt/ana-core/repo): bash deploy/vps-build.sh <commit>
# Then recreate the bot: docker compose --env-file .env.vps -f deploy/compose.vps.yml up -d --no-deps bot
set -euo pipefail
cd "$(dirname "$0")/.."

commit=${1:?usage: vps-build.sh <commit>}
git fetch -q origin
sha=$(git rev-parse --verify "$commit^{commit}")
git checkout -q "$sha"
image="ana-core:sha-$sha"

start=$(date +%s)
git archive "$sha" | DOCKER_BUILDKIT=1 docker build -q --build-arg VERSION="$sha" -t "$image" - >/dev/null
echo "built $image in $(($(date +%s) - start))s"

echo "BOT_IMAGE=$image" | python3 deploy/vps-set-env.py

# Keep the five newest local builds for rollback; a running image refuses removal.
docker image ls ana-core --format '{{.CreatedAt}}|{{.Repository}}:{{.Tag}}' | sort -r | tail -n +6 | cut -d'|' -f2 |
	xargs -r docker image rm >/dev/null 2>&1 || true
