"""Fixed follow-up of high-return candidates; one background worker, no new parameter search.
Candidate selection uses prior training results only. All annual periods replay at 1m.
A passed research gate means eligibility for prospective paper review, never live approval.
"""
import argparse, collections, concurrent.futures, csv, datetime as dt, fcntl, hashlib
import importlib.util, io, json, os, pathlib, shutil, statistics, subprocess, sys, zipfile

core_path=pathlib.Path(__file__).with_name('queue_core.py')
if not core_path.exists():core_path=pathlib.Path(__file__).with_name('queue.py')
spec=importlib.util.spec_from_file_location('research_queue_core',core_path)
core=importlib.util.module_from_spec(spec);spec.loader.exec_module(core)
SYMBOLS=core.SYMBOLS

def shortlist(plan,rows):
 ranked=[]
 for c in plan['configs']:
  if c['profile']!='aggressive':continue
  cases=[r for r in rows if r['Job']['Stage']=='training' and r['Job']['Candidate']==c['id']]
  if len(cases)!=10 or sum(r['Metrics']['Trades'] for r in cases)<10:continue
  ranked.append({'signal':c['id'].removesuffix('-aggressive'),'training_mean_return':statistics.mean(r['ConservativeReturn'] for r in cases),'training_worst_drawdown':max(r['Metrics']['MaxDrawdown'] for r in cases)})
 ranked.sort(key=lambda r:(-r['training_mean_return'],r['signal']))
 chosen=[]
 for row in ranked[:6]:
  for profile in ['larger','aggressive']:
   chosen.append(next(c for c in plan['configs'] if c['id']==row['signal']+'-'+profile))
 if len(chosen)!=12:raise RuntimeError('expected six complete pairs of risk profiles')
 return chosen,ranked

def init(parent):
 parent=parent.resolve();old=core.read(parent/'plan.json')
 rows=[core.read(f) for f in sorted((parent/'results').glob('training-*.json')) if not f.name.endswith('.failed.json')]
 chosen,ranking=shortlist(old,rows)
 project=pathlib.Path(old['project']);run=project/'var/backtest-research'/('validation-'+dt.datetime.now(core.UTC).strftime('%Y%m%dT%H%M%SZ'))
 run.mkdir(parents=True,exist_ok=False)
 for name in ['bin','jobs','results']: (run/name).mkdir()
 shutil.copy2(project/'bin/backtest-research',run/'bin/backtest')
 shutil.copy2(pathlib.Path(__file__),run/'worker.py');shutil.copy2(core_path,run/'queue_core.py')
 periods=[{'id':'training_replay','from':'2023-10-19','to':'2024-10-19','role':'Training-year execution-model check; does not provide independent validation.'},{'id':'validation','from':'2024-10-19','to':'2025-10-19','role':'Later validation of the fixed aggressive candidates; overall market period previously seen.'},{'id':'confirmation','from':'2025-10-19','to':'2026-10-19','role':'Retest on previously inspected recent year; not pristine holdout.'}]
 plan={'version':1,'kind':'fixed_high_return_validation','created_utc':core.now(),'project':str(project),'run_dir':str(run),'parent_run':str(parent),'parent_plan_sha256':core.sha(parent/'plan.json'),'data_dir':old['data_dir'],'symbols':SYMBOLS,'capital_idr_per_case':2000000,'usd_idr_fixed':18052,'configs':chosen,'ranking_from_training_only':ranking,'selection_rule':'Six distinct signals with highest mean return across 10 prior aggressive training cases; both larger/aggressive profiles, without parameter changes. No validation/confirmation results used for selection.','periods':periods,'costs':old['costs'],'resources':old['resources'],'max_jobs':360,'minute_input_sha256':old['minute_input_sha256'],'binary_sha256':core.sha(run/'bin/backtest'),'worker_sha256':core.sha(run/'worker.py'),'core_sha256':core.sha(run/'queue_core.py'),'gates':{'research_only':True,'minimum_total_closed_trades_on_two_validation_years':50,'minimum_positive_coin_years_out_of_10':7,'minimum_normal_mean_return_each_validation_year':0,'minimum_stress_mean_return_each_validation_year':0,'maximum_individual_validation_drawdown':.25,'maximum_individual_validation_loss':.20,'target_gate_requires_mean_each_validation_year_at_least':.50,'live_authorized':False,'prospective_paper_required':True},'limitations':['This follows a 216-configuration search; no statistical certainty from selecting the historical winner.','Three yearly replays include a training year and two already-inspected market periods; no claim of pristine holdout.','Five correlated coins are not ten independent experiments; averages are across independently funded Rp2m cases, not one jointly funded portfolio.','Current instrument filters, fixed USD/IDR conversion and no tax, latency or order-book queue model.','Risk profiles can hold 75%/95% of equity and risk 3%/5% per trade; simulated risk is not user approval for live risk.','Cold-start warmup is inside each year; risk halts remain active with no automatic reset.','Terminal inventory estimated exit costs are deducted; drawdown metrics remain original engine intraperiod values.','Research gates are explicit screening rules, not proof or a guarantee; live approval cannot be granted by this worker.'],'sources':old['sources']}
 core.atomic(run/'plan.json',plan);(run/'plan.sha256').write_text(core.sha(run/'plan.json')+'\n')
 core.atomic(run/'status.json',{'state':'prepared','updated_utc':core.now(),'completed_jobs':0,'failed_jobs':0,'max_jobs':360})
 core.atomic(project/'docs/testing/high-return-validation-plan.json',plan)
 core.log(str(run));return run

