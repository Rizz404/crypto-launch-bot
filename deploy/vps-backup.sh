#!/usr/bin/env bash
# Back up only this Compose project's DB and private configuration.
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
work=$(mktemp -d "$backup_dir/.incomplete-XXXXXXXX")
trap 'rm -rf -- "$work"' EXIT
dc() { docker compose --env-file .env.vps -f deploy/compose.vps.yml "$@"; }
dc exec -T db pg_dump -U bot -d tradingbot -Fc > "$work/database.dump"
# Validate the archive directory; a full restore drill is still a separate check.
dc exec -T db pg_restore --list < "$work/database.dump" > "$work/archive-list.txt"
tar -czf "$work/config.tar.gz" .env.vps configs/config.yaml
git rev-parse HEAD > "$work/source-commit.txt"
(cd "$work" && sha256sum database.dump config.tar.gz > SHA256SUMS)
chmod 600 "$work"/*
destination="$backup_dir/backup-$(date -u +%Y%m%dT%H%M%SZ)"
mv -T -- "$work" "$destination"
trap - EXIT
# Retain the seven newest successful backups. Never traverse symlinks or other names.
python3 - "$backup_dir" <<'PY'
import pathlib, re, shutil, sys
root = pathlib.Path(sys.argv[1])
backups = sorted(p for p in root.iterdir() if not p.is_symlink() and p.is_dir()
                 and re.fullmatch(r"backup-\d{8}T\d{6}Z", p.name))
for old in backups[:-7]:
    shutil.rmtree(old)
PY
echo "Backup completed: $destination"
