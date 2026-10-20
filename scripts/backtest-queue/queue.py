"""Bounded, resumable local research queue. No credentials, exchange orders or cron.
init freezes 216 candidates; launch starts one resource-limited user systemd service.
worker performs historical-data preparation, training selection and fixed validation.
"""
import argparse, concurrent.futures, csv, datetime as dt, fcntl, hashlib, io
import itertools, json, math, os, pathlib, shutil, statistics, subprocess, sys, time, urllib.request, zipfile

SYMBOLS=['BTCUSDT','ETHUSDT','BNBUSDT','XRPUSDT','SOLUSDT']
UTC=dt.timezone.utc

def now(): return dt.datetime.now(UTC).isoformat()
def sha(path):
 h=hashlib.sha256()
 with pathlib.Path(path).open('rb') as f:
  for b in iter(lambda:f.read(1024*1024),b''):h.update(b)
 return h.hexdigest()
def atomic(path,v):
 path=pathlib.Path(path);path.parent.mkdir(parents=True,exist_ok=True)
 tmp=path.with_suffix(path.suffix+'.tmp');tmp.write_text(json.dumps(v,indent=2)+'\n');os.replace(tmp,path)
def read(path): return json.loads(pathlib.Path(path).read_text())
def timestamp(s): return int(dt.datetime.fromisoformat(s).replace(tzinfo=UTC).timestamp())
def log(msg): print(now(),msg,flush=True)

# A documented Binance spot halt: keep this bar absent, never interpolate a tradable price.
HALT_HOUR=1679662800  # 2023-03-24 13:00 UTC
HALT_SOURCE='https://www.binance.com/en/support/announcement/detail/813a31506e9f478ea8c1058b425df87a'
def valid_hour_step(previous,current):
 return current==previous+3600 or (previous==HALT_HOUR-3600 and current==HALT_HOUR+3600)

def signals():
 rows=[]
 def add(family,name,params):rows.append({'family':family,'name':name,'params':params})
 for tf,(fast,slow),trend,(stop,trail) in itertools.product(['4h','1d'],[(12,26),(20,50)],[50,100],[(2,2),(3,3)]):
  add('ema','emacross',dict(timeframe=tf,fast=fast,slow=slow,trend_ema=trend,rsi_period=14,rsi_long_max=80,rsi_short_min=30,atr_period=14,stop_atr=stop,target_atr=0,trail_atr=trail,allow_short=False,exit_on_cross=True))
 for tf,entry,trend in itertools.product(['4h','1d'],[20,55],[50,100]):
  add('breakout','trendspot',dict(mode='breakout',timeframe=tf,trend_period=trend,entry_period=entry,exit_period=10 if entry==20 else 20,atr_period=14,stop_atr=3,target_atr=0,trail_atr=3))
 for tf,n,threshold,trend in itertools.product(['4h','1d'],[14,30,90],[0,0.05],[50,100]):
  entry=n*6 if tf=='4h' else n
  add('momentum','trendspot',dict(mode='momentum',timeframe=tf,trend_period=trend,entry_period=entry,exit_period=20,atr_period=14,stop_atr=3,target_atr=0,trail_atr=3,momentum_min=threshold))
 for tf,trend,entry,exit_ in itertools.product(['1h','4h'],[100,200],[35,40,45],[60,65]):
  add('pullback','trendspot',dict(mode='pullback',timeframe=tf,trend_period=trend,entry_period=55,exit_period=20,atr_period=14,rsi_period=14,rsi_entry=entry,rsi_exit=exit_,stop_atr=2.5,target_atr=5,trail_atr=0))
 assert len(rows)==72
 return rows

def configurations():
 profiles=[('baseline',0.01,0.25,0.05,0.20),('larger',0.03,0.75,0.08,0.30),('aggressive',0.05,0.95,0.10,0.35)]
 rows=[]
 for i,s in enumerate(signals(),1):
  for label,risk,pos,daily,dd in profiles:
   rows.append(dict(id=f"{s['family']}-{i:03}-{label}",**s,profile=label,risk={'RiskPerTrade':risk,'PositionPct':pos,'MaxPositionPct':pos,'MaxLeverage':1,'MaxOpenPositions':1,'MaxExposurePct':1.0,'DailyLossLimit':daily,'MaxDrawdown':dd,'CloseOnKill':True}))
 assert len(rows)==216
 return rows