def prepare(plan,status):
 data=pathlib.Path(plan['data_dir']);cache=data/'minute-archives';cache.mkdir(exist_ok=True)
 status('preparing_data',detail='Verify minute inputs and download 2023-10..2025-10 archives, two concurrent downloads')
 for name,digest in plan['minute_input_sha256'].items():
  if core.sha(data/name)!=digest:raise RuntimeError('changed input '+name)
 ready=data/'minute-2023-2025-manifest.json'
 if ready.exists():
  m=core.read(ready)
  if all((data/name).is_file() and core.sha(data/name)==digest for name,digest in m['normalized_sha256'].items()):return
 periods=[f'{y}-{m:02}' for y in range(2023,2026) for m in range(1,13) if '2023-10'<=f'{y}-{m:02}'<='2025-10']
 jobs=[(s,p) for s in SYMBOLS for p in periods]
 def download(job):
  symbol,period=job;name=f'{symbol}-1m-{period}.zip';url=f'https://data.binance.vision/data/spot/monthly/klines/{symbol}/1m/{name}';path=cache/name
  expected=core.fetch(url+'.CHECKSUM').decode().split()[0]
  # Reuse earlier local downloads when present; never trust them without checksum verification.
  if not path.exists():
   old=pathlib.Path('/tmp/trading-bot-year')/name
   if old.exists() and core.sha(old)==expected:shutil.copy2(old,path)
  if not path.exists() or core.sha(path)!=expected:
   b=core.fetch(url)
   if hashlib.sha256(b).hexdigest()!=expected:raise RuntimeError('checksum mismatch '+name)
   tmp=path.with_suffix('.tmp');tmp.write_bytes(b);os.replace(tmp,path)
  with zipfile.ZipFile(path) as z:
   if z.testzip():raise RuntimeError('ZIP corruption '+name)
  return {'file':name,'url':url,'sha256':expected}
 archives=[]
 with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
  for n,a in enumerate(pool.map(download,jobs),1):archives.append(a);status('preparing_data',archives_verified=n,archives_total=len(jobs))
 start,end=core.timestamp('2023-10-19'),core.timestamp('2025-10-19');hashes={};counts={}
 for symbol in SYMBOLS:
  path=data/(symbol+'-minute-2023-2025.csv');tmp=path.with_suffix('.tmp');previous=None;count=0
  with tmp.open('w') as out:
   w=csv.writer(out)
   for period in periods:
    with zipfile.ZipFile(cache/f'{symbol}-1m-{period}.zip') as z:
     with z.open(z.namelist()[0]) as f:
      for row in csv.reader(io.TextIOWrapper(f)):
       n=int(row[0]);sec=n//(1_000_000 if n>10**14 else 1000)
       if not start<=sec<end:continue
       if previous is not None and sec!=previous+60:raise RuntimeError(f'unexplained minute gap/duplicate {symbol}: {previous} {sec}')
       if count==0 and sec!=start:raise RuntimeError('missing initial candle '+symbol)
       previous=sec;count+=1;w.writerow([sec]+row[1:6]+[row[7],row[8]])
  if count!=(end-start)//60 or previous!=end-60:raise RuntimeError('incomplete minute history '+symbol)
  os.replace(tmp,path);hashes[path.name]=core.sha(path);counts[symbol]=count
 core.atomic(ready,{'from':'2023-10-19','to_exclusive':'2025-10-19','created_utc':core.now(),'archives':archives,'normalized_sha256':hashes,'counts':counts})
 core.log('Minute history verified without gaps')

