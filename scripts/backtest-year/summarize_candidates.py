"""Verify the fixed five-candidate experiment and compare it with the prior run."""
import csv,hashlib,json,pathlib,statistics,subprocess
root=pathlib.Path('.')
out=root/'docs/testing/candidates-2026-10-19'
base=root/'docs/testing/backtest-2026-10-19'
raw=pathlib.Path('/tmp/trading-bot-year')
read=lambda p:json.loads(p.read_text())
v=read(out/'results.json');old=read(base/'results.json')
assert len(v['results'])==25 and all(r['Status']=='completed' for r in v['results'])
for k in ['from','to_exclusive','initial_idr_each','initial_usdt_each','usd_idr_fixed','market']:
 assert v[k]==old[k],(k,v[k],old[k])
assert hashlib.sha256((out/'candidate-spec.json').read_bytes()).hexdigest()==v['candidate_spec_sha256']
assert (raw/'instruments.json').read_bytes()==(base/'instruments.json').read_bytes()
assert len(list(out.glob('*-equity.csv')))==25 and len(list(out.glob('*-trades.csv')))==25
ids=[c['ID'] for c in v['candidates']]
assert len(ids)==5 and len(set(ids))==5
labels={'ema4h-trend':'EMA 4 jam + tren','ema1d-trend':'EMA harian + tren','breakout1d':'Breakout harian','momentum1d':'Momentum harian','pullback4h':'Koreksi dalam tren naik (4 jam)'}
ledger_checks=[]
for r in v['results']:
 with (out/f"{r['Strategy']}-{r['Symbol']}-trades.csv").open() as f:tr=list(csv.DictReader(f))
 assert len(tr)==r['Metrics']['Trades']
 check={'symbol':r['Symbol'],'candidate':r['Strategy'],'closed_trades':len(tr),'open_positions':r['OpenPositions']}
 if not r['OpenPositions']:
  net=sum(float(t['net_pnl']) for t in tr)
  diff=r['Metrics']['FinalEquity']-r['Metrics']['InitialEquity']-net
  assert abs(diff)<=len(tr)*0.000051+1e-8,(r['Symbol'],r['Strategy'],diff)
  check['equity_vs_exported_trade_pnl_residual_usdt']=diff
 else:check['note']='Final equity also includes open inventory; closed-trade PnL alone is not a reconciliation.'
 ledger_checks.append(check)
summary=[]
for name in ids:
 rs=[r for r in v['results'] if r['Strategy']==name]
 summary.append({'candidate':name,'label':labels[name],'positive_coin_cases':sum(r['Metrics']['TotalReturn']>1e-9 for r in rs),'negative_coin_cases':sum(r['Metrics']['TotalReturn']<-1e-9 for r in rs),'zero_trade_coin_cases':sum(r['Metrics']['Trades']==0 and r['OpenPositions']==0 for r in rs),'mean_return_pct':statistics.mean(r['Metrics']['TotalReturn']*100 for r in rs),'mean_final_idr':statistics.mean(r['FinalIDR'] for r in rs),'closed_trades_total':sum(r['Metrics']['Trades'] for r in rs),'worst_max_drawdown_pct':max(r['Metrics']['MaxDrawdown']*100 for r in rs),'total_closed_trade_fees_usdt':sum(r['Metrics']['TotalFees'] for r in rs)})
