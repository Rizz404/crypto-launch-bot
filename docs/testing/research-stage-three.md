# Paper strategi tambahan — tahap tiga

**Aktif sejak 29 Oktober 2026 23:14:26 WIB** (`2026-10-29T16:14:26.402915148Z`), enam instance/enam binding running. Konfigurasi dibekukan pada `e56d051`; binary tetap `d30b93b`. [Bukti aktivasi/preservasi](research-stage-three-activation.json). Ini eksperimen indikator terpisah di bawah roadmap Plan 3, bukan penggantian pilot AI P3.

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

- Pullback: trend EMA 200, RSI 14 entry 40/exit 65, entry 55/exit 20, ATR 14 stop 2,5×/target 5×, trailing 0. Sama dengan pullback P2, simbol diganti.
- EMA: fast 12/slow 26, trend EMA 100, RSI 14 long ≤70/short ≥30, ATR 14 stop 2×/target 3×, trailing 0, exit-on-cross aktif, short nonaktif. Sama dengan EMA tren 1m P2, timeframe diganti.
- Spot paper Binance; fee maker/taker masing-masing **0,1% per sisi**, slippage **2 bps**. Tidak memakai API trading/live.
- Risiko tiap instance: 1% modal/trade, posisi maksimal 25%, satu posisi, leverage 1, eksposur 25%, daily loss 5%, drawdown 20%, close-on-kill. Wallet: maksimal 6 posisi/eksposur 75%; batas simbol parent tetap berlaku. Risk sizing/minimum notional dapat menyebabkan sinyal ditolak; penolakan harus dihitung saat review.

## Isolasi dan operasi

Service `research` di proyek `trading-bot-paper`, DB **`tradingbot_research_stage_three`**, wallet **`paper:research-stage-three`**. Database server/DNS/monitoring dipakai bersama; data dan ledger terpisah. Worker memakai binary rilis `d30b93b` dengan digest tetap yang sama dengan P2; versi konfigurasi dicatat terpisah pada bukti aktivasi. Limit 0,5 CPU/512 MiB, pool 4. Tidak menjalankan poller Telegram kedua. Dashboard loopback **18082**, health/metrics **19102**; akses dengan SSH tunnel dan auth yang sama dengan dashboard paper.

P2/P3, modal, parameter, T0 dan pause lama dipertahankan. Aktivasi hanya menambah DB/service research dan membuat ulang Prometheus proyek paper untuk memasang target baru; volume metrics dipertahankan. Worker P2/P3, DB dan DNS tidak direstart. Backup paper otomatis mencakup database baru; arsip konfigurasi menyertakan profil baru. Observer paper memeriksa dua worker, kedua ledger, kedelapan seri BTC/ETH × 1m/15m/1h/4h serta target Prometheus.

## Protokol review

T0 memakai waktu wallet mulai di atas. **Review awal 30 Oktober 23:14 WIB (D+1), perbandingan 1 November 23:14 WIB (D+3), checkpoint 5 November 23:14 WIB (D+7)**. Jika fill sedikit, teruskan hingga 28 November 23:14 WIB (D+30) tanpa mengganti parameter diam-diam. Tidak ada pengingat chat otomatis baru.

| Waktu | Yang diperiksa | Keputusan |
|---|---|---|
| D+1 | Semua kasus running, candle lengkap/segar, risk rejection, fee/slippage, restart/attention, backup | Perbaiki gangguan operasi; jangan optimasi sinyal |
| D+3 | Ringkasan per kasus: net return, gross PnL, fee, turnover, drawdown, exposure, closed trades dan signal rejection | Bandingkan biaya/frekuensi; hasil masih awal |
| D+7 | Metrik sama, stabilitas feed, perbandingan simbol/timeframe dan buy-and-hold periode yang sama | Lanjut/hentikan kasus dengan alasan; sampel kecil tetap belum konklusif |
| D+30 atau cukup transaksi | Konsistensi lintas kondisi pasar dan biaya, bukan hanya return tertinggi | Kandidat riset berikutnya; live tetap memerlukan keputusan sendiri |

Ambil snapshot bersama pada cutoff yang sama untuk P2, tahap tiga dan P3. Pisahkan unrealized/realized, gross/net, hot/archive fills; posisi belum ditutup bukan nol hasil. Catat lama observasi dan jumlah trade per kasus; enam kasus berarti risiko memilih pemenang kebetulan. Return dibandingkan dalam persen modal awal; modal P2/P3 berbeda. P3 juga perlu review sumber/tesis, keputusan AI invalid dan kuota, terpisah dari profit indikator.

Jangan reset ledger/T0, mengisi trade buatan, memaksa sinyal/AI, mengganti biaya atau menyetel parameter selama periode. Gangguan harus dicatat bersama durasinya. Observer tidak melakukan recovery/trading otomatis. Tidak membuat jadwal pesan/pengingat baru; bukti sistem tersedia melalui timer observer/backup yang telah ada.

Rollback: hentikan hanya service `research`; simpan DB/history, catat cutoff dan alasan. Jika dinonaktifkan permanen, keluarkan target research dari Prometheus dan pemeriksaan observer melalui rilis config berikutnya; jangan menghapus database untuk merapikan alert.

