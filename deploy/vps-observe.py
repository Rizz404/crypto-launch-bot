#!/usr/bin/env python3
"""Append bounded, credential-free operational evidence for this paper stack."""
import datetime as dt
import json
import os
from pathlib import Path
import shutil
import subprocess
import urllib.request

os.umask(0o077)
REPO = Path(__file__).resolve().parent.parent
EVIDENCE = REPO.parent / "evidence"
DC = ["docker", "compose", "--env-file", ".env.vps", "-f", "deploy/compose.vps.yml"]
LOCAL_HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def command(args):
    # Do not forward stderr: Docker/DB failures can include sensitive context.
    result = subprocess.run(args, cwd=REPO, capture_output=True, text=True, timeout=30)
    if result.returncode:
        raise RuntimeError("command failed (exit %d)" % result.returncode)
    return result.stdout


def http(url):
    with LOCAL_HTTP.open(url, timeout=5) as response:
        return response.read().decode()


def attempt(fn):
    try:
        return {"ok": True, "data": fn()}
    except Exception as exc:
        return {"ok": False, "error": type(exc).__name__}


def containers():
    ids = command(DC + ["ps", "-aq"]).split()
    rows = json.loads(command(["docker", "inspect", *ids])) if ids else []
    return [{"service": v["Config"]["Labels"]["com.docker.compose.service"],
             "id": v["Id"], "status": v["State"]["Status"],
             "started": v["State"]["StartedAt"], "restarts": v["RestartCount"],
             "oom_killed": v["State"]["OOMKilled"], "image": v["Image"]} for v in rows]


def database():
    # The deployed config has exactly two 1m series. Count gaps inside the last day.
    sql = """
    WITH recent AS (
      SELECT exchange, market, symbol, timeframe, open_time,
        lag(open_time) OVER (PARTITION BY exchange,market,symbol,timeframe ORDER BY open_time) AS previous
      FROM candles WHERE open_time >= now() - interval '24 hours'
    ), series AS (
      SELECT exchange,market,symbol,timeframe,count(*) AS bars,max(open_time) AS last_open,
        extract(epoch FROM now()-max(open_time))::int AS age_seconds,
        count(*) FILTER (WHERE open_time-previous > interval '1 minute') AS gaps
      FROM recent GROUP BY exchange,market,symbol,timeframe
    )
    SELECT json_build_object(
      'series',(SELECT coalesce(json_agg(series),'[]'::json) FROM series),
      'runs',(SELECT coalesce(json_agg(r),'[]'::json) FROM
        (SELECT id,mode,started_at,stopped_at FROM strategy_runs ORDER BY id DESC LIMIT 3) r),
      'orders',(SELECT count(*) FROM orders), 'fills',(SELECT count(*) FROM fills),
      'trades',(SELECT count(*) FROM trades),
      'last_equity',(SELECT max(ts) FROM equity_snapshots),
      'database_bytes',pg_database_size(current_database()));
    """
    return json.loads(command(DC + ["exec", "-T", "db", "psql", "-U", "bot", "-d",
                                  "tradingbot", "-v", "ON_ERROR_STOP=1", "-At", "-c", sql]))


def port(name, default):
    # Read only numeric port fields; never execute or expose the private env file.
    for line in (REPO / ".env.vps").read_text().splitlines():
        if line.startswith(name + "="):
            return int(line.split("=", 1)[1])
    return default


now = dt.datetime.now(dt.timezone.utc)
metrics_port = port("BOT_METRICS_PORT", 19100)
prometheus_port = port("PROMETHEUS_PORT", 19090)
grafana_port = port("GRAFANA_PORT", 13000)
disk = shutil.disk_usage(REPO)
sample = {
    "time": now.isoformat(),
    "containers": attempt(containers),
    "health": attempt(lambda: http("http://127.0.0.1:%d/healthz" % metrics_port).strip()),
    "grafana": attempt(lambda: json.loads(http("http://127.0.0.1:%d/api/health" % grafana_port))),
    "prometheus": attempt(lambda: json.loads(http("http://127.0.0.1:%d/api/v1/targets" % prometheus_port))),
    "database": attempt(database),
    "metrics": attempt(lambda: [line for line in http("http://127.0.0.1:%d/metrics" % metrics_port).splitlines()
                                if line.startswith(("tradingbot_", "process_start_time_seconds "))]),
    "disk_free_bytes": disk.free,
    "load": os.getloadavg(),
}
problems = [key for key in ("containers", "health", "grafana", "prometheus", "database", "metrics")
            if not sample[key]["ok"]]
if sample["containers"]["ok"]:
    rows = sample["containers"]["data"]
    if {r["service"] for r in rows} != {"bot", "db", "prometheus", "grafana"} or any(
            r["status"] != "running" or r["oom_killed"] for r in rows):
        problems.append("container_state")
if sample["database"]["ok"]:
    series = sample["database"]["data"]["series"]
    if len(series) != 2 or any(r["age_seconds"] > 180 or r["gaps"] for r in series):
        problems.append("candle_freshness_or_gap")
if sample["prometheus"]["ok"]:
    targets = sample["prometheus"]["data"]["data"]["activeTargets"]
    if not targets or any(t["health"] != "up" for t in targets):
        problems.append("prometheus_target")
if sample["metrics"]["ok"] and not any(
        line.startswith("tradingbot_build_info{") and 'mode="paper"' in line
        for line in sample["metrics"]["data"]):
    problems.append("paper_mode")
if disk.free < 5 * 1024 ** 3:
    problems.append("low_disk")
sample["problems"] = problems
EVIDENCE.mkdir(mode=0o700, exist_ok=True)
with (EVIDENCE / ("samples-" + now.strftime("%Y%m%d") + ".jsonl")).open("a") as output:
    output.write(json.dumps(sample, separators=(",", ":")) + "\n")
for old in EVIDENCE.glob("samples-????????.jsonl"):
    if not old.is_symlink() and old.is_file() and old.stat().st_mtime < now.timestamp() - 14 * 86400:
        old.unlink()
print(json.dumps({"time": sample["time"], "problems": problems}))
raise SystemExit(1 if problems else 0)
