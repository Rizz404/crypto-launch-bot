"""Compare identical simulated spot trades with standard vs hypothetical BNB-discount fees.
No account settings, real BNB purchases or live orders. Fixed quote-equivalent fee model.
"""
import argparse,datetime as dt,json,pathlib,subprocess,os,hashlib

def main():
 p=argparse.ArgumentParser();p.add_argument('--run-dir',type=pathlib.Path,default=pathlib.Path('var/backtest-research/latest'));p.add_argument('--out',type=pathlib.Path,default=pathlib.Path('docs/testing/fee-sensitivity-2026-10-19'));a=p.parse_args()
 parent=a.run_dir.resolve();out=a.out.resolve();out.mkdir(parents=True,exist_ok=True)
 symbols=['BTCUSDT','ETHUSDT','BNBUSDT','XRPUSDT','SOLUSDT'];cases=[]
 for s in symbols:
  baseline_path=parent/'results'/f'confirmation-momentum-028-aggressive-{s}.json';baseline=json.loads(baseline_path.read_text());job=baseline['Job'].copy();assert job['Fee']==.001 and job['InputTF']=='1m'
  assert abs(baseline['Metrics']['InitialEquity']*18052-2000000)<.001
  job['ID']='fee-discount-'+job['Candidate']+'-'+s;job['Stage']='fee_discount';job['Fee']=.00075
  cases.append({'symbol':s,'baseline_path':str(baseline_path),'baseline_sha256':hashlib.sha256(baseline_path.read_bytes()).hexdigest(),'job':job})
 plan={'created_utc':dt.datetime.now(dt.timezone.utc).isoformat(),'purpose':'Isolate fee-rate sensitivity, same strategy, data, capital, slippage and risk','capital_idr_each':2000000,'standard_fee':.001,'discount_fee':.00075,'fee_source':'https://www.binance.com/en/fee/trading','discount_requirements_source':'https://www.binance.com/en-NG/support/faq/detail/115000583311','assumptions':['Hypothetical BNB discount rate, not confirmation of actual account eligibility','No ETH network gas paid for internal Binance Spot trades in this model','Quote-equivalent fee-rate change only; separate BNB balance, acquisition costs and BNB price fluctuations are not simulated','Same previously inspected confirmation year, 2025-10-19..2026-10-19; this is cost sensitivity, not a new strategy holdout','Fixed USD/IDR rate, no taxes; no real orders or account changes'],'cases':cases}
 (out/'plan.json').write_text(json.dumps(plan,indent=2)+'\n')
 results=[]
 for c in cases:
  jobfile=out/(c['symbol']+'-job.json');resultfile=out/(c['symbol']+'-result.json');jobfile.write_text(json.dumps(c['job'],indent=2)+'\n')
  subprocess.run([str(parent/'bin/backtest'),'-job',str(jobfile),'-result',str(resultfile)],check=True,timeout=240,env={**os.environ,'GOMAXPROCS':'2','GOMEMLIMIT':'1536MiB'})
  new=json.loads(resultfile.read_text());old=json.loads(pathlib.Path(c['baseline_path']).read_text())
  r={'symbol':c['symbol'],'standard_return_pct':old['ConservativeReturn']*100,'discount_return_pct':new['ConservativeReturn']*100,'standard_final_idr':old['ConservativeFinalIDR'],'discount_final_idr':new['ConservativeFinalIDR'],'difference_final_idr':new['ConservativeFinalIDR']-old['ConservativeFinalIDR'],'standard_closed_trade_fees_idr':old['Metrics']['TotalFees']*18052,'discount_closed_trade_fees_idr':new['Metrics']['TotalFees']*18052,'standard_closed_trades':old['Metrics']['Trades'],'discount_closed_trades':new['Metrics']['Trades']}
  results.append(r);(out/'results.json').write_text(json.dumps({'plan':plan,'results':results},indent=2)+'\n');print(c['symbol'],f"{r['standard_return_pct']:+.2f}% -> {r['discount_return_pct']:+.2f}%; final capital difference Rp{r['difference_final_idr']:.0f}",flush=True)
 lines=['SENSITIVITAS BIAYA BINANCE SPOT','Modal setiap pengujian Rp2 juta. Momentum-028-aggressive, 19 Oktober 2025–19 Oktober 2026.','Perubahan hanya tarif komisi: 0,1% menjadi skenario diskon 0,075% per sisi. Bukan perubahan akun nyata.','']
 for r in results:lines.append(f"{r['symbol']}: {r['standard_return_pct']:+.2f}% -> {r['discount_return_pct']:+.2f}%; selisih saldo akhir Rp{r['difference_final_idr']:,.0f}")
 lines+=['','Asumsi:']+['- '+x for x in plan['assumptions']]
 (out/'summary.txt').write_text('\n'.join(lines)+'\n')
if __name__=='__main__':main()
