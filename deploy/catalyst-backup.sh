#!/usr/bin/env bash
# Quiesce only P3 while copying its ledger, thesis inbox, evidence and AI budget.
set -euo pipefail
umask 077
repo=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$repo"
backup_dir="$repo/../backups"
mkdir -p "$backup_dir"
chmod 700 "$backup_dir"
exec 9>"$backup_dir/.lock"
flock -n 9 || exit 0
available_kb=$(df -Pk "$backup_dir" | awk 'NR == 2 {print $4}')
if (( available_kb < 2097152 )); then
  echo 'Backup skipped: less than 2 GiB available' >&2
  exit 1
fi
dc() { docker compose --env-file .env.catalyst-image -f deploy/compose.catalyst.yml -f deploy/compose.catalyst.pilot.yml "$@"; }
cid=$(dc ps -aq catalyst)
[[ -n "$cid" ]]
archive=$(docker inspect -f '{{range .Mounts}}{{if eq .Destination "/var/lib/catalyst"}}{{.Source}}{{end}}{{end}}' "$cid")
[[ -d "$archive" ]]
work=$(mktemp -d "$backup_dir/.incomplete-XXXXXXXX")
restart_needed=0
cleanup() {
  result=$?
  if (( restart_needed )); then
    if ! docker start "$cid" >/dev/null; then
      echo 'P3 restart failed; inspect the existing pilot container' >&2
      result=1
    fi
  fi
  rm -rf -- "$work"
  exit "$result"
}
trap cleanup EXIT
if [[ $(docker inspect -f '{{.State.Running}}' "$cid") == true ]]; then
  restart_needed=1
  docker stop --time 30 "$cid" >/dev/null
fi
[[ $(docker inspect -f '{{.State.Running}}' "$cid") == false ]]
dc exec -T db pg_dump -U catalyst -d cryptolaunchbot_catalyst_paper_v1 -Fc </dev/null > "$work/database.dump"
dc exec -T db pg_restore --list < "$work/database.dump" > "$work/archive-list.txt"
tar -C "$archive" -czf "$work/archive.tar.gz" .
tar -czf "$work/config.tar.gz" .env.catalyst .env.catalyst-image configs/catalyst.policy.json deploy/compose.catalyst.yml deploy/compose.catalyst.pilot.yml
git rev-parse HEAD > "$work/source-commit.txt"
docker inspect -f '{{.Image}}' "$cid" > "$work/image.txt"
date -u +%FT%TZ > "$work/created-at.txt"
(cd "$work" && sha256sum database.dump archive-list.txt archive.tar.gz config.tar.gz source-commit.txt image.txt created-at.txt > SHA256SUMS)
chmod 600 "$work"/*
if (( restart_needed )); then
  docker start "$cid" >/dev/null
  restart_needed=0
fi
destination="$backup_dir/backup-$(date -u +%Y%m%dT%H%M%SZ)"
mv -T -- "$work" "$destination"
trap - EXIT
echo "Backup completed: $destination"