def init(project,cache):
 project=project.resolve();run=project/'var/backtest-research'/('run-'+dt.datetime.now(UTC).strftime('%Y%m%dT%H%M%SZ'))
 run.mkdir(parents=True,exist_ok=False);data=project/'var/backtest-research/data';data.mkdir(parents=True,exist_ok=True)
 # Preserve the already-verified one-minute dataset outside volatile /tmp.
 hashes={}
 for name in [s+'.csv' for s in SYMBOLS]+['instruments.json','manifest.json']:
  src=cache/name;dst=data/name
  if not src.is_file():raise RuntimeError(f'missing verified input {src}; run scripts/backtest-year/download.py first')
  digest=sha(src)
  if not dst.is_file() or sha(dst)!=digest:shutil.copy2(src,dst)
  hashes[name]=digest
 manifest=read(data/'manifest.json')
 assert manifest['from']=='2025-10-19T00:00:00Z' and manifest['to_exclusive']=='2026-10-19T00:00:00Z'
 assert sum(x['candles'] for x in manifest['series'])==2628000
 plan={'version':1,'created_utc':now(),'project':str(project),'run_dir':str(run),'data_dir':str(data),'symbols':SYMBOLS,'capital_idr_per_case':2000000,'usd_idr_fixed':18052,'target_return':0.50,'configs':configurations(),'training_periods':[['2022-10-19','2023-10-19'],['2023-10-19','2024-10-19']],'validation_period':['2024-10-19','2025-10-19'],'confirmation_period':['2025-10-19','2026-10-19'],'selection':{'top_per_family':3,'score':'mean annual return across 10 training cases - 0.5 * worst max drawdown - 0.25 * standard deviation of returns','minimum_closed_trades_training_total':10,'requires_all_10_training_jobs':True,'validation_or_confirmation_used_for_selection':False},'costs':{'normal_fee':0.001,'normal_slippage_bps':2,'stress_fee':0.002,'stress_slippage_bps':5},'resources':{'workers':1,'cpu_quota':'150%','memory_max':'3G','go_soft_memory_limit':'1536MiB','nice':10},'limits':{'training_jobs':2160,'max_validation_confirmation_stress_jobs':180,'max_jobs':2340,'per_job_timeout_seconds':240,'max_network_downloads':2},'limitations':['Spot only, no leverage, no short selling; this is not a search across all possible strategies','Current instrument rules approximate past rules; currency conversion is fixed and excludes IDR FX changes and taxes','Training/validation fills use 1h OHLC; selected configurations get full 1m confirmation and increased-cost stress test','Annual runs cold-start indicators within each period; idle warmup and risk-triggered halts count in annual return','2025-2026 confirmation year was inspected previously; this is not a pristine holdout or proof of future profit','Five current large coins were selected with hindsight; survivorship/universe selection bias remains','216 variants create multiple-testing risk; all results and failures are retained, not just target hits','Aggressive profiles can lose substantial capital; a high historical return alone does not pass live approval','Final ranking is descriptive; configurations are frozen before validation and no automatic retuning occurs','Conservative terminal equity deducts estimated sale costs on remaining inventory; drawdown is the original intraperiod engine metric'],'sources':[{'url':'https://www.nber.org/papers/w24877','reason':'Historical crypto time-series momentum motivates momentum and trend hypotheses, not a promised return.'},{'url':'https://www.aqr.com/Insights/Research/Journal-Article/Time-Series-Momentum','reason':'Trend-following hypothesis; evidence across other markets is not validation of these crypto implementations.'},{'url':'https://www.davidhbailey.com/dhbpapers/backtest-prob.pdf','reason':'Multiple testing and backtest overfitting motivate fixed selection and separate periods.'},{'url':'https://data.binance.vision/','reason':'Official public spot candle archives with SHA256 checksums.'}],'minute_input_sha256':hashes}
 plan['data_halt_policy']={'hour_utc':'2023-03-24T13:00:00Z','source':HALT_SOURCE,'policy':'Retain absent hourly candle; clock-aligned higher bars use real trading candles only; all other unexplained gaps fail.'}
 binary=project/'bin/backtest-research'
 if not binary.is_file():raise RuntimeError('build bin/backtest-research before init')
 plan['binary_sha256']=sha(binary)
 plan['source_revision']=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()
 plan['code_sha256']={str(p.relative_to(project)):sha(p) for p in [project/'scripts/backtest-year/job.go',project/'scripts/backtest-year/main.go',project/'internal/strategy/trendspot/trendspot.go',project/'scripts/backtest-queue/queue.py']}
 atomic(run/'plan.json',plan)
 (run/'plan.sha256').write_text(sha(run/'plan.json')+'\n')
 shutil.copy2(pathlib.Path(__file__),run/'worker.py')
 (run/'bin').mkdir();shutil.copy2(binary,run/'bin/backtest');(run/'jobs').mkdir();(run/'results').mkdir()
 atomic(run/'status.json',{'state':'prepared','updated_utc':now(),'completed_jobs':0,'max_jobs':2340})
 log(str(run));return run

