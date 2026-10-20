# Rilis melalui GitHub dan deploy ke VPS

Kode dan manifest berasal dari repo private `Rizz404/trading-bot`. GitHub Actions
menguji kode, membangun image, lalu menerbitkannya ke GHCR. VPS menarik image hasil
CI; folder kerja laptop tidak disalin ke server. Stack VPS bisa berjalan dalam mode
`paper` atau `testnet` (lihat "Mode dan Telegram"); mainnet memerlukan persetujuan
P2-10 pada [Plan 2](plan-2.md), kelanjutan Fase 11 Plan 1.

Panduan ini menjelaskan prosedur. Keadaan server saat ini (commit, digest, mode,
uji yang berjalan) ada di [handoff VPS](handoff-vps.md).

## Hubungan dengan Plan 2

[Plan 1](plan.md) ditutup; pengembangan berikutnya mengikuti
[Plan 2](plan-2.md). [Multi-instance](multi-instance.md) berarti pembagian
portofolio virtual oleh satu koordinator akun, bukan menjalankan beberapa
`bot run` independen pada wallet/database yang sama. Manifest dan CLI pada
panduan ini masih milik implementasi sekarang; dukungan multi-instance belum
tersedia. Migrasi baru diuji pada salinan DB sebelum P2-9; rencana ini tidak
mengizinkan memulai ulang testnet atau live.

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

### Resolver DNS bot

Di VPS Indonesia, resolver host bisa menjawab domain exchange (misalnya
`stream.binance.com`) dengan alamat halaman blokir Trust Positif, dan ISP juga
mencegat DNS ke 8.8.8.8/9.9.9.9. Atas keputusan user, service `dns` (CoreDNS,
[Corefile](../deploy/coredns/Corefile)) meneruskan query bot lewat DNS-over-TLS ke
Cloudflare dengan verifikasi sertifikat. Service ini hanya terhubung ke network
`resolver` (172.31.253.0/29, IP tetap 172.31.253.2) dan hanya dipakai bot; DNS embedded
Docker tetap menjawab nama service (`db`). DNS host dan service lain tidak diubah.
Pilih subnet lain bila subnet itu sudah dipakai di server. Untuk memeriksa jawaban:

```sh
nsenter -t "$(docker inspect -f '{{.State.Pid}}' trading-bot-paper-dns-1)" -n \
  dig +short @127.0.0.1 A stream.binance.com
```

Jawaban `139.255.196.196`/`182.23.79.195` (`lamanlabuh.aduankonten.id`) berarti
masih terblokir. Error WS menyebut tahap dial (`stage dns|connect|tls|upgrade`).
Binary CoreDNS memakai file capability port 53, sehingga service berjalan sebagai
UID 0 dengan `cap_drop: ALL`, hanya `NET_BIND_SERVICE`, read-only, dan
`no-new-privileges`. Saat update, nyalakan `dns` dan periksa jawabannya sebelum
me-recreate bot.

## Mode dan Telegram

Mode dipilih lewat `TB_APP__MODE` di `.env.vps` (manifest meneruskannya; default
`paper`) dan harus cocok dengan `configs/config.yaml`:

- `paper`: [config.vps.example.yaml](../configs/config.vps.example.yaml). Tanpa
  kredensial exchange; order disimulasikan dengan harga mainnet.
- `testnet`: [config.testnet.example.yaml](../configs/config.testnet.example.yaml)
  (lima strategi spot, satu per pair) plus kredensial Binance Spot Testnet di
  `TB_EXCHANGES__BINANCE__API_KEY`/`_SECRET`. Data disimpan dengan nama exchange
  `binance-testnet`, terpisah dari data paper/mainnet. Testnet tanpa fee dan riwayat
  candle-nya hanya sekitar dua minggu.

Telegram: `TB_TELEGRAM__TOKEN` di `.env.vps`, lalu `telegram.enabled: true` dan
`telegram.chat_ids` di `configs/config.yaml` server (chat ID tidak masuk repo).
Pesan berbahasa Indonesia; daftar perintah lewat `/help`.

