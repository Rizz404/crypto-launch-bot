"""Exercise the observer's complete evaluation with isolated external boundaries."""
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch

SOURCE = (Path(__file__).parent / 'vps-observe.py').read_text()


class ResearchObserverTests(unittest.TestCase):
    def evaluate(self, broken=None):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            services = ['bot', 'research', 'db', 'dns', 'prometheus', 'grafana']
            rows = [{'service': s, 'id': s, 'status': 'running', 'oom_killed': False, 'restarts': 0} for s in services]
            data = {'series': [{'symbol': symbol, 'timeframe': tf, 'age_seconds': 60, 'tf_seconds': seconds, 'gaps': 0}
                               for symbol in ['BTCUSDT', 'ETHUSDT']
                               for tf, seconds in [('1m', 60), ('15m', 900), ('1h', 3600), ('4h', 14400)]],
                    'managed': [{'attention': '', 'halted': False}]}
            def database(exchange, name=None):
                if name and broken == 'database':
                    raise RuntimeError('unavailable')
                result = json.loads(json.dumps(data))
                if name and broken == 'attention':
                    result['managed'][0]['attention'] = 'reconcile required'
                if name and broken == 'missing':
                    result['series'].pop()
                if name and broken == 'gap':
                    result['series'][0]['gaps'] = 1
                return result
            def http(url):
                if ':19102/' in url and broken == 'health':
                    raise RuntimeError('unhealthy')
                if url.endswith('/metrics'):
                    return 'tradingbot_build_info{mode="paper"} 1'
                if url.endswith('/targets'):
                    return json.dumps({'data': {'activeTargets': [{'health': 'up'}, {'health': 'up'}]}})
                if url.endswith('/api/health'):
                    return '{}'
                return 'ok'
            scope = {'__file__': str(root / 'deploy/vps-observe.py')}
            # Keep the real attempt/error handling and evidence writer; replace only IO.
            exec(SOURCE[:SOURCE.index('\nnow = dt.datetime.now')], scope)
            scope.update(REPO=root, EVIDENCE=root / 'evidence', database=database, http=http,
                         containers=lambda: rows, env_value=lambda name, default: default,
                         port=lambda name, default: int(default))
            with patch('builtins.print', Mock()):
                with self.assertRaises(SystemExit) as caught:
                    exec(SOURCE[SOURCE.index('\nnow = dt.datetime.now'):], scope)
            evidence = json.loads(next((root / 'evidence').glob('samples-*')).read_text())
            self.assertEqual(bool(evidence['problems']), bool(caught.exception.code))
            return evidence['problems']

    def test_healthy_cohort(self):
        self.assertEqual([], self.evaluate())

    def test_research_failure_is_not_hidden_by_healthy_p2(self):
        for broken, problem in [('database', 'research_database'), ('health', 'research_health'),
                                ('attention', 'research_managed_attention'), ('missing', 'research_missing_series'),
                                ('gap', 'research_candle_freshness_or_gap')]:
            with self.subTest(broken=broken):
                self.assertIn(problem, self.evaluate(broken))
