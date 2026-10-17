# Rilis melalui GitHub dan deploy paper di VPS

Kode dan manifest berasal dari repo private `Rizz404/trading-bot`. GitHub Actions
menguji kode, membangun image, lalu menerbitkannya ke GHCR. VPS menarik image hasil
CI; folder kerja laptop tidak disalin ke server. Deploy pertama adalah paper Spot.
Fase 9 tetap berjalan dan mainnet tetap memerlukan persetujuan Fase 11.

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

Sebelum update, simpan digest dan commit lama serta backup database:

```sh
mkdir -p backups
chmod 700 backups
dc exec -T db pg_dump -U bot -d tradingbot -Fc > backups/before-update.dump
chmod 600 backups/before-update.dump
```

Cadangkan juga konfigurasi lokal secara aman di luar repo. Backup di disk VPS
yang sama tidak melindungi dari kehilangan server; jadwal dan salinan di luar
server disiapkan saat target VPS sudah diketahui. Jangan gunakan `down -v` pada
stack yang berisi data yang ingin dipertahankan.

Untuk update: hentikan bot secara graceful, checkout commit baru dari GitHub,
ubah `BOT_IMAGE` ke digest yang lolos CI, tarik image, jalankan migrasi, lalu
mulai bot dan ulangi pemeriksaan kesehatan. Infrastructure tidak perlu dihentikan.
Untuk rollback, kembalikan commit manifest dan digest image sebelumnya; periksa
kompatibilitas skema sebelum menjalankan image lama. Rollback image tidak otomatis
mengembalikan migrasi database.

Referensi: [publikasi image dengan Actions](https://docs.github.com/en/actions/tutorials/publish-packages/publish-docker-images),
[akses registry dan digest](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry),
[cross-compilation multi-platform](https://docs.docker.com/build/building/multi-platform/).
