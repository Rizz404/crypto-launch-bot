"""Summarize frozen testnet evidence without network, exchange keys or DB writes."""
import argparse
import collections
from decimal import Decimal
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DEFAULT = ROOT / 'docs/testing/testnet-stop-2026-10-20'


def summarize(directory, errors_path=None):
    files = {name: directory / name for name in ('evidence.json', 'supplement.json')}
    evidence = json.loads(files['evidence.json'].read_text(), parse_float=Decimal)
    supplement = json.loads(files['supplement.json'].read_text(), parse_float=Decimal)
    runs = [r for r in evidence['runs'] if r['mode'] == 'testnet']
    if not runs:
        raise ValueError('no testnet runs in evidence')
    last = max(runs, key=lambda r: r['id'])
    positions = [p for p in evidence['positions'] if p['run_id'] == last['id'] and p['status'] == 'open']
    owned = collections.defaultdict(Decimal)
    for p in positions:
        owned[(p['strategy'], p['symbol'])] += p['quantity']
    net = collections.defaultdict(Decimal)
    for f in supplement['fills_qty']:
        net[f['strategy']] += f['qty'] * (1 if f['side'] == 'buy' else -1)
    cashflows = collections.defaultdict(Decimal)
    for f in evidence['fills']:
        cashflows[f['strategy']] += f['notional'] * (1 if f['side'] == 'sell' else -1)
    closed = {t['strategy']: t['net'] for t in evidence['trades']}
    prices = {p['symbol']: p['close'] for p in evidence['last_prices']}
    symbols = {f['strategy']: f['symbol'] for f in evidence['fills']}
    corrections = collections.defaultdict(Decimal)
    for error in supplement['errors']:
        if error['msg'] == 'position closed outside the bot (manual, liquidation or missed fill)':
            corrections[error['symbol']] += Decimal(error['qty'])
    strategies = []
    for strategy in sorted(cashflows):
        symbol = symbols[strategy]
        qty = owned[(strategy, symbol)]
        strategies.append({
            'strategy': strategy, 'symbol': symbol,
            'fill_net_quantity': net[strategy], 'latest_run_quantity': qty,
            'quantity_gap': net[strategy] - qty,
            'closed_trade_net': closed.get(strategy, Decimal(0)),
            'fill_cashflow': cashflows[strategy],
            'cashflow_plus_recorded_inventory_at_last_close': cashflows[strategy] + qty * prices[symbol],
        })
    equity = sorted(supplement['equity'], key=lambda r: r['ts'])
    if not equity:
        raise ValueError('no equity snapshots in evidence')
    # Plan 1 equity exchange names contain the account suffix ':spot'.
    first_balance = equity[0]['balance']
    final_balance = Decimal(last['summary']['balance'])
    trade_net = sum(closed.values(), Decimal(0))
    cleanup = evidence['exchange_cleanup']
    result = {
        'source_sha256': {n: hashlib.sha256(p.read_bytes()).hexdigest() for n, p in files.items()},
        'latest_run_id': last['id'], 'strategies': strategies,
        'external_close_notice_quantities': dict(corrections),
        'first_recorded_balance': first_balance,
        'final_reported_balance': final_balance,
        'closed_trade_net_total': trade_net,
        'reported_balance_change_since_first_snapshot': final_balance - first_balance,
        'trade_net_minus_reported_balance_change': trade_net - (final_balance - first_balance),
        'terminal_cleanup': {
            'owned_orders_before': cleanup['before_owned'],
            'cancelled_orders': len(cleanup['cancelled']),
            'remaining_owned_orders': len(cleanup['remaining_owned']),
            'inventory_liquidated': cleanup['inventory_liquidated'],
        },
        'limitations': [
            'Amounts are exact decimal arithmetic over saved aggregates, not a re-fetch of exchange trades.',
            'Aggregate fee fields are not grouped by fee asset; this evidence records zero testnet fees.',
            'Net fill quantity is before any base-asset fee or external transfer; neither may be assumed zero on live.',
            'Last candle closes differ from intra-minute marks in the final engine snapshot.',
            'Original aggregate errors were redacted; optional code-only log evidence can supplement classification.',
            'Historical snapshots are not rewritten; this report is an audit, not corrected financial statements.',
        ],
    }

    if errors_path is not None:
        errors = json.loads(errors_path.read_text())
        counts = collections.Counter((r['msg'], r['api_code'], r['reason_class']) for r in errors['events'])
        result['source_sha256'][errors_path.name] = hashlib.sha256(errors_path.read_bytes()).hexdigest()
        result['exchange_error_classes'] = [
            {'message': message, 'api_code': code, 'reason_class': reason, 'count': n}
            for (message, code, reason), n in sorted(counts.items())
        ]
    return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--evidence-dir', type=Path, default=DEFAULT)
    parser.add_argument('--exchange-errors', type=Path, help='optional code-only log classification JSON')
    args = parser.parse_args()
    print(json.dumps(summarize(args.evidence_dir, args.exchange_errors), indent=2, default=str))
