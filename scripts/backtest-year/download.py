"""Download official Binance spot archives, verify SHA256 and validate one-minute continuity.
No credentials or trading endpoints are used. Cache defaults to /tmp/trading-bot-year.
"""
import concurrent.futures as cf
import csv, datetime as dt, hashlib, io, json, pathlib, sys, time, urllib.request, zipfile
ROOT = pathlib.Path(sys.argv[1] if len(sys.argv)>1 else '/tmp/trading-bot-year')
ROOT.mkdir(parents=True, exist_ok=True)
SYMBOLS=['BTCUSDT','ETHUSDT','BNBUSDT','XRPUSDT','SOLUSDT']
START=int(dt.datetime(2025,10,19,tzinfo=dt.timezone.utc).timestamp())
END=int(dt.datetime(2026,10,19,tzinfo=dt.timezone.utc).timestamp())
def fetch(url):
 for attempt in range(4):
  try:
   with urllib.request.urlopen(url,timeout=60) as r: return r.read()
  except Exception:
   if attempt==3: raise
   time.sleep(attempt+1)
def archive(job):
 sym,period,kind=job
 name=f'{sym}-1m-{period}.zip'
 url=f'https://data.binance.vision/data/spot/{kind}/klines/{sym}/1m/{name}'
 path=ROOT/name
 expected=fetch(url+'.CHECKSUM').decode().split()[0]
 if not path.exists() or hashlib.sha256(path.read_bytes()).hexdigest()!=expected:
  data=fetch(url)
  if hashlib.sha256(data).hexdigest()!=expected: raise ValueError(f'checksum {name}')
  path.write_bytes(data)
 with zipfile.ZipFile(path) as z:
  if z.testzip(): raise ValueError(f'CRC {name}')
 return {'name':name,'url':url,'sha256':expected,'bytes':path.stat().st_size}
periods=[(f'{y}-{m:02}','monthly') for y,m in [(2025,10),(2025,11),(2025,12)]+[(2026,m) for m in range(1,10)]]+[(f'2026-10-{d:02}','daily') for d in range(1,19)]
jobs=[(s,p,k) for s in SYMBOLS for p,k in periods]
records=[]
with cf.ThreadPoolExecutor(max_workers=4) as pool:
 for n,r in enumerate(pool.map(archive,jobs),1):
  records.append(r)
  if n%10==0: print(f'verified {n}/{len(jobs)} archives',flush=True)
manifest={'from':'2025-10-19T00:00:00Z','to_exclusive':'2026-10-19T00:00:00Z','archives':records,'series':[]}
for sym in SYMBOLS:
 count=0; prev=None; first=None
 with (ROOT/(sym+'.csv')).open('w') as out:
  w=csv.writer(out)
  for p,k in periods:
   with zipfile.ZipFile(ROOT/f'{sym}-1m-{p}.zip') as z:
    with z.open(z.namelist()[0]) as f:
     for row in csv.reader(io.TextIOWrapper(f)):
      raw=int(row[0]); sec=raw//(1_000_000 if raw>10**14 else 1000)
      if not START<=sec<END: continue
      if prev is not None and sec!=prev+60: raise ValueError(f'gap/duplicate {sym} {prev} {sec}')
      if first is None: first=sec
      prev=sec; count+=1
      w.writerow([sec]+row[1:6]+[row[7],row[8]])
 if count!=(END-START)//60 or first!=START or prev!=END-60: raise ValueError(f'incomplete {sym}: {count}')
 manifest['series'].append({'symbol':sym,'candles':count,'first':first,'last':prev})
 print(f'{sym}: {count} candles, no gaps',flush=True)
manifest['download_finished_utc']=dt.datetime.now(dt.timezone.utc).isoformat()
(ROOT/'manifest.json').write_text(json.dumps(manifest,indent=2))
# Public current instrument rules are an explicit approximation, not historical snapshots.
info=json.loads(fetch('https://data-api.binance.vision/api/v3/exchangeInfo?symbols=%5B%22BTCUSDT%22,%22ETHUSDT%22,%22BNBUSDT%22,%22XRPUSDT%22,%22SOLUSDT%22%5D'))
(ROOT/'instruments.json').write_text(json.dumps(info))
print('download and verification complete',flush=True)
