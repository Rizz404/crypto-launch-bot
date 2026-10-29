# Paper strategi tambahan — tahap tiga

Status 29 Oktober 2026: konfigurasi dibekukan; aktivasi dan T0 akan dicatat setelah pemeriksaan VPS. Ini eksperimen indikator terpisah di bawah roadmap Plan 3, bukan penggantian pilot AI P3.

## Hipotesis dan kasus

P2 tujuh hari rugi 0,9941%; 77,67% kerugian bersih setara biaya transaksi. Dua EMA 1m dijeda. Hipotesis baru: timeframe lebih lambat mengurangi churn/fee; pullback BTC/ETH layak diuji prospektif setelah hasil replay positif. Replay memakai tahun yang telah dilihat, sehingga bukan bukti out-of-sample. Tidak ada klaim profit.

| Instance | Sinyal | Simbol | Timeframe | Modal virtual USDT |
|---|---|---|---|---|
| pullback-btc-4h | trendspot pullback | BTCUSDT | 4h | 110,79104808331486 |
| pullback-eth-4h | trendspot pullback | ETHUSDT | 4h | 110,79104808331486 |
| ema-btc-15m | EMA tren | BTCUSDT | 15m | 110,79104808331486 |
| ema-eth-15m | EMA tren | ETHUSDT | 15m | 110,79104808331486 |
| ema-btc-1h | EMA tren | BTCUSDT | 1h | 110,79104808331486 |
| ema-eth-1h | EMA tren | ETHUSDT | 1h | 110,79104808331486 |

Total **664,74628849988916 USDT virtual**, modal baru eksplisit, bukan transfer P2/P3. Setiap instance hanya satu binding `signal`; BTC/ETH tidak berbagi jatah satu posisi. Tiga strategi × dua simbol; tidak menambah SOL atau breakout harian yang hasil replay-nya lemah.

Sumber konfigurasi: [profil tahap tiga](../../configs/managed.research-stage-three.yaml). Parameter dibekukan sepanjang periode observasi:

- Pullback: trend EMA 200, RSI 14 entry40/exit65, entry55/exit20, ATR14 stop2,5×/target5×, trailing0. Sama dengan pullback P2, simbol diganti.
- EMA: fast12/slow26, trend EMA100, RSI14 long≤70/short≥30, ATR14 stop2×/target3×, trailing0, exit-on-cross aktif, short nonaktif. Sama dengan EMA tren 1m P2, timeframe diganti.
- Spot paper Binance; fee maker/taker masing-masing **0,1% per sisi**, slippage **2 bps**. Tidak memakai API trading/live.
- Risiko tiap instance: 1% modal/trade, posisi maksimal25%, satu posisi, leverage1, eksposur25%, daily loss5%, drawdown20%, close-on-kill. Wallet: maksimal6 posisi/eksposur75%; batas simbol parent tetap berlaku. Risk sizing/minimum notional dapat menyebabkan sinyal ditolak; penolakan harus dihitung saat review.

## Isolasi dan operasi

Service `research` di proyek `trading-bot-paper`, DB **`tradingbot_research_stage_three`**, wallet **`paper:research-stage-three`**. Database server/DNS/monitoring dipakai bersama; data dan ledger terpisah. Worker memakai binary rilis `d30b93b` dengan digest tetap yang sama dengan P2; versi konfigurasi dicatat terpisah pada bukti aktivasi. Limit 0,5 CPU/512 MiB, pool4. Tidak menjalankan poller Telegram kedua. Dashboard loopback **18082**, health/metrics **19102**; akses dengan SSH tunnel dan auth yang sama dengan dashboard paper.

P2/P3, modal, parameter, T0 dan pause lama dipertahankan. Aktivasi hanya menambah DB/service research dan membuat ulang Prometheus proyek paper untuk memasang target baru; volume metrics dipertahankan. Worker P2/P3, DB dan DNS tidak direstart. Backup paper otomatis mencakup database baru; arsip konfigurasi menyertakan profil baru. Observer paper memeriksa dua worker, kedua ledger, kedelapan seri BTC/ETH × 1m/15m/1h/4h serta target Prometheus.

## Protokol review

T0 adalah waktu wallet baru benar-benar mulai, bukan waktu dokumen dibuat. Jadwal kalender diisi dari bukti aktivasi. Pengamatan pertama D+1, perbandingan awal D+3, checkpoint D+7; jika fill sedikit, teruskan hingga D+30 tanpa mengganti parameter diam-diam.

| Waktu | Yang diperiksa | Keputusan |
|---|---|---|
| D+1 | Semua kasus running, candle lengkap/segar, risk rejection, fee/slippage, restart/attention, backup | Perbaiki gangguan operasi; jangan optimasi sinyal |
| D+3 | Ringkasan per kasus: net return, gross PnL, fee, turnover, drawdown, exposure, closed trades dan signal rejection | Bandingkan biaya/frekuensi; hasil masih awal |
| D+7 | Metrik sama, stabilitas feed, perbandingan simbol/timeframe dan buy-and-hold periode yang sama | Lanjut/hentikan kasus dengan alasan; sampel kecil tetap belum konklusif |
| D+30 atau cukup transaksi | Konsistensi lintas kondisi pasar dan biaya, bukan hanya return tertinggi | Kandidat riset berikutnya; live tetap memerlukan keputusan sendiri |

Ambil snapshot bersama pada cutoff yang sama untuk P2, tahap tiga dan P3. Pisahkan unrealized/realized, gross/net, hot/archive fills; posisi belum ditutup bukan nol hasil. Catat lama observasi dan jumlah trade per kasus; enam kasus berarti risiko memilih pemenang kebetulan. Return dibandingkan dalam persen modal awal; modal P2/P3 berbeda. P3 juga perlu review sumber/tesis, keputusan AI invalid dan kuota, terpisah dari profit indikator.

Jangan reset ledger/T0, mengisi trade buatan, memaksa sinyal/AI, mengganti biaya atau menyetel parameter selama periode. Gangguan harus dicatat bersama durasinya. Observer tidak melakukan recovery/trading otomatis. Tidak membuat jadwal pesan/pengingat baru; bukti sistem tersedia melalui timer observer/backup yang telah ada.

Rollback: hentikan hanya service `research`; simpan DB/history, catat cutoff dan alasan. Jika dinonaktifkan permanen, keluarkan target research dari Prometheus dan pemeriksaan observer melalui rilis config berikutnya; jangan menghapus database untuk merapikan alert.
