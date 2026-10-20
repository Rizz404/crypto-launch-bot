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


def database(exchange):
    # Series of the active exchange recorded within the last day; gaps and
    # age are judged against each series' own timeframe.
    sql = """
    WITH tf(timeframe, seconds) AS (VALUES ('1m',60),('3m',180),('5m',300),('15m',900),('30m',1800),
        ('1h',3600),('2h',7200),('4h',14400),('6h',21600),('12h',43200),('1d',86400)),
    recent AS (
      SELECT c.exchange, c.market, c.symbol, c.timeframe, c.open_time, tf.seconds,
        lag(c.open_time) OVER (PARTITION BY c.exchange,c.market,c.symbol,c.timeframe ORDER BY c.open_time) AS previous
      FROM candles c JOIN tf USING (timeframe)
      WHERE c.exchange = '%s' AND c.open_time >= now() - interval '48 hours'
    ), series AS (
      SELECT exchange,market,symbol,timeframe,max(seconds) AS tf_seconds,count(*) AS bars,max(open_time) AS last_open,
        extract(epoch FROM now()-max(open_time))::int AS age_seconds,
        count(*) FILTER (WHERE open_time >= now() - interval '24 hours'
          AND extract(epoch FROM open_time-previous) > seconds) AS gaps
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
    out = json.loads(psql(sql % exchange))
    out["managed"] = []
    if psql("SELECT to_regclass('managed_wallets') IS NOT NULL").strip() == "t":
        # Ledger health and hot-state size (compaction keeps it bounded).
        out["managed"] = json.loads(psql("""
        SELECT coalesce(json_agg(w),'[]'::json) FROM
          (SELECT id, revision, octet_length(state::text) AS state_bytes,
             nullif(state->>'Attention','') AS attention, coalesce((state->>'Halted')::boolean,false) AS halted,
             -- Go encodes empty collections as JSON null, which coalesce does not catch.
             CASE jsonb_typeof(state->'Pending') WHEN 'array' THEN jsonb_array_length(state->'Pending') ELSE 0 END AS pending,
             CASE jsonb_typeof(state->'Orders') WHEN 'object' THEN (SELECT count(*) FROM jsonb_object_keys(state->'Orders')) ELSE 0 END AS orders
           FROM managed_wallets) w"""))
    return out


def psql(sql):
    return command(DC + ["exec", "-T", "db", "psql", "-U", "bot", "-d", DB_NAME,
                         "-v", "ON_ERROR_STOP=1", "-At", "-c", sql])


def env_value(name, default):
    # Read only non-secret fields; never execute or expose the private env file.
    for line in (REPO / ".env.vps").read_text().splitlines():
        if line.startswith(name + "="):
            return line.split("=", 1)[1].strip() or default
    return default


def port(name, default):
    return int(env_value(name, default))


now = dt.datetime.now(dt.timezone.utc)
mode = env_value("TB_APP__MODE", "paper")
DB_NAME = env_value("BOT_DB_NAME", "tradingbot")
exchange = "binance-testnet" if mode == "testnet" else "binance"
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
    "mode": mode,
    "database": attempt(lambda: database(exchange)),
    "metrics": attempt(lambda: [line for line in http("http://127.0.0.1:%d/metrics" % metrics_port).splitlines()
                                if line.startswith(("tradingbot_", "process_start_time_seconds "))]),
    "disk_free_bytes": disk.free,
    "load": os.getloadavg(),
}
problems = [key for key in ("containers", "health", "grafana", "prometheus", "database", "metrics")
            if not sample[key]["ok"]]
if sample["containers"]["ok"]:
    rows = sample["containers"]["data"]
    if {r["service"] for r in rows} != {"bot", "db", "dns", "prometheus", "grafana"} or any(
            r["status"] != "running" or r["oom_killed"] for r in rows):
        problems.append("container_state")
    # A crash loop keeps the container "running" between restarts; compare
    # restart counts of the same container with the previous sample.
    previous = {}
    for path in sorted(EVIDENCE.glob("samples-????????.jsonl"))[-2:]:
        for line in path.read_text().splitlines()[-1:]:
            try:
                last = json.loads(line)
                previous = {c["id"]: c["restarts"] for c in last["containers"]["data"]} if last["containers"]["ok"] else {}
            except (ValueError, KeyError):
                pass
    if any(r["restarts"] > previous.get(r["id"], r["restarts"]) for r in rows):
        problems.append("container_restarted")
if sample["database"]["ok"]:
    series = sample["database"]["data"]["series"]
    # A closed candle is stored once its period ends: allow two periods plus a margin.
    if not series or any(r["age_seconds"] > 2 * r["tf_seconds"] + 120 or r["gaps"] for r in series):
        problems.append("candle_freshness_or_gap")
    if any(w["attention"] or w["halted"] for w in sample["database"]["data"]["managed"]):
        problems.append("managed_attention")
if sample["prometheus"]["ok"]:
    targets = sample["prometheus"]["data"]["data"]["activeTargets"]
    if not targets or any(t["health"] != "up" for t in targets):
        problems.append("prometheus_target")
if sample["metrics"]["ok"] and not any(
        line.startswith("tradingbot_build_info{") and 'mode="%s"' % mode in line
        for line in sample["metrics"]["data"]):
    problems.append("unexpected_mode")
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
