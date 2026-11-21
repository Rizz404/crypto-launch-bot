#!/usr/bin/env bash
# One research pass on the Plan 4 archive: collect every source, write a report, then
# advance the virtual shadow pilot. No inference, wallet or trading credentials.
# Run by crypto-launch-collect.service.
set -euo pipefail
cd /opt/crypto-launch/repo
image=$(cat /opt/crypto-launch/image)
spider=${SPIDER_IMAGE:-ghcr.io/rizz404/ana-scout:sha-c1ce8569c15eaa1685ad2488a9293ac041411150}
# The host runs a daily `docker image prune -af --filter until=24h`, which removes images
# no container references; `run --rm` leaves none. Rebuild a pruned image from its commit,
# then keep stopped pin containers so neither image is pruned again.
if ! docker image inspect "$image" > /dev/null 2>&1; then
  echo "rebuilding pruned $image" >&2
  git archive "${image##*:sha-}" | docker build -q -f Dockerfile.launch --build-arg SPIDER_IMAGE="$spider" \
    --build-arg SPIDER_PATH=/usr/local/bin/spider -t "$image" - > /dev/null
fi
pin() {
  [ "$(docker inspect -f '{{.Config.Image}}' "$1" 2> /dev/null)" = "$2" ] && return
  docker rm -f "$1" > /dev/null 2>&1 || true
  docker create --name "$1" --network none --entrypoint /bin/true "$2" > /dev/null
}
pin crypto-launch-pin "$image"
pin crypto-launch-spider-pin "$spider"
reports=/opt/crypto-launch/reports
mkdir -p "$reports"
stamp=$(date -u +%Y%m%dT%H%M%SZ)
run() { LAUNCH_IMAGE="$image" docker compose -f deploy/compose.launch.yml run --rm -T --pull never launch "$@"; }
status=0
for source in curve dex cex presale; do
  # Each source keeps its own cursor/cooldown; one failing source must not skip the others.
  if ! run -action collect -source "$source" -limit 10 -archive /var/lib/launch/research -spider-binary /usr/local/bin/spider > /dev/null; then
    echo "collect $source failed" >&2
    status=1
  fi
done
run -action report -archive /var/lib/launch/research > "$reports/report-$stamp.json"
ln -sfn "report-$stamp.json" "$reports/latest.json"
# No-AI baseline pilot launch-pilot-v1: virtual entries/exits on live quotes only.
if run -action shadow -archive /var/lib/launch/research > "$reports/shadow-$stamp.json"; then
  ln -sfn "shadow-$stamp.json" "$reports/shadow-latest.json"
else
  echo "shadow pass failed" >&2
  status=1
fi
# Keep 30 days of reports; the archive itself is never pruned here.
find "$reports" \( -name 'report-*.json' -o -name 'shadow-*.json' \) -mtime +30 -delete
exit "$status"
