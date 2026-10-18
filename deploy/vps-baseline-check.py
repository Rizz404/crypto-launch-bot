#!/usr/bin/env python3
"""Compare the other projects on a shared server with evidence/baseline.json.

Run on the VPS (as root) before and after every change to trading-bot-paper:
  python3 /opt/trading-bot-paper/repo/deploy/vps-baseline-check.py
Containers that restarted only because the host booted are counted separately,
so a real change (missing, stopped, restarted, new) still stands out.
Exit status 1 when something other than the host boot changed.
"""
import datetime as dt
import json
import pathlib
import subprocess
import sys

BASELINE = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else \
    pathlib.Path(__file__).resolve().parent.parent.parent / "evidence" / "baseline.json"
PROJECT = "trading-bot-paper"


def run(args):
    return subprocess.run(args, capture_output=True, text=True, check=True).stdout


def started(value):
    # Docker prints nanoseconds; Python keeps microseconds.
    head, _, frac = value.rstrip("Z").partition(".")
    return dt.datetime.fromisoformat(head + "." + (frac + "000000")[:6]).replace(tzinfo=dt.timezone.utc)


base = json.loads(BASELINE.read_text())
boot = dt.datetime.now(dt.timezone.utc) - dt.timedelta(seconds=float(open("/proc/uptime").read().split()[0]))
ids = run(["docker", "ps", "-aq"]).split()
now = {}
for v in json.loads(run(["docker", "inspect", *ids])) if ids else []:
    if v["Config"]["Labels"].get("com.docker.compose.project") == PROJECT:
        continue
    now[v["Id"]] = {"name": v["Name"], "status": v["State"]["Status"], "started": v["State"]["StartedAt"],
                    "restarts": v["RestartCount"]}

differences, after_boot = [], 0
for c in base["containers"]:
    cur = now.pop(c["id"], None)
    if cur is None:
        differences.append(["missing", c["name"]])
    elif cur["status"] != c["status"]:
        differences.append(["status", c["name"], c["status"], cur["status"]])
    elif cur["started"] != c["started"] or cur["restarts"] != c["restarts"]:
        if cur["status"] == "running" and abs((started(cur["started"]) - boot).total_seconds()) < 600:
            after_boot += 1
        else:
            differences.append(["restarted", c["name"], cur["started"], cur["restarts"]])
for cur in now.values():
    differences.append(["new", cur["name"]])

services = {}
for name in base["services"]:
    out = run(["systemctl", "show", name, "-p", "ActiveState,SubState"])
    state = dict(line.split("=", 1) for line in out.strip().splitlines())
    services[name] = state.get("ActiveState") + "/" + state.get("SubState")
    if state.get("ActiveState") != "active":
        differences.append(["service", name, services[name]])

print(json.dumps({"host_boot": boot.isoformat(timespec="seconds"), "containers_checked": len(base["containers"]),
                  "restarted_by_host_boot": after_boot, "services": services, "differences": differences}, indent=1))
sys.exit(1 if differences else 0)
