"""Failure-path fixtures only; never connect to Docker, a model or an exchange."""
import copy
import datetime as dt
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("observer", ROOT / "catalyst-observe.py")
observer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(observer)


class ObserverTests(unittest.TestCase):
    def test_go_timestamps_are_readable_on_python_39(self):
        self.assertEqual(123456, observer.timestamp("2026-10-29T14:14:08.123456789Z").microsecond)
        self.assertEqual(100000, observer.timestamp("2026-10-29T14:14:08.1Z").microsecond)
        self.assertEqual(0, observer.timestamp("2026-10-29T14:14:08Z").microsecond)

    def test_runtime_health_does_not_hide_research_failures(self):
        now = dt.datetime.now(dt.timezone.utc)
        at = now.isoformat()
        policy = {"symbols": ["ETHUSDT"], "sources": [{"id": "primary"}], "poll_seconds": 900}
        sample = {"containers": [{"service": s, "status": "running", "oom_killed": False, "health": "healthy"}
                                  for s in ["catalyst", "db", "dns"]],
                  "research": {"Status": {"TickAt": at, "ResearchAt": at, "Prices": {"ETHUSDT": at},
                                           "Error": "", "SourceGaps": []},
                               "Report": {"Sources": [{"source_id": "primary", "at": at, "status": "ok"}]}},
                  "ledger": {"halted": False, "attention": ""}, "budget_slots": 12,
                  "disk_free_bytes": 10 * 1024 ** 3, "last_backup_age_seconds": 60}
        self.assertEqual([], observer.problems(sample, policy, now))
        for field, value, expected in [("Error", "invalid action, symbol or reason", "research_error"),
                                        ("ResearchAt", (now - dt.timedelta(hours=1)).isoformat(), "research_stale"),
                                        ("SourceGaps", ["primary"], "source_gap")]:
            broken = copy.deepcopy(sample)
            broken["research"]["Status"][field] = value
            self.assertIn(expected, observer.problems(broken, policy, now))
        sample["budget_slots"] = 13
        sample["last_backup_age_seconds"] = None
        self.assertIn("ai_budget_exceeded", observer.problems(sample, policy, now))
        self.assertIn("backup_missing_or_stale", observer.problems(sample, policy, now))
        sample["research"]["Report"]["Reviews"] = [{"At": at, "Valid": False}]
        self.assertIn("latest_review_invalid", observer.problems(sample, policy, now))


class BackupTests(unittest.TestCase):
    # The VPS backup is a bash script driving a shebang docker stub; Windows has neither.
    @unittest.skipIf(os.name == "nt", "catalyst-backup.sh runs on the Linux VPS only")
    def test_dump_failure_restarts_only_worker_and_publishes_no_backup(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            repo = root / "repo"
            (repo / "deploy").mkdir(parents=True)
            archive = root / "archive"
            archive.mkdir()
            shutil.copy(ROOT / "catalyst-backup.sh", repo / "deploy/catalyst-backup.sh")
            binary = root / "bin"
            binary.mkdir()
            docker = binary / "docker"
            docker.write_text("""#!/usr/bin/env python3
import os,sys,pathlib,json
root=pathlib.Path(os.environ['FIXTURE_ROOT'])
args=sys.argv[1:]
with (root/'calls').open('a') as f: f.write(json.dumps(args)+'\\n')
if args[0]=='compose' and 'ps' in args: print('fixture-p3-worker')
elif args[0]=='inspect':
 if 'State.Running' in args[2]: print('false' if (root/'stopped').exists() else 'true')
 else: print(root/'archive')
elif args[0]=='stop': (root/'stopped').touch()
elif args[0]=='start': (root/'stopped').unlink()
elif 'pg_dump' in args: sys.exit(42)
else: sys.exit(99)
""")
            docker.chmod(0o700)
            env = dict(os.environ, PATH=str(binary) + os.pathsep + os.environ["PATH"], FIXTURE_ROOT=str(root))
            result = subprocess.run(["bash", str(repo / "deploy/catalyst-backup.sh")], env=env,
                                    capture_output=True, text=True, timeout=10)
            self.assertEqual(42, result.returncode)
            self.assertFalse((root / "stopped").exists(), "failed backup must restore the prior running state")
            self.assertEqual([], list((root / "backups").glob("backup-*")))
            calls = [json.loads(line) for line in (root / "calls").read_text().splitlines()]
            self.assertEqual([["start", "fixture-p3-worker"]], [c for c in calls if c[0] == "start"])
            self.assertEqual([], list((root / "backups").glob(".incomplete-*")))


if __name__ == "__main__":
    unittest.main()
