# Handoff VPS

Panduan operasional server untuk sesi/agent berikutnya. Status per **2026-10-20
11.55 UTC (18.55 WIB)**. Cakupan dan pekerjaan aktif ada di [Plan 2](plan-2.md);
[Plan 1](plan.md) sudah ditutup sebagai arsip;
prosedur umum ada di [panduan deployment](deployment.md).

## Status sekarang

- **Plan 2 aktif:** fitur banyak instance dengan beberapa strategi dan pembagian
  modal dijadwalkan di [plan baru](plan-2.md). Ini instance virtual aplikasi,
  bukan izin menggandakan container dengan wallet/DB yang sama. Baca
  [rancangan ownership](multi-instance.md). Kode fitur ini belum diterapkan.
  Status server tetap checkpoint 20 Oktober. Audit P2-0 kemudian membaca
  kode/kategori log container stopped secara read-only; dua fix reconciliation
  hanya di lokal, belum di-deploy. [Hasil audit](testing/p2-0-audit-2026-10-20.md).


- **Keputusan strategi 20 Oktober:** ETHUSDT momentum 4h adalah kandidat paper
  utama, BTC momentum harian sebagai pembanding. Baca
  [dokumen keputusan](strategy-decision-2026-10-20.md) sebelum melanjutkan;
  belum ada trial baru atau live dijalankan.


- **Tambahan 2026-10-19:** ETH momentum 4h adalah calon paper terpisah,
  **belum di-deploy di VPS**. User melarang menjalankannya di laptop; trial lokal
  sudah dihentikan. Testnet kemudian ditutup 20 Oktober; persiapan ETH
  diteruskan lewat P2-9 setelah audit/implementasi yang diperlukan.
  Baca [referensi ETH](eth-paper-vps-reference.md) sebelum menyiapkan rilis atau
  instance kedua. Belum ada izin memulai trial baru.

- **Uji multi-strategi dihentikan lebih awal atas permintaan user** pada
  2026-10-20 11:52:43 UTC (18.52.43 WIB), exit 0; run 9 selesai normal.
  Mulai 2026-10-18 16:43 UTC, total sekitar 43 jam 9 menit, bukan 72 jam.
  Tujuh order tersisa dibatalkan di testnet; query ulang exchange nol open order.
  Inventori tetap disimpan. Bot tidak di-resume. Timer observasi disabled/inactive;
  timer backup, DB, DNS, Grafana, Prometheus tetap aktif.
  **Jadwal review 21 Oktober dibatalkan.** Baca
  [analisis final dan bukti](testing/testnet-stop-2026-10-20/analysis.md).
  Backup final `backup-20261020T115448Z`, baseline `differences: []`.
  DB masih menyimpan tujuh order `new` karena pembatalan sesudah recorder stop;
  gunakan bukti exchange final dan rekonsiliasi sebelum trial baru.
- Source `f709429f5f7c697f7d7073dbaa35994ab628f85c` (detached), image
  `ghcr.io/rizz404/trading-bot@sha256:e5b8710bc76da2cd5d581511e19fb7bf9e6a472e0eb61b14fb3c099054fd0438`.
  Rilis sebelumnya, untuk rollback: `69d8dcb` /
  `sha256:166ffc59c471e97717ba361a990e180836fc658a5d792175083294c3f20a1836` (USD/IDR),
  lalu `7cc7096` /
  `sha256:0070bdc06e384868963a3c4a3392101a84ba66df4d634a63cf6b6e7f27ece3eb` (awal uji).
- `TB_APP__MODE=testnet`, kredensial Spot Testnet dan token Telegram ada di
  `.env.vps`. Telegram mengirim ke chat user (chat ID di `configs/config.yaml`
  server). Strategi: `ema-trend-btc`, `ema-fast-eth`, `grid-sol`, `dca-xrp`,
  `mm-doge` ([config](../configs/config.testnet.example.yaml)).