summary.sort(key=lambda r:r['mean_return_pct'],reverse=True)
comparison={'purpose':'Exploratory comparison of exactly five predeclared alternatives on the SAME previously inspected year; not untouched out-of-sample validation','summary':summary,'ledger_checks':ledger_checks,'source_revision':subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip(),'initial_idr_each':2000000,'baseline_results_sha256':hashlib.sha256((base/'results.json').read_bytes()).hexdigest(),'candidate_results_sha256':hashlib.sha256((out/'results.json').read_bytes()).hexdigest(),'raw_csv_sha256':{sym:hashlib.sha256((raw/(sym+'.csv')).read_bytes()).hexdigest() for sym in ['BTCUSDT','ETHUSDT','BNBUSDT','XRPUSDT','SOLUSDT']},'code_sha256':{str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in [root/'scripts/backtest-year/main.go',root/'internal/strategy/trendspot/trendspot.go',root/'internal/strategy/trendspot_test.go',root/'scripts/backtest-year/main_test.go']},'checks':{'go_test_race_all':'passed','go_vet_all':'passed','go_build_all':'passed','golangci_lint':'0 issues','paper_restart_integration':'passed on dedicated local tradingbot_phase9_test database'},'deployment':'Local research only; active configs and VPS unchanged'}
(out/'comparison.json').write_text(json.dumps(comparison,indent=2)+'\n')
lines=['PERBANDINGAN LIMA KANDIDAT STRATEGI',f"Periode: {v['from']} sampai {v['to_exclusive']} (akhir eksklusif).",'Lima koin: BTC, ETH, BNB, XRP, SOL. Modal Rp2.000.000 per kandidat per koin.',f"25 simulasi selesai dalam {v['elapsed_seconds']:.2f} detik, termasuk membaca CSV; tanpa unduh ulang.",'Biaya, modal, periode, data dan batas risiko sama dengan pengujian sebelumnya.','Aturan lima kandidat dikunci di candidate-spec.json sebelum simulasi; tidak ada pencarian parameter lanjutan.','','Rata-rata lima pengujian independen; bukan simulasi portofolio bersama:']
for s in summary:lines.append(f"{s['label']}: {s['mean_return_pct']:+.3f}%; untung {s['positive_coin_cases']}/5; rugi {s['negative_coin_cases']}/5; tidak trading {s['zero_trade_coin_cases']}/5; {s['closed_trades_total']} penutupan transaksi; drawdown terburuk {s['worst_max_drawdown_pct']:.2f}%")
lines+=['','Hasil per koin (perubahan nilai modal):','Koin | EMA 4 jam | EMA harian | Breakout harian | Momentum harian | Koreksi 4 jam']
for sym in ['BTCUSDT','ETHUSDT','BNBUSDT','XRPUSDT','SOLUSDT']:
 rs={r['Strategy']:r for r in v['results'] if r['Symbol']==sym}
 lines.append(sym[:-4]+' | '+' | '.join(f"{rs[n]['Metrics']['TotalReturn']*100:+.2f}%" for n in ids))
lines+=['','Rincian kandidat koreksi 4 jam:']
for r in v['results']:
 if r['Strategy']=='pullback4h':
  m=r['Metrics'];baseline=next(x for x in old['results'] if x['Symbol']==r['Symbol'] and x['Strategy']=='emacross')
  lines.append(f"{r['Symbol']}: Rp{r['FinalIDR']:,.0f} ({m['TotalReturn']*100:+.2f}%), {m['Trades']} penutupan, drawdown {m['MaxDrawdown']*100:.2f}%; EMA lama Rp{baseline['FinalIDR']:,.0f} ({baseline['Metrics']['TotalReturn']*100:+.2f}%).")
lines+=['','Cara kerja kandidat koreksi:', '- Mengamati candle 4 jam yang sudah ditutup.', '- Membeli hanya di atas EMA 200 yang sedang naik, setelah RSI pulih menembus 40 dari bawah dan masih di bawah 65.', '- Keluar ketika RSI mencapai 65 atau harga di bawah EMA; stop 2,5 ATR dan target 5 ATR juga dipasang.', '- Risiko dan batas ukuran posisi tetap sama; tidak menambah modal atau menggunakan leverage.','','Batasan penting:', '- EMA harian tidak melakukan transaksi sama sekali; hasil 0% bukan bukti strategi berhasil.', '- Kandidat koreksi untung pada 3/5 koin dan rugi pada BNB/SOL. Sampel kecil dan keuntungan relatif tipis.', '- Beberapa kandidat lain masih memegang koin pada akhir periode; lihat OpenPositions pada results.json. Nilainya dihitung pada harga terakhir tanpa biaya penjualan akhir.', '- Tahun ini sudah pernah diperiksa; membandingkan kandidat di sini bukan pengujian pada data baru yang belum terlihat.', '- Agregasi 4 jam/harian memakai data satu menit yang sama. Warmup indikator terjadi dalam tahun pengujian; selama warmup saldo tetap tunai.', '- Hasil belum cukup untuk memilih konfigurasi live. Perlu data periode lain yang belum dipakai memilih strategi dan pengujian paper.','','Asumsi lengkap:']+['- '+s for s in v['assumptions']]
lines+=['','Validasi: seluruh test dengan race detector, build, vet, lint dan test integrasi pemulihan sesi lokal lolos.','Aktifasi: strategi baru terdaftar sebagai trendspot, tetapi tidak diaktifkan pada konfigurasi lokal/VPS.','','Reproduksi (dari root project):','go run ./scripts/backtest-year -suite candidates -declare','go run ./scripts/backtest-year -suite candidates','python scripts/backtest-year/summarize_candidates.py','Data mentah yang dibutuhkan: /tmp/trading-bot-year/*.csv dan instruments.json; unduh ulang dengan download.py bila cache hilang.']
(out/'summary.txt').write_text('\n'.join(lines)+'\n')
print('\n'.join(lines[:22]))