def fetch(url):
 for attempt in range(4):
  try:
   with urllib.request.urlopen(url,timeout=45) as r:return r.read()
  except Exception:
   if attempt==3:raise
   time.sleep(2**attempt)

def prepare(plan,status):
 data=pathlib.Path(plan['data_dir']);cached=data/'hourly-archives';cached.mkdir(exist_ok=True)
 status('preparing_data',detail='Verify existing minute inputs and download historical spot 1h archives')
 for name,digest in plan['minute_input_sha256'].items():
  if sha(data/name)!=digest:raise RuntimeError(f'input changed: {name}')
 ready=data/'hourly-manifest.json'
 if ready.exists():
  m=read(ready)
  if all((data/name).is_file() and sha(data/name)==h for name,h in m['normalized_sha256'].items()):return
 periods=[f'{y}-{m:02}' for y in range(2022,2026) for m in range(1,13) if '2022-10'<=f'{y}-{m:02}'<='2025-10']
 jobs=[(s,p) for s in SYMBOLS for p in periods]
 def download(job):
  s,p=job;name=f'{s}-1h-{p}.zip';url=f'https://data.binance.vision/data/spot/monthly/klines/{s}/1h/{name}';path=cached/name
  expected=fetch(url+'.CHECKSUM').decode().split()[0]
  if not path.exists() or sha(path)!=expected:
   b=fetch(url)
   if hashlib.sha256(b).hexdigest()!=expected:raise RuntimeError('checksum mismatch '+name)
   tmp=path.with_suffix('.tmp');tmp.write_bytes(b);os.replace(tmp,path)
  with zipfile.ZipFile(path) as z:
   if z.testzip():raise RuntimeError('ZIP corruption '+name)
  return {'url':url,'sha256':expected,'file':name}
 records=[]
 with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
  for n,r in enumerate(pool.map(download,jobs),1):
   records.append(r)
   status('preparing_data',archives_verified=n,archives_total=len(jobs))
 start,end=timestamp('2022-10-19'),timestamp('2025-10-19');hashes={};counts={}
 for s in SYMBOLS:
  path=data/(s+'-1h.csv');tmp=path.with_suffix('.tmp');previous=None;count=0
  with tmp.open('w') as out:
   w=csv.writer(out)
   for p in periods:
    with zipfile.ZipFile(cached/f'{s}-1h-{p}.zip') as z:
     with z.open(z.namelist()[0]) as f:
      for row in csv.reader(io.TextIOWrapper(f)):
       raw=int(row[0]);sec=raw//(1_000_000 if raw>10**14 else 1000)
       if not start<=sec<end:continue
       if previous is not None and not valid_hour_step(previous,sec):raise RuntimeError(f'hourly gap/duplicate {s}: {previous} {sec}')
       if count==0 and sec!=start:raise RuntimeError('wrong start '+s)
       previous=sec;count+=1;w.writerow([sec]+row[1:6]+[row[7],row[8]])
  expected=(end-start)//3600-int(start<=HALT_HOUR<end)
  if count!=expected or previous!=end-3600:raise RuntimeError('incomplete historical data '+s)
  os.replace(tmp,path);hashes[path.name]=sha(path);counts[s]=count
 atomic(ready,{'created_utc':now(),'from':'2022-10-19','to_exclusive':'2025-10-19','archives':records,'normalized_sha256':hashes,'counts':counts,'approved_halts':[{'hour_utc':'2023-03-24T13:00:00Z','source':HALT_SOURCE,'policy':'Missing bar retained; no synthetic prices/fills; clock-aligned aggregates use actual available trading bars.'}]})
 log('Historical data verified; documented 2023-03-24 halt retained without fabricated candles')

