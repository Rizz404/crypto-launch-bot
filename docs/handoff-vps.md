# Handoff VPS untuk Claude — 2026-10-17

User meminta pekerjaan dihentikan dan dokumentasi disiapkan untuk take over.
Checkpoint terakhir: **2026-10-17 14:16:57 UTC / 21:16:57 WIB**. Bot paper tetap
berjalan di VPS; tidak ada langkah deployment baru setelah permintaan berhenti.
Fase 9 belum selesai dan uji 24 jam belum dinyatakan dimulai/lulus secara formal.
Baca [plan](plan.md) untuk cakupan lengkap dan [runbook](deployment.md) untuk operasi.

## Status terbaru (Claude, 2026-10-18 15:50 UTC / 22:50 WIB)

- **Observasi paper 24 jam lulus** (lihat plan, "Hasil observasi 24 jam").
- Berjalan: source `eb4c11ba8a836f22006f52e9ea8c2fb5130bd14b`, image
  `ghcr.io/rizz404/trading-bot@sha256:6d051bdacec5a433469ea1f64bf20863b152fc91b3a50938c611278a6c30e3c0`.
- Telegram aktif (token di `.env.vps`, chat ID di `configs/config.yaml` server). `dns` limit 128 MiB.
- Bagian di bawah ini adalah status 2026-10-17; ikuti bagian ini bila bertentangan.

## Status sebelumnya (Claude, 2026-10-17 15:05 UTC / 22:05 WIB)

Bagian setelah ini adalah catatan handoff awal; jika bertentangan, ikuti bagian ini.

