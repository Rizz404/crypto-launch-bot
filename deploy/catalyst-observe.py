#!/usr/bin/env python3
"""Read-only P3 checks; emit local systemd alerts without sending chat messages."""
import base64
import datetime as dt
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import urllib.request

REPO = Path(__file__).resolve().parent.parent
DC = ["docker", "compose", "--env-file", ".env.catalyst-image", "-f",
      "deploy/compose.catalyst.yml", "-f", "deploy/compose.catalyst.pilot.yml"]
HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def timestamp(value):
    # Python 3.9 on the VPS accepts microseconds, while Go emits nanoseconds.
    value = re.sub(r"\.(\d+)", lambda m: "." + m[1][:6].ljust(6, "0"), value)
    return dt.datetime.fromisoformat(value.replace("Z", "+00:00"))


def command(args):
    result = subprocess.run(args, cwd=REPO, capture_output=True, text=True, timeout=20)
    if result.returncode:
        raise RuntimeError("P3 command failed")
    return result.stdout


def request(path, password=""):
    headers = {}
    if password:
        headers["Authorization"] = "Basic " + base64.b64encode(("observer:" + password).encode()).decode()
    req = urllib.request.Request("http://127.0.0.1:18081" + path, headers=headers)
    with HTTP.open(req, timeout=10) as response:
        return json.load(response)


def problems(sample, policy, now):
    issues = []
    rows = sample["containers"]
    if {c["service"] for c in rows} != {"catalyst", "db", "dns"} or any(
            c["status"] != "running" or c["oom_killed"] or c["health"] == "unhealthy" for c in rows):
        issues.append("container_state")
    status = sample["research"]["Status"]
    def age(value):
        return (now - timestamp(value)).total_seconds()
    tick_age = age(status["TickAt"])
    if not 0 <= tick_age <= 10:
        issues.append("timer_stale")
    if any(not 0 <= age(status.get("Prices", {}).get(symbol, "0001-01-01T00:00:00Z")) <= 15
           for symbol in policy["symbols"]):
        issues.append("quote_stale")
    if not 0 <= age(status["ResearchAt"]) <= 2 * policy["poll_seconds"] + 240:
        issues.append("research_stale")
    if status.get("Error"):
        issues.append("research_error")
    if status.get("SourceGaps"):
        issues.append("source_gap")
    reviews = sample["research"]["Report"].get("Reviews") or []
    if reviews and not max(reviews, key=lambda r: timestamp(r["At"]))["Valid"]:
        issues.append("latest_review_invalid")
    sources = {s["source_id"]: s for s in sample["research"]["Report"]["Sources"]}
    if any(s["id"] not in sources or sources[s["id"]]["status"] != "ok" or
           not 0 <= age(sources[s["id"]]["at"]) <= 2 * policy["poll_seconds"] + 240
           for s in policy["sources"]):
        issues.append("source_stale_or_failed")
    if sample["ledger"]["halted"] or sample["ledger"]["attention"]:
        issues.append("ledger_attention")
    if sample["budget_slots"] > 12:
        issues.append("ai_budget_exceeded")
    if sample["disk_free_bytes"] < 5 * 1024 ** 3:
        issues.append("low_disk")
    if sample["last_backup_age_seconds"] is None or sample["last_backup_age_seconds"] > 36 * 3600:
        issues.append("backup_missing_or_stale")
    return issues


def main():
    os.umask(0o077)
    now = dt.datetime.now(dt.timezone.utc)
    try:
        values = {}
        for line in (REPO / ".env.catalyst").read_text().splitlines():
            if "=" in line and not line.lstrip().startswith("#"):
                key, value = line.split("=", 1)
                values[key.strip()] = value.strip().strip('"').strip("'")
        password = values["TB_CATALYST_WEB_PASSWORD"]
        if not password:
            raise RuntimeError("dashboard password required")
        policy = json.loads((REPO / "configs/catalyst.policy.json").read_text())
        ids = command(DC + ["ps", "-aq"]).split()
        rows = json.loads(command(["docker", "inspect", *ids])) if ids else []
        containers = [{"service": r["Config"]["Labels"]["com.docker.compose.service"],
                       "id": r["Id"], "status": r["State"]["Status"], "restarts": r["RestartCount"],
                       "oom_killed": r["State"]["OOMKilled"],
                       "health": r["State"].get("Health", {}).get("Status", "none")} for r in rows]
        worker = next(r for r in rows if r["Config"]["Labels"]["com.docker.compose.service"] == "catalyst")
        archive = Path(next(m["Source"] for m in worker["Mounts"] if m["Destination"] == "/var/lib/catalyst"))
        day = now.astimezone(dt.timezone(dt.timedelta(hours=7))).date().isoformat()
        budget = archive / "daily-budget" / day
        slots = sum(p.is_file() and p.name.isdigit() for p in budget.iterdir()) if budget.exists() else 0
        sql = "SELECT json_build_object('halted',state->'Halted','attention',state->'Attention','revision',revision) FROM managed_wallets"
        ledger = json.loads(command(DC + ["exec", "-T", "db", "psql", "-qAt", "-U", "catalyst",
                                         "-d", "cryptolaunchbot_catalyst_paper_v1", "-v", "ON_ERROR_STOP=1", "-c", sql]))
        backups = sorted(p for p in (REPO.parent / "backups").glob("backup-*")
                         if p.is_dir() and not p.is_symlink() and (p / "SHA256SUMS").is_file())
        backup_age = (now.timestamp() - backups[-1].stat().st_mtime) if backups else None
        sample = {"time": now.isoformat(), "containers": containers,
                  "health": request("/healthz"), "research": request("/research.json", password),
                  "ledger": ledger, "budget_slots": slots, "wib_day": day,
                  "disk_free_bytes": shutil.disk_usage(REPO).free, "last_backup_age_seconds": backup_age}
        now = dt.datetime.now(dt.timezone.utc)
        sample["time"] = now.isoformat()
        sample["problems"] = problems(sample, policy, now)
        evidence = REPO.parent / "evidence"
        evidence.mkdir(mode=0o700, exist_ok=True)
        previous = evidence / "catalyst-observer-last.json"
        if previous.exists():
            old = json.loads(previous.read_text())
            restarts = {r["id"]: r["restarts"] for r in old.get("containers", [])}
            if any(r["restarts"] > restarts.get(r["id"], r["restarts"]) for r in containers):
                sample["problems"].append("container_restarted")
        temporary = previous.with_suffix(".tmp")
        temporary.write_text(json.dumps(sample, separators=(",", ":")) + "\n")
        temporary.replace(previous)
        # Retain only the latest snapshot; systemd's journal holds alert history.
        print(json.dumps({"time": sample["time"], "problems": sample["problems"]}))
        return bool(sample["problems"])
    except Exception as exc:
        # Upstream errors can contain credentials; only print the exception type.
        print(json.dumps({"time": now.isoformat(), "problems": ["observer_failed"], "error": type(exc).__name__}))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