def make_job(plan,c,symbol,stage,index,period,stress=False):
 minute=stage in ['confirmation','stress'];data=pathlib.Path(plan['data_dir'])
 return {'ID':f"{stage}-{index}-{c['id']}-{symbol}",'Candidate':c['id'],'Family':c['family'],'Stage':stage,'Symbol':symbol,'From':period[0],'To':period[1],'InputTF':'1m' if minute else '1h','InputFile':str(data/(symbol+'.csv' if minute else symbol+'-1h.csv')),'RulesFile':str(data/'instruments.json'),'Name':c['name'],'Params':c['params'],'Risk':c['risk'],'Fee':plan['costs']['stress_fee' if stress else 'normal_fee'],'SlippageBps':plan['costs']['stress_slippage_bps' if stress else 'normal_slippage_bps']}

def choose(plan,rows):
 ranked=[]
 for c in plan['configs']:
  rs=[r for r in rows if r['Job']['Candidate']==c['id'] and r['Job']['Stage']=='training']
  if len(rs)!=10 or sum(r['Metrics']['Trades'] for r in rs)<10:continue
  returns=[r['ConservativeReturn'] for r in rs];worst=max(r['Metrics']['MaxDrawdown'] for r in rs)
  score=statistics.mean(returns)-0.5*worst-0.25*statistics.pstdev(returns)
  ranked.append({'candidate':c['id'],'family':c['family'],'score':score,'mean_return':statistics.mean(returns),'worst_drawdown':worst,'target_hits':sum(x>=0.5 for x in returns)})
 selected=[]
 for family in ['ema','breakout','momentum','pullback']:
  selected.extend(sorted([r for r in ranked if r['family']==family],key=lambda r:(-r['score'],r['candidate']))[:3])
 return {'rule':plan['selection'],'ranked_training':sorted(ranked,key=lambda r:(-r['score'],r['candidate'])),'selected':selected,'selected_before_validation':True,'selected_at_utc':now()}