- Berjalan: source `1e8e78e2987fdf0d835b036db0e0dfe3c44f45a2`, image
  `ghcr.io/rizz404/trading-bot@sha256:fea8a18b4bcf5f2508c33ca3ea962dae467a074af4c6b0ea4b5ed98115903a7a`
  (CI [39042941107](https://github.com/Rizz404/trading-bot/actions/runs/39042941107)
  sukses). Rilis `cedbfdc`/`e34b989` sudah tercakup. Limit total kini
  ±2,56 GiB RAM / 2,75 CPU (dns 64 MiB / 0,25 CPU).
- Lima container: bot, db, prometheus, grafana, dan **dns** (CoreDNS 1.14.7).
  Bot memakai resolver DNS-over-TLS di network `trading-bot-paper_resolver`
  (172.31.253.0/29) karena resolver host kadang mengembalikan halaman blokir
  Trust Positif untuk `stream.binance.com`. DNS host/Tailscale tidak diubah.
- Restart graceful dengan posisi paper terbuka lolos dua kali (run 1 → 2 → 3).
- Backup: `backups/pre-update-20261017T142721Z` dan `backups/backup-20261017T150314Z`.
  Restore ke DB sementara lolos dan DB uji dihapus. Belum ada salinan di luar VPS.
- Unit `trading-bot-paper-{backup,observe}.{service,timer}` terpasang; kedua timer
  aktif. Sampel ada di `evidence/samples-YYYYMMDD.jsonl`.
- Host reboot tak terduga 15:18–15:42 UTC (bukan dari user maupun sesi ini; pemicu tidak diketahui). Stack pulih otomatis.
- **Observasi 24 jam dihitung ulang dari 15:42:27 UTC** (keputusan user); nilai setelah 2026-10-18 15:43 UTC.
- Baseline 28 container dan 5 service lama tetap tidak berubah setelah semua langkah.
- Izin Claude untuk `ssh -o BatchMode=yes plesk ...` diberikan user lewat
  `.claude/settings.local.json` (lokal, tidak di-commit), terbatas pada project ini.

## Batas yang wajib dipertahankan

- Server `ssh plesk` sudah memiliki Plesk dan banyak proyek produksi. Jangan
  mengubah domain, web server, firewall, Docker daemon, atau resource proyek lain.
  Jangan restart Docker/host, upgrade global, prune, atau `down -v`.
- Semua perubahan hanya untuk project `trading-bot-paper` dalam
  `/opt/trading-bot-paper`. Bandingkan baseline layanan lama sebelum/sesudah
  perubahan berikutnya. Pemeriksaan terakhir: **28 container lama dan 5 service
  tidak berubah**, termasuk waktu mulai/restart/PID yang dicatat.
- Source harus dari GitHub, image dari CI/GHCR berdasarkan digest. Jangan salin
  folder source laptop. Rahasia tidak boleh dicetak, di-commit, atau masuk chat.
- Mainnet order belum diizinkan. VPS hanya paper tanpa kredensial exchange.
  Telegram masih disabled; belum ada pesan sungguhan dikirim.
- Commit Conventional Commits satu baris, satu logical scope per commit,
  tanpa body/footer/Co-Authored-By. User meminta preferensi ini berlaku global.

## Yang sudah benar-benar dikerjakan

Repo private: https://github.com/Rizz404/trading-bot. GHCR login dilakukan sendiri
oleh user di VPS dan akses image sudah diverifikasi. Source di-clone langsung dari
GitHub melalui deploy key read-only khusus repo (GitHub key ID `166765923`).
Host key GitHub diambil melalui API HTTPS GitHub dan disimpan khusus proyek.
Konfigurasi SSH global dan kunci proyek lain tidak diubah.

Susunan direktori server:

```text
/opt/trading-bot-paper/             # mode 700
  access/                         # mode 700
    github-deploy                 # private key tetap di VPS, jangan tampilkan
    github-deploy.pub
    known_hosts                   # khusus proyek, bukan ~/.ssh global
  repo/                           # clone GitHub
    .env.vps                      # mode 600, secret dibuat di VPS
    configs/config.yaml           # contoh config.vps: paper BTC/ETH spot 1m
  evidence/                       # baseline dan pemeriksaan awal
  backups/                        # dibuat, belum ada backup yang dijalankan
```

`repo/.git/config` memiliki `core.sshCommand` khusus deploy key, `IdentitiesOnly`,
`UserKnownHostsFile`, `StrictHostKeyChecking=yes`, dan `BatchMode=yes`.
Server bekerja sebagai root melalui alias SSH tersebut. Tidak perlu mengirim
private key, PAT, atau password ke chat. `.env.vps` berisi tiga password berbeda
untuk DB/Grafana/web; API key Binance dan token Telegram kosong.

Rilis yang **sedang berjalan**:

- Source: `d5a52e2b2f1aa1c5c78f2bbee7c55cb48468643f` (detached HEAD).
- Image: `ghcr.io/rizz404/trading-bot@sha256:7818595501fa20eaa94a4a9ff0de74ad9a19678cd99de11f7e578fe969fec063`.
- CI sukses: https://github.com/Rizz404/trading-bot/actions/runs/39037824371.
- Migrasi 1, 2, 3 berhasil pada DB baru; run paper ID 1 mulai 14:09:38 UTC.

Stack baru terdiri atas bot, TimescaleDB, Prometheus, Grafana. Keempatnya running,
restart count 0 dan OOM false saat checkpoint. Docker host sudah aktif/enabled;
restart policy `unless-stopped` terpasang, tetapi reboot host tidak diuji.
Limit total container 2,5 GiB RAM/2,5 CPU, log 3 × 10 MB per container,
Prometheus retention 7 hari/1 GB blok. Tidak ada hard quota disk total; host disk
sekitar 86% terisi, sekitar 56 GB kosong sebelum pull. Grafana sempat mendekati
limit 512 MiB (sekitar 506 MiB), kemudian turun sekitar 468 MiB tanpa OOM;
pemakaian memori tetap perlu dipantau.

Port host hanya loopback:

| Layanan | Port |
|---|---:|
| Dashboard bot | 18080 |
| Metrics/health bot | 19100 |
| Prometheus | 19090 |
| Grafana | 13000 |

DB tidak punya port host. Web auth sudah diuji 401 tanpa password dan 200 dengan
password. Grafana dashboard UID `tradingbot`, datasource Prometheus/TimescaleDB
keduanya `OK`; keenam alert rules valid/sehat. Belum ada Alertmanager atau
notifikasi eksternal. `/healthz` hanya memeriksa DB, sehingga tidak cukup untuk
membuktikan feed hidup.

## Uji koneksi dan temuan

Backfill awal memuat 2.000 candle. Network hanya container
`trading-bot-paper-bot-1` dilepas dari `trading-bot-paper_default` pada 14:10:45 UTC,
lalu disambungkan kembali dengan alias `bot` dan nama container pada 14:12:10 UTC.
Script uji **sudah selesai**; tidak ada network yang masih sengaja diputus.

Bot mendeteksi idle timeout, mencoba reconnect dengan backoff, kemudian berhasil
sendiri pada **14:15:01 UTC**, sekitar 2 menit 52 detik setelah network dipulihkan.
Backfill 10 candle selesai 14:15:03 UTC. Tidak perlu restart untuk pemulihan ini.
Saat penelusuran, DNS embedded Docker untuk `db`/Binance sudah menjawab, REST
Binance bisa diakses dari network namespace bot, dan handshake WS 9443 berhasil.
Penyebab jeda pemulihan belum dipastikan; jangan menganggap itu bug DNS permanen.

Isolasi network juga memutus DB. Log 14:11:48 UTC memuat
`recorder write failed`, `what=account`, `insert equity: context deadline exceeded`.
Equity berikutnya kembali ditulis. **Audit retry/durabilitas recorder masih
tersisa**, terutama jika kegagalan serupa terjadi pada order/fill. Belum ada
order/fill/trade selama tes ini sehingga ketahanan write order tidak terbukti.

Checkpoint DB: BTCUSDT/ETHUSDT masing-masing 1007 candle, **gap 1m = 0** pada data
yang tersimpan, candle terakhir open 14:15 UTC, equity terbaru 14:16:38 UTC.
Metrics stream connects = 2 dan backfilled candles = 2010 setelah pemulihan.
Bukti read-only tersanitasi: [vps-handoff-2026-10-17.json](testing/vps-handoff-2026-10-17.json).
Di VPS terdapat `evidence/baseline.json`, `existing-services-after.json`,
`deployment-check.json`, `monitoring-check.json`, dan script uji koneksi yang sudah
selesai. File `existing-services-after.json` berasal dari pemeriksaan awal;
JSON handoff di repo memuat pembandingan terakhir.

## Rilis yang sudah siap tetapi BELUM dipasang

- `cedbfdc68389bacd2285239b0fe0a12eb03a7b7b`
  `feat(deploy): schedule isolated paper backups and health samples`.
- `e34b989` `docs(deployment): explain paper backup and observation timers`.
- CI semua job sukses: https://github.com/Rizz404/trading-bot/actions/runs/39039542901.
- Image baru: `ghcr.io/rizz404/trading-bot@sha256:239c7ac671be4337c0927f9346043e369214d2e4676cd33e01dc0466729df837`.

Commit ini menambah `deploy/vps-backup.sh`, `deploy/vps-observe.py`, dan empat unit
di `deploy/systemd/`. Bash/Python lolos pemeriksaan sintaks; belum dijalankan pada
VPS, belum ada backup/dump/restore, dan belum ada unit/timer terpasang. Server
sudah `git fetch origin main` sampai `e34b989`, tetapi checkout tetap `d5a52e2`.
Jangan menyimpulkan CI Go membuktikan script operasi tersebut bekerja; perlu
validasi runtime dan unit systemd pada server sebelum diaktifkan.

## Urutan lanjut yang disarankan

1. Periksa status dan baseline kembali. Tinjau timeout recorder dan jeda reconnect;
   catat temuan di plan. Lakukan tindakan hanya pada stack bot.
2. Buat backup awal DB/config secara privat sebelum update. Untuk deploy rilis
   operasi, checkout commit `cedbfdc...` dari GitHub dan pasang digest baru pada
   `.env.vps` tanpa menimpa password/config. Pertahankan pasangan source/image.
3. Validasi dan jalankan script backup/observasi secara manual. Setelah lolos,
   pasang hanya keempat unit baru yang namanya belum digunakan. Aktifkan dua
   timer sesuai runbook. Tidak perlu restart Docker/Plesk.
4. Uji restart graceful **container bot saja** dan pastikan resume, DB, feed,
   metrics, serta layanan lama tetap benar. Uji posisi/order di VPS belum pernah
   dilakukan; pengujian restart lokal sebelumnya tidak menggantikannya.
5. Catat waktu awal observasi 24 jam setelah kondisi stabil. Periksa candle/gap,
   restart/OOM, error recorder, equity, dan resource. Waktu proses hidup belum
   membuktikan 24 jam sehat. Selesaikan backup di luar server dan uji restore
   terpisah; backup lokal tidak melindungi kehilangan VPS.
6. Lanjutkan sisa Fase 9 di plan: Spot Testnet, Futures Demo (key belum ada),
   Telegram whitelist/ujicoba nyata, audit lanjutan dan validasi strategi. Bybit
   dan xspread ditunda karena regulatory restriction; OKX sudah dihapus.

Perintah baca status dari server:

```sh
cd /opt/trading-bot-paper/repo
dc() { docker compose --env-file .env.vps -f deploy/compose.vps.yml "$@"; }
dc ps
dc logs --tail 50 bot
curl --fail http://127.0.0.1:19100/healthz
curl --silent http://127.0.0.1:19100/metrics | grep '^tradingbot_marketdata'
```

Untuk one-off CLI gunakan `dc run --interactive=false --rm --no-deps bot ...`.
Default interactive dapat menghabiskan stdin bila perintah dijalankan di dalam
script yang dialirkan melalui `ssh ... 'bash -s'`; itu sempat membuat langkah
setelah perintah version tidak terbaca. Migrasi/start kemudian dijalankan secara
terpisah dan berhasil. Gunakan script file atau non-interactive untuk melanjutkan.

Tunnel dari laptop: `ssh -N -L 18080:127.0.0.1:18080 -L 13000:127.0.0.1:13000 plesk`.
Password dibaca sendiri melalui terminal SSH dari `.env.vps`, jangan dicetak oleh
agent. Menutup laptop tidak mematikan container VPS.

Hindsight MCP pada sesi ini masih gagal `Transport closed`; dokumen repo menjadi
handoff yang dapat diandalkan. Jangan menyimpan kredensial dalam shared memory.
