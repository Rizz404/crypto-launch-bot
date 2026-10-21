"""Produce a compact text report and attach provenance to completed results."""
import hashlib,json,pathlib,subprocess
out=pathlib.Path('docs/testing/backtest')
p=out/'results.json';v=json.loads(p.read_text())
assert len(v['results'])==30
assert sum(r['Status']=='completed' for r in v['results'])==20
v['code_revision']=subprocess.check_output(['git','rev-parse','HEAD'],text=True).strip()
v['research_scripts_sha256']={str(f):hashlib.sha256(f.read_bytes()).hexdigest() for f in pathlib.Path('scripts/backtest-year').glob('*') if f.is_file()}
v['ranking_snapshot']={'as_of':'2026-10-19','BTC':1,'ETH':2,'BNB':4,'XRP':5,'SOL':7,'source':'https://coinucation.com/coins','selection':'five highest-ranked non-stablecoins in top ten'}
p.write_text(json.dumps(v,indent=2)+'\n')
lines=['BACKTEST LOKAL: 19 Oktober 2025 - 19 Oktober 2026 (UTC)',
'Modal setiap kombinasi strategi dan koin: Rp2.000.000 = 110,791048 USDT.',
'Kurs tetap Rp18.052/USD; USDT diasumsikan setara USD.',
'20 simulasi selesai; 10 kombinasi funding/xspread belum didukung.',
f"Waktu seluruh simulasi + baca CSV lokal: {v['elapsed_seconds']:.2f} detik (unduh data tidak termasuk).",'','Saldo akhir (Rp), termasuk nilai koin yang masih dipegang:','Koin      EMA          Grid         DCA          Market making']
for sym in ['BTCUSDT','ETHUSDT','BNBUSDT','XRPUSDT','SOLUSDT']:
 rs={r['Strategy']:r for r in v['results'] if r['Symbol']==sym and r['Status']=='completed'}
 lines.append(sym[:-4].ljust(10)+' '.join(f"{rs[s]['FinalIDR']:>12,.0f}" for s in ['emacross','grid','dca','mm']))
lines+=['','Rincian tiap simulasi (trades = penutupan/penjualan, bukan seluruh order):']
for r in v['results']:
 if r['Status']!='completed':continue
 m=r['Metrics'];lines.append(f"{r['Symbol']} {r['Strategy']}: {m['TotalReturn']*100:+.2f}%; saldo Rp{r['FinalIDR']:,.0f}; drawdown terbesar {m['MaxDrawdown']*100:.2f}%; {m['Trades']} trades; posisi terbuka {r['OpenPositions']}; {r['Seconds']:.2f} detik")
 for msg,count in r['Issues']['Counts'].items():lines.append(f'  Catatan ({count}): {msg}')
lines+=['','Strategi belum diuji:','Funding: mesin backtest belum memasok riwayat funding ke strategi. Biaya funding konstan di mesin bukan pengganti data sinyal funding.','Xspread: perlu riwayat futures tersinkron dari dua bursa. Endpoint publik Bybit mengalami connection reset. Tidak diganti dengan data Binance Spot.','','Asumsi dan batasan:']+['- '+s for s in v['assumptions']]
lines+=['','Sumber:']+[f'{k}: {url}' for k,url in v['sources'].items()]
lines+=['','Reproduksi (dari root repository):','python scripts/backtest-year/download.py /tmp/trading-bot-year','go run ./scripts/backtest-year -data /tmp/trading-bot-year -out docs/testing/backtest','python scripts/backtest-year/summarize.py','','Download membutuhkan internet. Simulasi memakai CSV lokal, tanpa database, API key, atau order nyata.','Data mentah besar ada di /tmp/trading-bot-year dan dapat hilang saat reboot; checksum dan URL arsip disimpan di data-manifest.json.','Hasil ini hanya berlaku untuk parameter dan periode yang diuji; bukan bukti keuntungan di masa depan.']
(out/'summary.txt').write_text('\n'.join(lines)+'\n')
print('\n'.join(lines[:15]))