def jobs(plan):
 for c in plan['configs']:
  for period in plan['periods']:
   for stress in [False,True]:
    stage=period['id']+('_stress' if stress else '')
    for symbol in SYMBOLS:
     path=pathlib.Path(plan['data_dir'])/(symbol+'.csv' if period['id']=='confirmation' else symbol+'-minute-2023-2025.csv')
     yield {'ID':f"{stage}-{c['id']}-{symbol}",'Candidate':c['id'],'Family':c['family'],'Stage':stage,'Symbol':symbol,'From':period['from'],'To':period['to'],'InputTF':'1m','InputFile':str(path),'RulesFile':str(pathlib.Path(plan['data_dir'])/'instruments.json'),'Name':c['name'],'Params':c['params'],'Risk':c['risk'],'Fee':plan['costs']['stress_fee' if stress else 'normal_fee'],'SlippageBps':plan['costs']['stress_slippage_bps' if stress else 'normal_slippage_bps']}

def assess(plan,rows):
 gates=plan['gates'];groups=[]
 for c in plan['configs']:
  rs=[r for r in rows if r['Job']['Candidate']==c['id']];stages={}
  for stage in ['training_replay','training_replay_stress','validation','validation_stress','confirmation','confirmation_stress']:
   ss=[r for r in rs if r['Job']['Stage']==stage]
   if ss:stages[stage]={'cases':len(ss),'mean_return':statistics.mean(r['ConservativeReturn'] for r in ss),'positive_cases':sum(r['ConservativeReturn']>0 for r in ss),'target_hits':sum(r['ConservativeReturn']>=.50 for r in ss),'worst_drawdown':max(r['Metrics']['MaxDrawdown'] for r in ss),'worst_return':min(r['ConservativeReturn'] for r in ss),'closed_trades':sum(r['Metrics']['Trades'] for r in ss),'per_coin':{r['Job']['Symbol']:{'return_pct':r['ConservativeReturn']*100,'final_idr':r['ConservativeFinalIDR'],'max_drawdown_pct':r['Metrics']['MaxDrawdown']*100,'closed_trades':r['Metrics']['Trades']} for r in ss}}
  required=['validation','validation_stress','confirmation','confirmation_stress'];checks={}
  checks['complete']=len(rs)==30 and all(stages.get(k,{}).get('cases')==5 for k in required)
  if checks['complete']:
   normal=[stages['validation'],stages['confirmation']];stress=[stages['validation_stress'],stages['confirmation_stress']]
   checks.update(positive_each_year=all(x['mean_return']>gates['minimum_normal_mean_return_each_validation_year'] for x in normal),positive_stress_each_year=all(x['mean_return']>gates['minimum_stress_mean_return_each_validation_year'] for x in stress),positive_coin_years=sum(x['positive_cases'] for x in normal)>=gates['minimum_positive_coin_years_out_of_10'],enough_trades=sum(x['closed_trades'] for x in normal)>=gates['minimum_total_closed_trades_on_two_validation_years'],drawdown=max(x['worst_drawdown'] for x in normal+stress)<=gates['maximum_individual_validation_drawdown'],worst_loss=min(x['worst_return'] for x in normal+stress)>=-gates['maximum_individual_validation_loss'])
  passed=checks.get('complete',False) and all(checks.values())
  target=passed and all(stages[k]['mean_return']>=gates['target_gate_requires_mean_each_validation_year_at_least'] for k in ['validation','confirmation'])
  groups.append({'candidate':c['id'],'profile':c['profile'],'params':c['params'],'stages':stages,'checks':checks,'eligible_for_prospective_paper_review':passed,'meets_historical_50pct_research_target':target,'live_authorized':False})
 return groups

