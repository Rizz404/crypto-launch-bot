import importlib.util
import pathlib
import tempfile
import unittest

spec=importlib.util.spec_from_file_location('backtest_research_queue',pathlib.Path(__file__).with_name('queue.py'))
queue=importlib.util.module_from_spec(spec)
spec.loader.exec_module(queue)

class ResearchQueueTests(unittest.TestCase):
 def test_frozen_grid_stays_bounded(self):
  configs=queue.configurations()
  self.assertEqual(len(configs),216)
  self.assertEqual(len({c['id'] for c in configs}),216)
  self.assertEqual({c['family'] for c in configs},{'ema','breakout','momentum','pullback'})
  for c in configs:
   self.assertEqual(c['risk']['MaxLeverage'],1)
   self.assertLessEqual(c['risk']['MaxPositionPct'],.95)
   self.assertLessEqual(c['risk']['RiskPerTrade'],.05)
 def test_holdout_does_not_influence_selection(self):
  c=queue.configurations()[0];plan={'configs':[c],'selection':{}}
  rows=[{'Job':{'Candidate':c['id'],'Stage':'training'},'ConservativeReturn':.1,'Metrics':{'Trades':2,'MaxDrawdown':.1}} for _ in range(10)]
  first=queue.choose(plan,rows)['selected']
  rows.append({'Job':{'Candidate':c['id'],'Stage':'validation'},'ConservativeReturn':1000,'Metrics':{'Trades':200,'MaxDrawdown':0}})
  self.assertEqual(first,queue.choose(plan,rows)['selected'])
  self.assertEqual(len(first),1)
  self.assertFalse(queue.choose(plan,rows[:9])['selected'])
 def test_only_documented_missing_hour_is_allowed(self):
  h=queue.HALT_HOUR
  self.assertTrue(queue.valid_hour_step(h-3600,h+3600))
  self.assertFalse(queue.valid_hour_step(h-3600,h+7200))
  self.assertFalse(queue.valid_hour_step(h+3600,h+3600))
  self.assertFalse(queue.valid_hour_step(h+3600,h+10800))
 def test_status_is_replaced_and_readable(self):
  with tempfile.TemporaryDirectory() as d:
   p=pathlib.Path(d)/'status.json'
   queue.atomic(p,{'state':'running'})
   queue.atomic(p,{'state':'completed'})
   self.assertEqual(queue.read(p),{'state':'completed'})
   self.assertFalse(p.with_suffix('.json.tmp').exists())

if __name__=='__main__':unittest.main()