Setiap nominal USDT di pesan disertai perkiraan USD dan rupiah, misalnya
`+4.80 USDT (≈ +$4.80 · +Rp 86.400)`, beserta baris kurs (sumber dan jam WIB). Bot
mengambil kurs tiap jam dari CoinGecko (`api.coingecko.com`), dengan cadangan
Frankfurter (`api.frankfurter.dev`, kurs ECB, asumsi 1 USDT ≈ 1 USD). Keduanya tanpa
API key, tetapi container bot harus bisa menjangkaunya. Bila gagal, kurs terakhir
tetap dipakai; tanpa kurs sama sekali, pesan hanya memakai USDT. Kurs tidak
memengaruhi trading. Matikan dengan `telegram.fx: false`.

Semua variabel di atas, juga `BOT_IMAGE`, dipasang dengan
[`deploy/vps-set-env.py`](../deploy/vps-set-env.py), yang membaca `KEY=VALUE` dari
stdin SSH dan tidak pernah mencetak nilainya. Skrip ini hanya menerima `TB_*` dan
`BOT_IMAGE`:

```sh
grep -E '^TB_TELEGRAM__TOKEN=' .env | ssh plesk 'python3 /opt/trading-bot-paper/repo/deploy/vps-set-env.py'
```

Perubahan mode, config, atau `.env.vps` berlaku setelah container bot di-recreate
(`dc up -d --no-deps bot`). Sebelum dan sesudahnya, jalankan
[`deploy/vps-baseline-check.py`](../deploy/vps-baseline-check.py) di server bersama.

## Start dan pemeriksaan

Buat fungsi terminal berikut dari root repo:

```sh
dc() { docker compose --env-file .env.vps -f deploy/compose.vps.yml "$@"; }
```

Lalu tarik image, mulai infrastruktur, dan jalankan migrasi sebelum bot:

```sh
dc pull
dc up -d db dns prometheus grafana
dc run --interactive=false --rm --no-deps bot migrate up
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

Uji restore (lolos 2026-10-17) memakai DB sementara di container DB yang sama,
lalu menghapusnya. TimescaleDB memerlukan pre/post restore:

```sh
b=../backups/backup-<waktu>
q() { dc exec -T db psql -U bot -d "$1" -v ON_ERROR_STOP=1 -c "$2" </dev/null; }
q postgres "CREATE DATABASE tradingbot_restore_test"
q tradingbot_restore_test "CREATE EXTENSION IF NOT EXISTS timescaledb"
q tradingbot_restore_test "SELECT timescaledb_pre_restore()"
dc exec -T db pg_restore -U bot -d tradingbot_restore_test --exit-on-error < "$b/database.dump"
q tradingbot_restore_test "SELECT timescaledb_post_restore()"
# Bandingkan jumlah baris candles/orders/fills/strategy_runs dengan DB live, lalu:
q postgres "DROP DATABASE tradingbot_restore_test"
```

Peringatan pg_dump tentang `circular foreign-key constraints` pada `continuous_agg`
berasal dari katalog TimescaleDB dan tidak menggagalkan restore di atas.

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

## Rilis runtime managed Plan 2

Sudah di-deploy sebagai trial P2-9 (DB terpisah via `BOT_DB_NAME`, lihat
[handoff VPS](handoff-vps.md)). Ikuti [panduan managed](managed-runtime.md)
untuk migrasi 4 pada DB salinan, lease, review legacy dan rollback. Binary VPS
lama tidak mengenal lease baru. Jangan menjalankan worker lama dan managed
bersamaan pada akun/mode yang sama. Jangan down migration pada DB dengan history
managed. Trial baru memakai database baru (`CREATE DATABASE`, `BOT_DB_NAME`,
`bot migrate up`) agar DB lama tetap utuh; backup mencakup semua DB proyek.
Keputusan mainnet tetap P2-10 dan belum disetujui.
