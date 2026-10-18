#!/usr/bin/env python3
"""Set variables in .env.vps from KEY=VALUE lines on stdin, never printing values.

Secrets travel from the laptop's .env through the SSH pipe only, e.g.:
  grep -E '^TB_TELEGRAM__TOKEN=' .env | ssh plesk 'python3 /opt/trading-bot-paper/repo/deploy/vps-set-env.py'
Non-secret settings can be piped the same way:
  echo TB_APP__MODE=testnet | ssh plesk 'python3 .../vps-set-env.py'
The file is rewritten atomically with mode 600. Only TB_* names are accepted.
Takes effect when the bot container is recreated.
"""
import os
import pathlib
import re
import sys

env = pathlib.Path(sys.argv[1]) if len(sys.argv) > 1 else pathlib.Path(__file__).resolve().parent.parent / ".env.vps"
updates = {}
for line in sys.stdin.read().splitlines():
    if not line.strip():
        continue
    name, sep, value = line.partition("=")
    if not sep or not re.fullmatch(r"TB_[A-Z0-9_]+", name) or not value or "\n" in value:
        sys.exit("refusing input line for %r: expected TB_NAME=value" % name)
    updates[name] = value.strip()
if not updates:
    sys.exit("no KEY=VALUE lines on stdin")

lines = env.read_text().splitlines() if env.exists() else []
lines = [l for l in lines if l.partition("=")[0] not in updates]
lines += [k + "=" + v for k, v in updates.items()]
tmp = env.with_suffix(".tmp")
fd = os.open(tmp, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
with os.fdopen(fd, "w") as f:
    f.write("\n".join(lines) + "\n")
os.replace(tmp, env)
print("set:", ", ".join(sorted(updates)), "| file mode", oct(env.stat().st_mode & 0o777))