def report(run,plan,rows,failures):
 selected=read(run/'selection.json') if (run/'selection.json').exists() else {'selected':[]}
 groups=[]
 for picked in selected['selected']:
  cid=picked['candidate'];c=next(c for c in plan['configs'] if c['id']==cid)
  group={'candidate':cid,'family':c['family'],'risk_profile':c['profile'],'training_score':picked['score'],'stages':{}}
  for stage in ['training','validation','confirmation','stress']:
   rs=[r for r in rows if r['Job']['Candidate']==cid and r['Job']['Stage']==stage]
   if rs:group['stages'][stage]={'cases':len(rs),'mean_return_pct':statistics.mean(r['ConservativeReturn'] for r in rs)*100,'target_50pct_cases':sum(r['ConservativeReturn']>=0.5 for r in rs),'positive_cases':sum(r['ConservativeReturn']>0 for r in rs),'worst_max_drawdown_pct':max(r['Metrics']['MaxDrawdown'] for r in rs)*100,'closed_trades':sum(r['Metrics']['Trades'] for r in rs),'per_coin':{(r['Job']['Symbol']+'@'+r['Job']['From'] if stage=='training' else r['Job']['Symbol']):{'return_pct':r['ConservativeReturn']*100,'final_idr':r['ConservativeFinalIDR'],'drawdown_pct':r['Metrics']['MaxDrawdown']*100,'closed_trades':r['Metrics']['Trades'],'open_positions':r['OpenPositions']} for r in rs}}
  groups.append(group)
 groups.sort(key=lambda g:-g['stages'].get('confirmation',{}).get('mean_return_pct',-1e9))
 hits=[{'candidate':r['Job']['Candidate'],'symbol':r['Job']['Symbol'],'stage':r['Job']['Stage'],'return_pct':r['ConservativeReturn']*100,'final_idr':r['ConservativeFinalIDR'],'max_drawdown_pct':r['Metrics']['MaxDrawdown']*100} for r in rows if r['ConservativeReturn']>=0.5]
 result={'created_utc':now(),'capital_idr_per_independent_case':2000000,'successful_jobs':len(rows),'failed_jobs':len(failures),'configurations_screened':216,'selected_configurations':len(groups),'selected_results':groups,'all_target_hits':hits,'target_hits_are_not_proof_of_future_returns':True,'failures':failures,'limitations':plan['limitations'],'sources':plan['sources']}
 atomic(run/'report.json',result)
 lines=['RISET BACKTEST SELESAI',f"Modal: Rp2 juta per kombinasi strategi/koin. {len(rows)} pekerjaan berhasil, {len(failures)} gagal.",'216 konfigurasi = 72 aturan sinyal x 3 profil ukuran posisi; satu worker, Spot tanpa leverage.','Seleksi hanya memakai 2022-10-19..2024-10-19; validasi 2024-10-19..2025-10-19; konfirmasi 2025-10-19..2026-10-19.',f"Kasus yang mencapai +50% (termasuk training): {len(hits)}. Kasus konfirmasi: {sum(x['stage']=='confirmation' for x in hits)}; stress: {sum(x['stage']=='stress' for x in hits)}.",'Angka di bawah adalah rata-rata lima akun simulasi independen, bukan hasil portofolio dengan modal bersama.','']
 for g in groups:
  lines.append(g['candidate']+' / '+g['risk_profile'])
  for stage,m in g['stages'].items():lines.append(f"  {stage}: {m['mean_return_pct']:+.2f}%; +50% pada {m['target_50pct_cases']}/{m['cases']} kasus; drawdown terburuk {m['worst_max_drawdown_pct']:.2f}%; penutupan transaksi {m['closed_trades']}")
 lines+=['','Batasan:']+['- '+s for s in plan['limitations']]+['','Detail tiap koin: report.json. Semua pekerjaan, kegagalan, plan dan pemilihan kandidat disimpan.','Tidak ada order sungguhan atau perubahan konfigurasi aktif/VPS.']
 (run/'report.txt').write_text('\n'.join(lines)+'\n')

def notify(run,title,body):
 result={'attempted_utc':now(),'title':title,'body':body}
 try:
  p=subprocess.run(['notify-send','--app-name=Trading Bot Research','--urgency=critical','--expire-time=0',title,body],capture_output=True,text=True,timeout=10)
  result.update(delivered=p.returncode==0,error=p.stderr[:500] if p.returncode else None)
 except Exception as e:result.update(delivered=False,error=str(e))
 atomic(run/'notification.json',result)