## Validasi rilis

[Seluruh lima job CI lulus](https://github.com/Rizz404/trading-bot/actions/runs/40908911546): Go/race/vet/lint/build, PostgreSQL/recovery, alert Prometheus, image bot dan image/protokol Spider. Uji lokal terarah config, lima fixture operasi, promtool serta validasi Compose VPS juga lulus. Image hasil commit konfigurasi tidak dipasang; ketiga worker tetap menggunakan binary rilis `d30b93b` yang sudah tervalidasi.

Backup pra-aktivasi **23:14:18 WIB**, lalu backup pasca-aktivasi **23:15:40 WIB** berhasil. Backup kedua memuat dump DB tahap tiga dan profil baru; checksum serta daftar isi dump lulus. Belum ada restore drill baru untuk DB tahap tiga, simulasi outage atau recovery posisi aktif. Backup masih satu VPS sesuai keputusan sebelumnya.

Sampel sesaat **23:15:42 WIB**: research CPU 1,82% satu core, RAM 46,53 MiB/512; P2 CPU 12,76%, DB 13,71%, P3 1,97%. Bukan pengukuran beban puncak/24 jam.

## Checkpoint aktivasi

Konfigurasi diambil lewat GitHub; tidak menyalin aplikasi dari laptop. Migrasi hanya DB baru (schema 5); image tetap `ghcr.io/rizz404/trading-bot@sha256:fbf28da706e06aebf022e964e12ee1e29183eb948a5b6261dd7617b5a7f7665e`. P2 dan worker research memakai binary yang sama, tetapi profil/DB/wallet berbeda. Source konfigurasi/operasi paper `e56d051`; checkout P3 tetap `d30b93b`. Checkout paper dapat menunjuk commit dokumentasi sesudahnya tanpa mengubah image/config aktif.

Pemeriksaan **23:14:54–23:14:57 WIB**: enam kasus running, kas awal per kasus sama persis dengan tabel; nol fill/holding/pending pada snapshot awal, tidak halt/attention. Delapan seri candle tersedia, kedua target Prometheus up, HTTP health P2/research/P3 semuanya 200. Dashboard auth 200/tanpa auth 401. Observer paper exit 0 `problems: []`. Ini bukti start/health, belum penerimaan performa.

Sebanyak **37 dari 38 container lama identik**, termasuk worker P2/P3, DB, DNS dan Grafana. Satu Prometheus proyek paper dibuat ulang dengan volume tetap; satu worker research ditambahkan. Proyeksi finansial/config/T0/status kedua wallet lama identik sebelum/sesudah, config mount P2 identik, kuota P3 tetap 4/12. P2 tetap 214 fill total (206 hot+8 archive pada checkpoint ini); perubahan rasio hot/archive adalah compaction.

P3 melewati jadwal D+1 pada malam ini: runtime/feed sehat, belum ada fill/holding/pending dan modal tetap 1.000 USDT, kuota 4/12. Observer masih exit 1 untuk `research_error`/`latest_review_invalid` dari WATCH simbol kosong lama. Tidak ada inference baru yang dipaksakan. Review riset/keputusan AI belum lulus hanya karena health 200. Jadwal P3 berikutnya **31 Oktober/4 November 22:53 WIB**. Pada review bersama 1 November, periksa P2 kelanjutan pasca-pause, enam kasus baru D+3 dan P3 sekitar D+4; jadwal ini tidak mereset T0 masing-masing.

Observer paper kemudian terpicu **otomatis pada 23:17:39 WIB**, unit selesai success/exit 0 dan sample 23:17:40 memiliki `problems: []`. Pada23:17:41, masing-masing BTC/ETH memiliki dua candle 1m pasca-start yang berurutan; health research tetap 200. Semua timer paper/P3 tetap enabled/active. Ini observasi beberapa menit, belum 24 jam.

## Pemeriksaan saat kembali

Jalankan observer paper lewat SSH; exit 0 berarti pemeriksaan operasionalnya lulus, bukan strategi untung:

```sh
ssh plesk 'cd /opt/trading-bot-paper/repo && python3 deploy/vps-observe.py'
ssh plesk 'cd /opt/trading-bot-catalyst/repo && python3 deploy/catalyst-observe.py'
```

Untuk dashboard tahap tiga, buka tunnel berikut di satu terminal, lalu kunjungi `http://127.0.0.1:28082` dan gunakan password dashboard paper yang sudah ada. `/api/state` mengekspor snapshot terautentikasi. Jangan menempelkan credential ke laporan/command history.

```sh
ssh -N -L 28082:127.0.0.1:18082 plesk
```

P2 tetap port 18080/metrics 19100; P3 tetap port 18081. Simpan snapshot JSON dan observer pada cutoff yang sama, lalu isi tabel enam instance di atas dengan equity/PnL, biaya, transaksi tutup, drawdown, exposure dan penolakan sinyal. Hitung total fill hot+archive; jangan menyimpulkan data hilang karena compaction.