- Pesan Telegram menampilkan nominal USDT beserta perkiraan USD dan rupiah. Kurs
  diambil bot tiap jam dari CoinGecko, dengan cadangan Frankfurter (ECB). Lihat
  [Kurs USD/IDR](#kurs-usdidr-di-telegram).
- Restart terencana di dalam jendela uji: 2026-10-18 17:10 UTC (rilis `69d8dcb`) dan
  17:23 UTC (rilis `f709429`). Run testnet 7 → 8 → 9; ini bukan crash.
- Riwayat: paper 2026-10-17 14:09 → 2026-10-18 16:43 UTC (observasi 24 jam lulus,
  data tetap di DB); host reboot tak terduga 2026-10-17 15:18–15:42 UTC (bukan dari
  user, pulih otomatis). Detail dan bukti ada di plan.
- Mainnet tidak diizinkan sampai user menyetujui P2-10 (kelanjutan Fase 11 lama). `TB_APP__ALLOW_LIVE` dipaksa
  `false` oleh manifest.

## Batas yang wajib dipertahankan

- `ssh plesk` adalah server Plesk bersama dengan banyak proyek produksi. Hanya sentuh
  project Compose `trading-bot-paper` di `/opt/trading-bot-paper`. Jangan mengubah
  Plesk, domain, web server, firewall, atau Docker daemon. Jangan restart Docker/host,
  upgrade global, `docker system prune`, atau `down -v`.
- Sebelum dan sesudah setiap perubahan jalankan
  `python3 /opt/trading-bot-paper/repo/deploy/vps-baseline-check.py` (exit 0 = tidak ada
  perubahan selain restart akibat boot host). Baseline: 28 container dan 5 service.
- Source dari GitHub (deploy key read-only), image dari CI/GHCR berdasarkan digest
  dengan pasangan commit yang sama. Jangan menyalin folder laptop.
- Secret hanya lewat `deploy/vps-set-env.py` (stdin SSH). Jangan mencetak isi
  `.env.vps`, menaruh secret di repo, chat, atau memori bersama.
- Nama project/direktori tetap `trading-bot-paper` walau mode testnet, supaya unit
  systemd, backup, dan observasi tidak perlu diubah.

## Akses dan susunan server

Claude mengakses lewat `ssh -o BatchMode=yes plesk ...` (root). Izin lokal ada di
`.claude/settings.local.json` laptop user (tidak di-commit), dibatasi pada project ini.
Server AlmaLinux 9, amd64, 8 vCPU, RAM sekitar 32 GB, disk sekitar 52 GB kosong
(sekitar 88% terisi), Docker 29 + Compose 5.

```text
/opt/trading-bot-paper/                 # mode 700
  access/                               # deploy key read-only + known_hosts khusus proyek
  repo/                                 # clone GitHub (detached pada commit rilis)
    .env.vps                            # mode 600: password DB/Grafana/web, BOT_IMAGE, mode, kredensial
    configs/config.yaml                 # config aktif (saat ini salinan config.testnet.example + chat ID)
    configs/config.paper.yaml           # config paper lama, untuk kembali ke paper
  evidence/
    baseline.json                       # baseline proyek lain (2026-10-17)
    samples-YYYYMMDD.jsonl              # sampel observasi tiap 5 menit, disimpan 14 hari
  backups/backup-YYYYMMDDThhmmssZ/      # dump DB + config, 7 terakhir disimpan
```

Unit systemd: `trading-bot-paper-observe.timer` (5 menit) dan
`trading-bot-paper-backup.timer` (harian sekitar 03:15 UTC), keduanya aktif.

## Stack

| Container | Limit | Catatan |
|---|---|---|
| bot | 512 MiB, 0,5 CPU | read-only, cap drop all, stop grace 30 s |
| db (TimescaleDB 2.30 pg17) | 1 GiB, 1 CPU | tanpa port host |
| dns (CoreDNS 1.14.7) | 128 MiB, 0,25 CPU | DNS-over-TLS ke Cloudflare khusus bot; network `resolver` 172.31.253.0/29 |
| prometheus | 512 MiB, 0,5 CPU | retensi 7 hari / 1 GB |
| grafana | 512 MiB, 0,5 CPU | biasanya 380–470 MiB, pantau |

Port host hanya loopback: dashboard 18080, metrics/health 19100, Prometheus 19090,
Grafana 13000. Tunnel dari laptop:
`ssh -N -L 18080:127.0.0.1:18080 -L 13000:127.0.0.1:13000 plesk`. Password dashboard
dan Grafana dibaca user sendiri dari `.env.vps`.

DNS ISP/Tailscale server memblokir domain Binance (Trust Positif), sehingga bot
memakai container `dns`. Error WS mencatat tahap dial (`stage dns|connect|tls|upgrade`).

## Operasi harian

```sh
cd /opt/trading-bot-paper/repo
dc() { docker compose --env-file .env.vps -f deploy/compose.vps.yml "$@" </dev/null; }
dc ps
dc logs --tail 100 bot                                   # log JSON
python3 deploy/vps-observe.py                            # satu sampel; problems: [] berarti sehat
curl -s http://127.0.0.1:19100/metrics | grep -E '^tradingbot_(build_info|kill_switch|paused)'
python3 deploy/vps-baseline-check.py                     # proyek lain tidak berubah
```

Selalu `</dev/null` atau `--interactive=false` untuk perintah Compose di dalam
skrip yang dialirkan lewat `ssh ... 'bash -s'`, karena stdin bisa tertelan.

## Analisis uji testnet

**Sudah dianalisis dan ditutup:** [laporan 20 Oktober](testing/testnet-stop-2026-10-20/analysis.md).
Jendela aktual: 2026-10-18 16:43 UTC sampai 2026-10-20 11:52:43 UTC.
Contoh perintah di bawah adalah runbook historis; batas 21 Oktober merupakan
target lama dan tidak menyatakan bot masih berjalan. Jalankan dari
`/opt/trading-bot-paper/repo` dengan fungsi `dc` di atas.

```sh
# 1. Kesehatan: sampel bermasalah, restart, OOM
python3 - <<'PY'
import json, glob
rows = [json.loads(l) for f in sorted(glob.glob("../evidence/samples-*.jsonl")) for l in open(f)]
w = [r for r in rows if "2026-10-18T16:43" <= r["time"] < "2026-10-21T16:44"]
print(len(w), "samples;", sum(1 for r in w if r["problems"]), "with problems")
print({r["time"][:16]: r["problems"] for r in w if r["problems"]})
last = w[-1]["containers"]["data"]
print([(c["service"], c["restarts"], c["oom_killed"]) for c in last])
PY
# 2. WARN/ERROR selama jendela, dikelompokkan
dc logs --no-log-prefix --since 2026-10-18T16:43:00Z bot 2>&1 | python3 -c '
import sys, json, collections
c = collections.Counter(json.loads(l)["msg"] for l in sys.stdin if l.startswith("{") and json.loads(l)["level"] in ("WARN","ERROR"))
print(c.most_common(30))'
# 3. Order dan trade per strategi
dc exec -T db psql -U bot -d tradingbot -c "
SELECT strategy, side, status, count(*) FROM orders WHERE exchange='binance-testnet' GROUP BY 1,2,3 ORDER BY 1,2,3;
SELECT strategy, count(*) trades, count(*) FILTER (WHERE net_pnl>0) wins, round(sum(net_pnl),2) net, round(sum(fees),2) fees
  FROM trades WHERE exchange='binance-testnet' GROUP BY 1 ORDER BY 1;
SELECT id, started_at, stopped_at, stop_reason FROM strategy_runs WHERE mode='testnet' ORDER BY id;"
```

Periksa juga:

- Grid: setiap buy terisi diikuti sell satu level di atas, dan sebaliknya; tidak
  ada order ganda setelah restart.
- DCA: tepat satu pembelian per jam sampai maksimal 24, take profit 3% memulai
  siklus baru.
- MM: frekuensi requote wajar (tidak ada `-1003`/rate limit di log), inventori di
  bawah `max_inventory_quote`, tidak menjual tanpa inventori.
- Emacross: entry disertai OCO SL/TP, exit sekali saja.
- Saldo bot vs exchange: posisi bot tidak boleh melebihi saldo, saldo bawaan
  testnet tidak tersentuh.
- Telegram: ringkasan harian 07:00 WIB terkirim, tidak ada `telegram send failed`,
  nominal disertai USD/Rp, dan tidak ada WARN `exchange rates unavailable` yang
  berulang.
- ERROR `open position without stop loss` untuk XRP muncul di setiap start karena
  DCA memang tanpa stop loss. Ini wajar; nilai apakah perlu dikecualikan.

Testnet tanpa fee: angka untung/rugi bukan dasar keputusan live.

## Rilis, deploy, dan mengganti mode

1. Push ke `main`, tunggu CI sukses, ambil digest dari langkah "Record immutable image
   reference" di log job container.
2. `python3 deploy/vps-baseline-check.py`, lalu `bash deploy/vps-backup.sh`.
3. `git fetch origin main && git checkout --detach <sha>`; set
   `BOT_IMAGE=<image@digest>` dengan `echo BOT_IMAGE=... | python3 deploy/vps-set-env.py`
   (diterima sejak `f709429`; skrip menolak nama selain `TB_*` dan `BOT_IMAGE`).
4. `dc run --interactive=false --rm --no-deps bot migrate status` (jalankan
   `migrate up` bila ada migrasi baru).
5. Gerbang DNS: `dc up -d --no-deps dns`, lalu cek jawabannya:
   `nsenter -t "$(docker inspect -f '{{.State.Pid}}' trading-bot-paper-dns-1)" -n dig +short @127.0.0.1 A stream.binance.com`
   (bukan `139.255.196.196`/`182.23.79.195`). Baru setelah itu `dc up -d --no-deps bot`.
6. Verifikasi log startup, `vps-observe.py`, metrics `build_info`, lalu baseline lagi.

Mengganti mode:

- **Ke paper:** `cp configs/config.paper.yaml configs/config.yaml`,
  `echo TB_APP__MODE=paper | python3 deploy/vps-set-env.py`, recreate bot. Paper
  melanjutkan sesi paper terakhir di DB.
- **Ke testnet:** salin `configs/config.testnet.example.yaml` ke `configs/config.yaml`,
  isi `telegram.chat_ids`, sesuaikan rentang grid dengan harga saat itu, set
  `TB_APP__MODE=testnet` dan kredensial Spot Testnet lewat `vps-set-env.py`.
- **Live:** belum boleh. Butuh persetujuan P2-10 Plan 2, `allow_live` di manifest,
  kredensial mainnet terpisah, dan batas risk dari user.

Secret dari laptop:

```sh
# di laptop, dari root repo; nilai tidak pernah tampil
grep -E '^TB_EXCHANGES__BINANCE__API_(KEY|SECRET)=' .env | ssh -o BatchMode=yes plesk 'python3 /opt/trading-bot-paper/repo/deploy/vps-set-env.py'
```

Rollback: kembalikan commit dan digest sebelumnya (tercatat di plan), periksa
kompatibilitas migrasi, recreate bot. Rollback image tidak membatalkan migrasi DB.

## Kurs USD/IDR di Telegram

- Sumber: CoinGecko `simple/price?ids=tether&vs_currencies=usd,idr` (harga USDT),
  cadangan Frankfurter `v1/latest?base=USD&symbols=IDR` (kurs ECB, asumsi
  1 USDT ≈ 1 USD). Tanpa API key. Bot menghubungi keduanya lewat resolver `dns`.
- Diperbarui saat start lalu tiap jam. Bila keduanya gagal, kurs terakhir tetap dipakai
  dan log mencatat WARN `exchange rates unavailable, keeping the last ones`. Bila belum
  pernah berhasil, pesan menulis "Kurs rupiah belum tersedia: nilai hanya dalam USDT."
- Hanya untuk tampilan; trading tidak bergantung pada kurs.
- Untuk mematikan, set `telegram.fx: false` di `configs/config.yaml` server lalu
  recreate bot.

## Backup dan restore

`deploy/vps-backup.sh` membuat dump custom PostgreSQL dan arsip config di
`../backups`; timer harian menyimpan 7 terakhir. Uji restore 2026-10-17 lolos
(prosedur di panduan deployment). Backup masih di disk yang sama; salinan di luar
server ditunda sampai live (keputusan user).

## Catatan teknis yang perlu diingat

- Reconnect WS proaktif tiap 23 jam menghasilkan dua WARN normal.
- Host shutdown menghentikan semua container bersamaan; run yang tertinggal ditutup
  `interrupted` saat start berikutnya, dan bot menunggu DB hingga 60 detik saat start.
- `/healthz` hanya memeriksa DB; kesehatan feed dinilai dari `vps-observe.py`.
- Belum ada Alertmanager; peringatan penting datang lewat Telegram (notice level warn).
- `deploy/vps-baseline-check.py` dan `deploy/vps-set-env.py` sudah ada di checkout
  server (sejak `69d8dcb`).
- Sejak `f709429`, restart cepat aman: candle warmup yang masih segar baru dijalankan
  setelah rekonsiliasi pertama (log `strategies warmed up ... held=N`), jadi strategi
  melihat saldo dan order sesi sebelumnya sebelum bertindak.
