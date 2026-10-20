import importlib.util,pathlib,unittest,copy
spec=importlib.util.spec_from_file_location('fixed_validation',pathlib.Path(__file__).with_name('validation.py'))
v=importlib.util.module_from_spec(spec);spec.loader.exec_module(v)

class FixedValidationTests(unittest.TestCase):
 def test_selection_cannot_use_later_results(self):
  configs=v.core.configurations();rows=[]
  for c in configs:
   if c['profile']!='aggressive':continue
   for i in range(10):rows.append({'Job':{'Candidate':c['id'],'Stage':'training'},'ConservativeReturn':int(c['id'].split('-')[1])/1000,'Metrics':{'Trades':2,'MaxDrawdown':.1}})
  selected,_=v.shortlist({'configs':configs},rows)
  rows.append({'Job':{'Candidate':configs[2]['id'],'Stage':'confirmation'},'ConservativeReturn':999,'Metrics':{'Trades':100,'MaxDrawdown':0}})
  second,_=v.shortlist({'configs':configs},rows)
  self.assertEqual(selected,second);self.assertEqual(len(selected),12)
  self.assertEqual({c['profile'] for c in selected},{'larger','aggressive'})
 def test_risky_or_incomplete_results_cannot_pass_gate(self):
  c=v.core.configurations()[1]
  gates={'minimum_total_closed_trades_on_two_validation_years':50,'minimum_positive_coin_years_out_of_10':7,'minimum_normal_mean_return_each_validation_year':0,'minimum_stress_mean_return_each_validation_year':0,'maximum_individual_validation_drawdown':.25,'maximum_individual_validation_loss':.20,'target_gate_requires_mean_each_validation_year_at_least':.5}
  plan={'configs':[c],'gates':gates};rows=[]
  for stage in ['training_replay','training_replay_stress','validation','validation_stress','confirmation','confirmation_stress']:
   for symbol in v.SYMBOLS:rows.append({'Job':{'Candidate':c['id'],'Stage':stage,'Symbol':symbol},'ConservativeReturn':.6,'ConservativeFinalIDR':3200000,'OpenPositions':0,'Metrics':{'Trades':10,'MaxDrawdown':.1}})
  g=v.assess(plan,rows)[0];self.assertTrue(g['eligible_for_prospective_paper_review']);self.assertTrue(g['meets_historical_50pct_research_target']);self.assertFalse(g['live_authorized'])
  bad=copy.deepcopy(rows);bad[-1]['Metrics']['MaxDrawdown']=.4
  self.assertFalse(v.assess(plan,bad)[0]['eligible_for_prospective_paper_review'])
  self.assertFalse(v.assess(plan,rows[:-1])[0]['eligible_for_prospective_paper_review'])
  low=copy.deepcopy(rows)
  for r in low:r['ConservativeReturn']=.1
  self.assertTrue(v.assess(plan,low)[0]['eligible_for_prospective_paper_review']);self.assertFalse(v.assess(plan,low)[0]['meets_historical_50pct_research_target'])
 def test_job_grid_is_one_minute_and_fixed(self):
  cs=v.core.configurations()[:12]
  p={'configs':cs,'data_dir':'/tmp/data','costs':{'normal_fee':.001,'normal_slippage_bps':2,'stress_fee':.002,'stress_slippage_bps':5},'periods':[{'id':s,'from':a,'to':b} for s,a,b in [('training_replay','2023-10-19','2024-10-19'),('validation','2024-10-19','2025-10-19'),('confirmation','2025-10-19','2026-10-19')]]}
  jobs=list(v.jobs(p));self.assertEqual(len(jobs),360);self.assertEqual(len({j['ID'] for j in jobs}),360);self.assertTrue(all(j['InputTF']=='1m' for j in jobs))

if __name__=='__main__':unittest.main()
