"""Isolated local ETH paper experiment; never loads project .env or API keys.

start builds a frozen binary/config and prepares its own TimescaleDB container.
status/stop operate only this experiment. Runtime files and credentials are ignored.
"""
import argparse
import datetime as dt
import hashlib
import json
import os
import pathlib
import secrets
import shutil
import subprocess
import sys
import time

ROOT = pathlib.Path(__file__).resolve().parents[1]
STATE = ROOT / 'var/paper-eth'
UNIT = 'trading-bot-eth-paper'
CONTAINER = 'trading-bot-eth-paper-db'
DATABASE = 'ethpaper_trial'
ENV = {k: v for k, v in os.environ.items() if not k.startswith('TB_')}


def run(args, **kwargs):
    return subprocess.run(args, check=True, cwd=ROOT, env=ENV, **kwargs)


def worker():
    ensure_local_allowed()
    # No exchange credentials and no inherited TB_ overrides. Paper is frozen.
    env = dict(ENV)
    env['TB_DATABASE__URL'] = (STATE / 'database.url').read_text().strip()
    binary = str(STATE / 'bot')
    os.execve(binary, [binary, '-config', str(STATE / 'config.yaml'),
                       '-env-file', '/dev/null', 'run'], env)


def stopped():
    result = os.environ.get('SERVICE_RESULT', 'unknown')
    (STATE / 'last-stop.json').write_text(json.dumps({
        'at': dt.datetime.now(dt.timezone.utc).isoformat(), 'service_result': result
    }, indent=2) + '\n')
    if shutil.which('notify-send'):
        subprocess.run(['notify-send', 'ETH paper trading berhenti',
                        f'Status: {result}. Cek python scripts/eth-paper.py status'],
                       env=ENV, check=False)


