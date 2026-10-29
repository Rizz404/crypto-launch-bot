# Riset strategi terbatas — 29 Oktober 2026

Dua kandidat **yang sudah ada dan parameternya dibekukan**: `pullback4h` dan
`breakout1d`, masing-masing BTC/ETH/SOL. Dua skenario biaya menghasilkan
**12 job serial**, selesai tanpa error/warning engine. Total waktu job 77,35
detik, termasuk membaca CSV. Tidak mengunduh data atau menjalankan bot/trial
lokal/VPS. GOMAXPROCS 2 dan GOMEMLIMIT 768 MiB.

[Plan sebelum run dan hash data](plan.json), [hasil lengkap](results.json),
[ringkasan](summary.json). Reuse CLI `scripts/backtest-year -job … -result …`.
Job JSON bisa diekspor dari array `jobs` pada plan; hasil mentah lokal juga
ada di `var/backtest-research/strategy-cost-review/`.

Periode **19 Oktober 2025–19 Oktober 2026**, candle eksekusi 1m. Ini tahun
yang **sudah pernah ditinjau**, bukan holdout baru. Modal tiap kasus independen
Rp2 juta virtual (110,791048 USDT pada kurs pembanding lama); risiko 1%/trade,
posisi/exposure maksimal 25%, satu posisi, tanpa leverage. Tidak menjumlahkan
return enam kasus sebagai portofolio modal bersama.

Biaya standar 0,1%/sisi + slippage 2 bps; stres 0,2%/sisi +5 bps. Keduanya
asumsi simulasi, bukan tarif akun aktual. Final return di bawah memasukkan
estimasi biaya exit inventori akhir; BTC breakout masih punya satu posisi.

| Kandidat | Simbol | Return standar | Return stres | Penutupan standar/stres |
|---|---|---:|---:|---:|
| Pullback 4h | BTC | +2,4105% | +1,4533% | 14/14 |
| Pullback 4h | ETH | +2,3497% | +1,1112% | 20/20 |
| Pullback 4h | SOL | −1,4986% | −2,0860% | 10/10 |
| Breakout 1d | BTC | −0,1194% | −0,2406% | 3/3 |
| Breakout 1d | ETH | −0,5424% | −0,6097% | 3/3 |
| Breakout 1d | SOL | +0,1444% | +0,0725% | 3/3 |

Rata-rata return independen pullback turun **+1,0872% → +0,1595%** akibat
biaya stres. Drawdown intraperiode terburuk 3,0863% →3,3227%. Breakout rata-rata
**−0,1725% → −0,2593%**, hanya sembilan penutupan total pada tiga simbol.
Drawdown metrik intraperiode tidak dihitung ulang setelah estimasi exit akhir.

## Keputusan riset

- **Tidak mengganti strategi aktif secara otomatis.** Dua EMA 1m sudah pause;
  Tren/EMA 4h dan policy P3 tetap sesuai keputusan operasi.
- Pullback BTC/ETH layak masuk antrean validasi prospektif kecil setelah feed
  stabil. Ini shortlist hasil eksplorasi, dengan bias seleksi; jangan mengklaim
  BTC/ETH akan lebih baik dari SOL pada periode selanjutnya.
- SOL pullback dan breakout harian belum memberi alasan untuk ekspansi pada
  uji ini. Breakout tetap referensi arsip; tidak membuat keluarga strategi baru
  atau menyisir parameter tambahan.
- Varian berikut harus punya versi/periode baru, biaya dan sizing yang sama,
  ledger/modal terpisah bila diaktifkan. Catat eksposur, frekuensi penutupan,
  PnL setelah seluruh biaya dan drawdown; perpanjang observasi bila trade sedikit.
  Tidak ada DB/wallet/modal baru yang diprovisikan pada riset ini.
- Kalender/biaya model/sumber P3 tetap dinilai terpisah dari simulasi price-only
  ini. Belum ada dasar untuk menyatakan AI atau strategi lambat lebih unggul.

Simulator belum memodelkan antrean order book/latency, aturan instrumen memakai
snapshot historis yang tersedia dan tidak memasukkan pajak. Tidak ada izin
uang asli atau janji profit dari dua kandidat ini.
