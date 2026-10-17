# Rilis melalui GitHub dan deploy paper di VPS

Kode dan manifest berasal dari repo private `Rizz404/trading-bot`. GitHub Actions
menguji kode, membangun image, lalu menerbitkannya ke GHCR. VPS menarik image hasil
CI; folder kerja laptop tidak disalin ke server. Deploy pertama adalah paper Spot.
Fase 9 tetap berjalan dan mainnet tetap memerlukan persetujuan Fase 11.

**Checkpoint 2026-10-17:** deployment awal paper sudah berjalan, tetapi langkah
backup/timer di panduan ini belum dipasang pada VPS. Lihat [handoff VPS](handoff-vps.md)
untuk commit/image yang benar-benar aktif, hasil uji koneksi, dan pekerjaan tersisa.
Panduan ini menjelaskan prosedur; bukan pernyataan bahwa semua langkah sudah dilakukan.

## Pipeline

`.github/workflows/ci.yml` berjalan pada push ke `main`, pull request, dan pemicu
manual. Push yang hanya mengubah `docs/` atau README tidak membangun ulang image.
Sebelum menerbitkan image, pipeline menjalankan:

- Format, verifikasi modul, vet, golangci-lint, unit test dengan race detector,
  serta build ketiga CLI.
- Test database, siklus migrasi up/down/up, dan paper restart secara berurutan
  pada database TimescaleDB sementara `tradingbot_ci_test`.
- Test alert Prometheus tanpa akses jaringan container.
- Build image amd64 dan smoke test ketiga CLI tanpa akses jaringan.

Publikasi hanya untuk `main` pada push atau pemicu manual. PR membangun image
tanpa publikasi. Image mendukung `linux/amd64` dan `linux/arm64`; Actions dipin ke
commit SHA. Tag image adalah `sha-<full commit SHA>`. Ringkasan job container
menampilkan referensi lengkap `ghcr.io/rizz404/trading-bot@sha256:...`; gunakan
digest tersebut untuk deploy. Jangan memakai tag bergerak seperti `latest`.

Actions menggunakan `GITHUB_TOKEN` bawaan dengan izin `contents: read` dan
`packages: write` hanya pada job container. API key exchange dan token Telegram
tidak diperlukan di CI. Repo dan paket GHCR harus tetap private.

## Persiapan server

Periksa kapasitas, arsitektur CPU, aplikasi yang sudah berjalan, akses Binance
REST/WS, dan port yang tersedia sebelum mengubah server. Panduan berikut memakai
Docker dengan Compose v2; konfigurasi service saat boot disesuaikan dengan OS dan
runtime server. Sediakan akses baca repo (misalnya SSH deploy key) serta akses
baca paket GHCR private. Untuk GHCR, gunakan token khusus dengan `read:packages`
melalui prompt `docker login ghcr.io --username Rizz404`, bukan lewat argumen
password atau chat. Jangan menaruh token itu di repo.

### Server Plesk bersama

Target saat ini adalah alias SSH `plesk`, yang sudah menjalankan Plesk dan proyek
lain. Pertahankan konfigurasi Plesk, web server, domain, firewall, Docker daemon,
serta container/network/volume proyek lain. Jangan melakukan upgrade global,
restart Docker, `docker system prune`, atau perubahan pada stack lain sebagai
bagian deployment ini. Pilih direktori baru yang belum digunakan (calon:
`/opt/trading-bot-paper`) dan project Compose khusus `trading-bot-paper`.

Manifest membatasi DB ke 1 GiB/1 CPU dan bot, Prometheus, Grafana masing-masing
512 MiB/0,5 CPU: total batas RAM 2,5 GiB dan CPU 2,5 core. TimescaleDB ditune
untuk limit container, bukan total RAM host. Prometheus dibatasi retention 7 hari
atau 1 GB data blok; WAL dan overhead bisa menambah pemakaian di luar angka itu.
Log dibatasi 3 × 10 MB per container. Ini bukan kuota disk total; database,
backup, dan image tetap perlu dipantau.

Sebelum dan sesudah deployment, bandingkan status, waktu mulai, dan restart count
container lama serta status service Plesk/web server. Jika terjadi masalah pada
stack baru, hentikan hanya project `trading-bot-paper` untuk membebaskan resource;
jangan mengubah proyek lama. Validasi port loopback masih tersedia tepat sebelum
start dan jangan mengganti port aplikasi lain untuk membuat ruang bagi bot.

Clone di server dari GitHub dan checkout commit yang sama dengan image:

```sh
git clone git@github.com:Rizz404/trading-bot.git
cd trading-bot
git checkout --detach <full-commit-sha>
```

Pada instalasi pertama saja, buat konfigurasi lokal berikut. Jangan menimpa file
ini saat update:

```sh
install -m 600 deploy/vps.env.example .env.vps
install -m 644 configs/config.vps.example.yaml configs/config.yaml
```

Edit `.env.vps`: isi `BOT_IMAGE` dengan digest dari Actions, lalu buat tiga
password berbeda untuk database, Grafana, dan dashboard. Gunakan password database
berupa hex (misalnya hasil `openssl rand -hex 24`) karena dipasang dalam URL DB.
Paper tidak memerlukan kredensial exchange. Telegram baru diaktifkan setelah
token dan chat tujuan sudah siap; isi whitelist `telegram.chat_ids` di YAML.
Konfigurasi contoh memakai BTCUSDT/ETHUSDT Spot dan strategi EMA 1m untuk uji
operasional, bukan parameter yang sudah terbukti menguntungkan.