def write_report(run,plan,rows,failures):
 groups=assess(plan,rows);out={'created_utc':core.now(),'successful_jobs':len(rows),'failed_jobs':len(failures),'candidate_count':12,'capital_idr_per_case':2000000,'candidates':groups,'paper_review_eligible':sum(g['eligible_for_prospective_paper_review'] for g in groups),'historical_target_passes':sum(g['meets_historical_50pct_research_target'] for g in groups),'live_authorized':False,'gates':plan['gates'],'limitations':plan['limitations'],'failures':failures}
 core.atomic(run/'report.json',out)
 lines=['VALIDASI KANDIDAT AGRESIF SELESAI',f"{len(rows)}/360 pekerjaan berhasil; {len(failures)} gagal. Modal setiap kasus Rp2 juta.",f"Lolos saringan untuk peninjauan paper: {out['paper_review_eligible']}/12. Lolos target historis +50% pada rata-rata kedua tahun validasi: {out['historical_target_passes']}/12.",'Lolos saringan ini tidak berarti diizinkan live. Hasil gagal juga dipertahankan, tanpa tuning ulang.','']
 for g in groups:
  lines.append(g['candidate']+f" | paper review: {g['eligible_for_prospective_paper_review']} | target +50%: {g['meets_historical_50pct_research_target']}")
  for stage,s in g['stages'].items():lines.append(f"  {stage}: rata-rata {s['mean_return']*100:+.2f}%; target pada {s['target_hits']}/{s['cases']} koin; drawdown terburuk {s['worst_drawdown']*100:.2f}%; transaksi ditutup {s['closed_trades']}")
  lines.append('  Pemeriksaan: '+json.dumps(g['checks']))
 lines+=['','Batasan:']+['- '+x for x in plan['limitations']]
 (run/'report.txt').write_text('\n'.join(lines)+'\n')

def worker(run):
 plan=core.read(run/'plan.json')
 for path,key in [('bin/backtest','binary_sha256'),('worker.py','worker_sha256'),('queue_core.py','core_sha256')]:
  if core.sha(run/path)!=plan[key]:raise RuntimeError('frozen code changed '+path)
 if core.sha(run/'plan.json')!=(run/'plan.sha256').read_text().strip():raise RuntimeError('frozen plan changed')
 lock=(run/'worker.lock').open('w');fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
 completed=0;failed=0
 def status(state,**kw):core.atomic(run/'status.json',{'state':state,'updated_utc':core.now(),'pid':os.getpid(),'completed_jobs':completed,'failed_jobs':failed,'max_jobs':360,**kw})
 try:
  prepare(plan,status);rows=[]
  for job in jobs(plan):
   path=run/'results'/(job['ID']+'.json');jobfile=run/'jobs'/(job['ID']+'.json');failure=path.with_suffix('.failed.json');core.atomic(jobfile,job)
   if path.exists():
    result=core.read(path)
    if result['Job']!=job:raise RuntimeError('checkpoint mismatch '+job['ID'])
    rows.append(result);completed+=1;continue
   if failure.exists():failed+=1;continue
   status('running',current_job=job['ID'],stage=job['Stage']);core.log(job['ID'])
   try:
    p=subprocess.run([str(run/'bin/backtest'),'-job',str(jobfile),'-result',str(path)],capture_output=True,text=True,timeout=240,env={**os.environ,'GOMAXPROCS':'2','GOMEMLIMIT':'1536MiB'})
    if p.returncode:raise RuntimeError(p.stderr[-3000:])
    result=core.read(path)
    if result['Job']!=job:raise RuntimeError('result mismatch')
    rows.append(result);completed+=1
   except Exception as e:
    if path.exists():path.unlink()
    core.atomic(failure,{'job':job,'error':str(e),'at_utc':core.now()});failed+=1
  failures=[core.read(f) for f in sorted((run/'results').glob('*.failed.json'))]
  write_report(run,plan,rows,failures);state='completed_with_errors' if failures else 'completed';status(state)
  (run/'DONE.txt').write_text(state+'\n'+core.now()+'\n')
  core.notify(run,'Validasi backtest selesai',f'{completed} pekerjaan berhasil, {failed} gagal. Hasil: {run / "report.txt"}. Kabari Codex untuk analisis.')
 except Exception as e:
  status('failed',error=str(e));(run/'FAILED.txt').write_text(str(e)+'\n');core.notify(run,'Validasi backtest terhenti',str(e)[:180]);raise
 finally:lock.close()

def main():
 p=argparse.ArgumentParser();p.add_argument('command',choices=['init','launch','worker','status']);p.add_argument('--parent',type=pathlib.Path,default=pathlib.Path('var/backtest-research/latest'));p.add_argument('--run-dir',type=pathlib.Path);a=p.parse_args()
 if a.command=='init':init(a.parent);return
 if a.run_dir is None:p.error('--run-dir required')
 run=a.run_dir.resolve()
 if a.command=='launch':core.launch(run)
 elif a.command=='worker':worker(run)
 else:print(json.dumps(core.read(run/'status.json'),indent=2))
if __name__=='__main__':main()