def worker(run):
 plan=read(run/'plan.json')
 if sha(run/'plan.json')!=(run/'plan.sha256').read_text().strip():raise RuntimeError('frozen plan changed')
 if sha(run/'bin/backtest')!=plan['binary_sha256']:raise RuntimeError('frozen executable changed')
 lock=(run/'worker.lock').open('w')
 try:fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
 except BlockingIOError:raise RuntimeError('another worker owns this queue')
 completed=0;failed=0
 def status(state,**kw):atomic(run/'status.json',{'state':state,'updated_utc':now(),'pid':os.getpid(),'completed_jobs':completed,'failed_jobs':failed,'max_jobs':2340,**kw})
 def execute(j):
  nonlocal completed,failed
  path=run/'results'/(j['ID']+'.json');jobfile=run/'jobs'/(j['ID']+'.json');failure=path.with_suffix('.failed.json')
  atomic(jobfile,j)
  if path.exists():
   r=read(path)
   if r['Job']!=j:raise RuntimeError('checkpoint job mismatch '+j['ID'])
   completed+=1;return r
  if failure.exists():failed+=1;return None
  status('running',stage=j['Stage'],current_job=j['ID'])
  log('start '+j['ID'])
  try:
   p=subprocess.run([str(run/'bin/backtest'),'-job',str(jobfile),'-result',str(path)],capture_output=True,text=True,timeout=240,env={**os.environ,'GOMAXPROCS':'2','GOMEMLIMIT':'1536MiB'})
   if p.returncode:raise RuntimeError(f'exit {p.returncode}: {p.stderr[-3000:]}')
   r=read(path)
   if r['Job']!=j:raise RuntimeError('unexpected job result')
   completed+=1;return r
  except Exception as e:
   if path.exists():path.unlink()
   atomic(failure,{'job':j,'error':str(e),'at_utc':now()});failed+=1;return None
 try:
  status('starting');prepare(plan,status);rows=[]
  for c in plan['configs']:
   for i,period in enumerate(plan['training_periods']):
    for s in SYMBOLS:
     r=execute(make_job(plan,c,s,'training',i,period))
     if r:rows.append(r)
  selection=choose(plan,rows)
  sp=run/'selection.json'
  if sp.exists():
   previous=read(sp)
   if [r['candidate'] for r in previous['selected']]!=[r['candidate'] for r in selection['selected']]:raise RuntimeError('training selection changed from checkpoint')
  else:atomic(sp,selection)
  for picked in selection['selected']:
   c=next(c for c in plan['configs'] if c['id']==picked['candidate'])
   for stage,period in [('validation',plan['validation_period']),('confirmation',plan['confirmation_period']),('stress',plan['confirmation_period'])]:
    for s in SYMBOLS:
     r=execute(make_job(plan,c,s,stage,0,period,stage=='stress'))
     if r:rows.append(r)
  failures=[read(f) for f in sorted((run/'results').glob('*.failed.json'))]
  report(run,plan,rows,failures)
  state='completed_with_errors' if failures else 'completed';status(state,selected_configs=len(selection['selected']))
  (run/'DONE.txt').write_text(f'{state}\n{now()}\nReport: {run / "report.txt"}\n')
  notify(run,'Riset backtest selesai',f'{completed} pengujian selesai, {failed} gagal. Hasil: {run / "report.txt"}. Kabari Codex untuk analisis.')
  log(state)
 except Exception as e:
  status('failed',error=str(e));(run/'FAILED.txt').write_text(str(e)+'\n')
  notify(run,'Riset backtest terhenti',f'{str(e)[:160]}. Checkpoint tersimpan di {run}.');raise
 finally:lock.close()

def launch(run):
 plan=read(run/'plan.json');unit='trading-bot-research-'+run.name.removeprefix('run-').lower()
 args=['systemd-run','--user','--collect','--unit='+unit,'--description=Local crypto backtest research queue','--property=Type=exec','--property=CPUQuota=150%','--property=MemoryMax=3G','--property=MemoryHigh=2500M','--property=Nice=10','--property=IOWeight=10','--property=TasksMax=64','--property=WorkingDirectory='+plan['project'],'--property=StandardOutput=append:'+str(run/'worker.log'),'--property=StandardError=append:'+str(run/'worker.log'),'--setenv=GOMAXPROCS=2','--setenv=GOMEMLIMIT=1536MiB','--setenv=DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/'+str(os.getuid())+'/bus',sys.executable,str(run/'worker.py'),'worker','--run-dir',str(run)]
 subprocess.run(args,check=True);atomic(run/'service.json',{'unit':unit+'.service','launched_utc':now()});print(unit+'.service',flush=True)

def main():
 p=argparse.ArgumentParser();p.add_argument('command',choices=['init','launch','worker','status']);p.add_argument('--run-dir',type=pathlib.Path);p.add_argument('--cache',type=pathlib.Path,default=pathlib.Path('/tmp/trading-bot-year'));a=p.parse_args()
 if a.command=='init':init(pathlib.Path.cwd(),a.cache);return
 if a.run_dir is None:p.error('--run-dir is required')
 run=a.run_dir.resolve()
 if a.command=='launch':launch(run)
 elif a.command=='worker':worker(run)
 else:print(json.dumps(read(run/'status.json'),indent=2))
if __name__=='__main__':main()