Manifest VPS tidak memiliki `build:`. Ia memaksa `TB_APP__ALLOW_LIVE=false`, memakai
volume database/monitoring persisten, membatasi log, dan hanya membuka port host
di loopback. Mode default adalah `paper`. Mode `testnet` bisa diuji terpisah dengan
key Spot Testnet; Futures masih memerlukan konfigurasi dan key Futures Demo yang
terpisah. Jangan menjalankan dua proses pada sesi/DB paper yang sama.

## Start dan pemeriksaan

Buat fungsi terminal berikut dari root repo:

```sh
dc() { docker compose --env-file .env.vps -f deploy/compose.vps.yml "$@"; }
```

Lalu tarik image, mulai infrastruktur, dan jalankan migrasi sebelum bot:

```sh
dc pull
dc up -d db prometheus grafana
dc run --rm --no-deps bot migrate up
dc up -d bot
dc ps
dc logs --tail 100 bot
curl --fail http://127.0.0.1:19100/healthz
```

Health DB saja belum membuktikan feed/strategi aktif. Pastikan log menyebut mode
paper, versi image benar, candle terus masuk, metrics Prometheus sehat, dan panel
Grafana terisi. Uji shutdown/restart, backfill setelah putus koneksi, konsistensi
state, notifikasi Telegram, dan uptime 24 jam. `restart: unless-stopped` menangani
crash; layanan runtime container juga harus aktif saat boot server.

Dashboard bisa dibuka melalui tunnel dari laptop, tanpa domain:

```sh
ssh -N -L 18080:127.0.0.1:18080 -L 13000:127.0.0.1:13000 <alias-vps>
```

Dashboard: `http://127.0.0.1:18080`; Grafana: `http://127.0.0.1:13000`.
Sesuaikan port jika sudah dipakai layanan lain. Bot di VPS tetap berjalan ketika
tunnel atau laptop ditutup.

## Backup, update, dan rollback

Target Plesk menggunakan `/opt/trading-bot-paper/repo`. Kunci deploy read-only dan
`known_hosts` khusus proyek berada di direktori saudara `access/`; jangan menyalin
private key dari laptop atau mengubah konfigurasi SSH global. Password lokal
dibuat di VPS pada `.env.vps` (mode `600`). Untuk membaca password dashboard saat
dibutuhkan, buka file itu sendiri melalui terminal SSH; jangan tempel isinya ke
chat atau log.

Script `deploy/vps-backup.sh` membuat dump custom PostgreSQL beserta arsip
konfigurasi rahasia di `/opt/trading-bot-paper/backups`. Tujuh backup sukses
terbaru dipertahankan. Script memvalidasi daftar isi dump dan checksum; validasi
tersebut belum menggantikan uji restore penuh. Backup lokal tetap mengandung
secret, sehingga direktori harus `700` dan file `600`.

`deploy/vps-observe.py` mencatat status container, health, datasource monitoring,
metrics, waktu candle, gap 1m, equity terbaru, serta disk kosong setiap lima menit
ke `evidence/samples-YYYYMMDD.jsonl`. Retention sampel 14 hari. Pemeriksaan gap ini
khusus konfigurasi paper dua seri 1m; sesuaikan script bila universe/timeframe
diubah. Sampel ini bukan pemantau eksternal dan tidak mengirim notifikasi.

Unit systemd di `deploy/systemd/` hanya untuk project baru ini. Sebelum memasang,
pastikan nama unit belum digunakan. Setelah bot sehat, pasang unit lalu aktifkan
dua timer berikut (tidak perlu me-restart Docker/Plesk):

```sh
install -m 644 deploy/systemd/trading-bot-paper-* /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now trading-bot-paper-observe.timer trading-bot-paper-backup.timer
systemctl start trading-bot-paper-observe.service trading-bot-paper-backup.service
systemctl list-timers 'trading-bot-paper-*'
```

Timer backup berjalan setiap hari sekitar 03:15 UTC (10:15 WIB); backup yang
terlewat saat server mati dijalankan setelah boot. Setiap service memiliki limit
CPU/memori dan prioritas rendah. Periksa hasil melalui `journalctl -u
trading-bot-paper-observe.service` serta `journalctl -u
trading-bot-paper-backup.service`. Gagal health hanya mencatat temuan, tanpa
mengubah stack lain atau melakukan pemulihan otomatis.

Sebelum update, simpan digest dan commit lama serta backup database:

```sh
mkdir -p backups
chmod 700 backups
dc exec -T db pg_dump -U bot -d tradingbot -Fc > backups/before-update.dump
chmod 600 backups/before-update.dump
```

Backup di disk VPS yang sama tidak melindungi dari kehilangan server. Salinan di
luar server dan uji restore penuh tetap perlu disiapkan. Jangan gunakan `down -v`
pada stack yang berisi data yang ingin dipertahankan.

Untuk update: hentikan bot secara graceful, checkout commit baru dari GitHub,
ubah `BOT_IMAGE` ke digest yang lolos CI, tarik image, jalankan migrasi, lalu
mulai bot dan ulangi pemeriksaan kesehatan. Infrastructure tidak perlu dihentikan.
Untuk rollback, kembalikan commit manifest dan digest image sebelumnya; periksa
kompatibilitas skema sebelum menjalankan image lama. Rollback image tidak otomatis
mengembalikan migrasi database.

Referensi: [publikasi image dengan Actions](https://docs.github.com/en/actions/tutorials/publish-packages/publish-docker-images),
[akses registry dan digest](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry),
[cross-compilation multi-platform](https://docs.docker.com/build/building/multi-platform/).