def start():
    ensure_local_allowed()
    if (STATE / 'manifest.json').exists():
        raise SystemExit('Experiment already initialized; use systemctl --user start '
                         f'{UNIT}.service to resume. Do not reset its balance.')
    STATE.mkdir(parents=True, exist_ok=True, mode=0o700)
    password_path = STATE / 'database.password'
    if not password_path.exists():
        password_path.write_text(secrets.token_hex(24))
        password_path.chmod(0o600)
    password = password_path.read_text().strip()
    url = STATE / 'database.url'
    url.write_text(f'postgres://ethpaper:{password}@127.0.0.1:55434/{DATABASE}?sslmode=disable\n')
    url.chmod(0o600)
    shutil.copyfile(ROOT / 'configs/paper.eth.yaml', STATE / 'config.yaml')
    run(['go', 'build', '-trimpath', '-o', str(STATE / 'bot'), './cmd/bot'])
    run(['go', 'build', '-trimpath', '-o', str(STATE / 'datafetch'), './cmd/datafetch'])
    existing = subprocess.run(['podman', 'container', 'exists', CONTAINER], env=ENV)
    if existing.returncode == 1:
        db_env = dict(ENV, POSTGRES_PASSWORD=password)
        subprocess.run(['podman', 'run', '-d', '--name', CONTAINER,
                        '--memory', '768m', '--cpus', '1',
                        '-p', '127.0.0.1:55434:5432',
                        '-v', 'trading-bot-eth-paper-data:/var/lib/postgresql/data',
                        '-e', 'POSTGRES_USER=ethpaper', '-e', 'POSTGRES_DB=ethpaper',
                        '-e', 'POSTGRES_PASSWORD', '-e', 'TIMESCALEDB_TELEMETRY=off',
                        'docker.io/timescale/timescaledb:2.30.2-pg17'],
                       env=db_env, check=True)
    elif existing.returncode == 0:
        run(['podman', 'start', CONTAINER])
    else:
        raise SystemExit('Cannot inspect paper database container')
    for _ in range(30):
        ready = subprocess.run(['podman', 'exec', CONTAINER, 'pg_isready',
                                '-U', 'ethpaper', '-d', 'ethpaper'],
                               env=ENV, stdout=subprocess.DEVNULL)
        if ready.returncode == 0:
            break
        time.sleep(1)
    else:
        raise SystemExit('Paper database did not become ready')
    exists = run(['podman', 'exec', CONTAINER, 'psql', '-U', 'ethpaper', '-d', 'ethpaper',
                  '-Atc', f"SELECT 1 FROM pg_database WHERE datname='{DATABASE}'"],
                 capture_output=True, text=True)
    if exists.stdout.strip() != '1':
        run(['podman', 'exec', CONTAINER, 'createdb', '-U', 'ethpaper', DATABASE])
    bot_env = dict(ENV, TB_DATABASE__URL=url.read_text().strip())
    common = ['-config', str(STATE / 'config.yaml'), '-env-file', '/dev/null']
    subprocess.run([str(STATE / 'bot'), *common, 'migrate', 'up'], env=bot_env, check=True)
    # Prefill before session creation: past bars warm indicators, not past fills.
    since = (dt.datetime.now(dt.timezone.utc) - dt.timedelta(days=60)).date().isoformat()
    subprocess.run([str(STATE / 'datafetch'), *common, '-market', 'spot',
                    '-symbols', 'ETHUSDT', '-tf', '4h', '-from', since,
                    '-workers', '1'], env=bot_env, check=True)
    launch()
    manifest = {
        'started_at': dt.datetime.now(dt.timezone.utc).isoformat(),
        'mode': 'paper', 'live_authorized': False, 'symbol': 'ETHUSDT',
        'capital_idr': 2000000, 'reference_usdidr': 18052,
        'initial_usdt': 2000000 / 18052, 'fee_per_side': 0.001,
        'slippage_bps': 2, 'unit': UNIT, 'database_container': CONTAINER,
        'dashboard': 'http://127.0.0.1:18080',
        'review': 'First operational review after 7 days; strategy evidence needs longer and sufficient trades.',
        'sha256': {name: hashlib.sha256((STATE / name).read_bytes()).hexdigest()
                   for name in ('bot', 'config.yaml')},
        'persistence': 'User systemd transient service survives this chat; laptop must stay awake. '
                       'After reboot use python scripts/eth-paper.py resume; no automatic reboot setup.',
    }
    (STATE / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(json.dumps(manifest, indent=2))


def launch():
    script = str(pathlib.Path(__file__).resolve())
    run(['systemd-run', '--user', '--unit', UNIT,
         '--description', 'ETHUSDT prospective PAPER only, Rp2m reference',
         '--property', f'WorkingDirectory={ROOT}',
         '--property', 'CPUQuota=50%', '--property', 'MemoryMax=512M',
         '--property', 'Nice=10', '--property', 'TimeoutStopSec=30',
         '--property', f'ExecStopPost={sys.executable} {script} stopped',
         sys.executable, script, 'worker'])


def resume():
    ensure_local_allowed()
    manifest = json.loads((STATE / 'manifest.json').read_text())
    for name, expected in manifest['sha256'].items():
        if hashlib.sha256((STATE / name).read_bytes()).hexdigest() != expected:
            raise SystemExit(f'{name} changed; review before resuming the frozen experiment')
    run(['podman', 'start', CONTAINER])
    for _ in range(30):
        ready = subprocess.run(['podman', 'exec', CONTAINER, 'pg_isready',
                                '-U', 'ethpaper', '-d', DATABASE],
                               env=ENV, stdout=subprocess.DEVNULL)
        if ready.returncode == 0:
            break
        time.sleep(1)
    else:
        raise SystemExit('Paper database did not become ready; not starting the bot')
    check = subprocess.run(['systemctl', '--user', 'is-active', '--quiet', UNIT], env=ENV)
    if check.returncode == 0:
        print('Paper service already running; not starting another instance.')
        return
    # Loaded transient units can be restarted; after reboot recreate the unit.
    loaded = run(['systemctl', '--user', 'show', UNIT, '-p', 'LoadState', '--value'],
                 capture_output=True, text=True)
    if loaded.stdout.strip() == 'loaded':
        run(['systemctl', '--user', 'start', UNIT])
    else:
        launch()


def ensure_local_allowed():
    if (STATE / 'disabled-by-user.json').exists():
        raise SystemExit('Local ETH paper disabled at user request. Do not resume on laptop. '
                         'See docs/eth-paper-vps-reference.md for the VPS plan.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('action', choices=['start', 'resume', 'worker', 'stopped', 'status', 'stop'])
    action = parser.parse_args().action
    if action == 'start':
        start()
    elif action == 'resume':
        resume()
    elif action == 'worker':
        worker()
    elif action == 'stopped':
        stopped()
    elif action == 'stop':
        run(['systemctl', '--user', 'stop', UNIT + '.service'])
    else:
        subprocess.run(['systemctl', '--user', 'status', UNIT + '.service', '--no-pager'], env=ENV)
        run(['journalctl', '--user', '-u', UNIT + '.service', '-n', '15', '--no-pager'])
