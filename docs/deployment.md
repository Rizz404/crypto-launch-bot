# Deployment

Prosedur rilis dan operasi untuk project `trading-bot-paper`. Status server ada
di [handover VPS](handover-vps.md); **paper pembanding tahap dua aktif** sejak
22 Oktober 2026 19:58:45 WIB. Panduan ini tidak memberi izin trial tambahan
atau transaksi mainnet dengan uang asli.

Rencana aktif [Plan 3](plan-3.md); Plan 2 sudah diarsipkan. Strategi AI belum
memiliki deployment. P3-7 memerlukan kesiapan/izin start sendiri, DB/wallet baru
dan pemeriksaan resource collector; runbook ini tidak mengizinkan perubahan
trial pembanding hanya karena plan berganti.

## Rencana penempatan Spider

**28 Oktober — izin terbaru:** user mengizinkan commit, push dan deploy.
Rilis awal hanya memasang komponen CLI/image dan menjalankan default `report`
sekali di proyek P3 terpisah. Runner/pilot trading belum tersedia/diterima.
CI kini menyiapkan publikasi `ghcr.io/rizz404/trading-bot-catalyst:sha-<commit>`
setelah checks/integration/monitoring serta smoke/protokol khusus lulus.
Pasang hasil CI melalui digest tetap; jangan mengganti runtime P2.

User menetapkan Spider sebagai collector utama P3 pada 26 Oktober. Inspeksi
kapasitas VPS hanya baca sudah dilakukan; rencana awal memakai project/path
`trading-bot-catalyst` / `/opt/trading-bot-catalyst` yang terpisah dari P2.
`Dockerfile.catalyst` dan `deploy/compose.catalyst.yml` disiapkan 28 Oktober,
build/smoke image linux/amd64 serta binary/protokol fixture tanpa jaringan lulus.
CI job image/protokol disiapkan tanpa publikasi, belum dijalankan di GitHub;
belum ada publikasi image khusus, instalasi, rilis atau start P3 di VPS.
`podman compose ... config` lulus untuk manifest khusus;
ini validasi konfigurasi, bukan menjalankan container. Jangan memasang draft
langsung ke runtime P2. [Handover Spider](catalyst-spider-handover.md)
memuat pekerjaan dan bukti yang harus diselesaikan sebelum rilis.

Image khusus memakai crate CLI tepat `2.53.9`, checksum terverifikasi dan
Cargo.lock upstream (`--locked`), features `headers,status_code` dan tanpa
default Chrome/cloud. Patch `build/spider-cli.patch` mem-flush stdout sebelum
shutdown agar halaman terakhir tidak hilang; binary mengaku versi `2.53.9-p3.1`
dan collector menolak versi tanpa patch. Compose meminta
`CATALYST_IMAGE` dari image **khusus P3** yang kelak dipublikasikan dengan digest;
image bot P2 tidak menyertakan Spider. Default adalah `report`, volume arsip
sendiri, non-root/read-only, tanpa port/restart otomatis, 0,5 CPU/512 MiB/pids
128. File ini tidak memasang worker/model/DB atau mengaktifkan pilot.
Sebelum rilis: uji egress adversarial/situs publik, resource dan batas container,
lalu jalankan CI dan lengkapi publikasi GHCR/digest khusus P3. Dua halaman fixture
berhasil dalam budget, tetapi belum membuktikan kapasitas situs/beban pilot.

## Rilis dan persiapan

- Source dari repo private `Rizz404/trading-bot`; image dari CI/GHCR dengan
  pasangan commit dan digest tetap. Jangan salin source laptop atau pakai `latest`.
- CI memeriksa format, modul, vet/lint, race test, build lima CLI, integrasi DB/
  migrasi/restart, alert Prometheus dan smoke image. Publikasi hanya dari `main`;
  PR membangun tanpa publikasi. Rilis mendukung amd64/arm64.
- Binary catalyst ditambahkan untuk development/report/replay; entrypoint bot
  dan manifest P2 tidak berubah. Adapter AI/runner prospective belum diterima;
  adanya binary di image bukan izin mengaktifkan worker baru.
- Gunakan deploy key read-only dan akses GHCR `read:packages`. Jangan mencetak
  token/password atau memasukkannya ke argumen perintah, repo maupun chat.
- VPS Plesk bersama: hanya `/opt/trading-bot-paper`, project Compose
  `trading-bot-paper`. Jangan mengubah service/port proyek lain, Plesk, firewall,
  DNS host atau Docker daemon; jangan restart host/Docker, prune atau `down -v`.
- Periksa resource/disk, port loopback, subnet resolver dan baseline sebelum
  perubahan. Jika baseline historis berbeda, simpan penyebab/selisih dan snapshot
  baru untuk perbandingan sebelum/sesudah; jangan menghapus bukti lama.

Instalasi pertama membuat `.env.vps` mode 600 dan `configs/config.yaml` dari
contoh VPS. Jangan menimpanya saat update. Password DB, dashboard dan Grafana
harus berbeda; password DB hex agar aman dalam URL. Direktori backup mode 700,
file backup 600. Config bind-mount harus terbaca UID container 65532;
trial saat ini memakai owner 65532:65532 mode 600. File env tetap root mode 600.
Paper memakai harga publik tanpa key perdagangan.

| Pengaturan | Penggunaan |
|---|---|
| `BOT_IMAGE` | `ghcr.io/rizz404/trading-bot@sha256:<digest>` dari CI |
| `BOT_DB_NAME` | Database trial yang tepat; trial baru menggunakan DB baru |
| `TB_APP__MODE` | `paper` / `testnet`, harus cocok dengan config |
| `TB_APP__ALLOW_LIVE` | Dipaksa `false` oleh manifest; live memerlukan P3-LIVE |
| `TB_TELEGRAM__TOKEN` | Secret via environment; whitelist di YAML server |
| `TB_WEB__PASSWORD` | Auth dashboard; jangan buka port publik |

Pasang env melalui `deploy/vps-set-env.py` (stdin SSH), menerima `TB_*`,
`BOT_IMAGE`, `BOT_DB_NAME`. Telegram hanya satu long-poller per token; `/help`
menjelaskan kontrol. Kurs USDT/USD/IDR berasal CoinGecko dengan fallback
Frankfurter, hanya tampilan dan bukan input trading.

## Compose dan resource

Semua perintah berikut dari `/opt/trading-bot-paper/repo` di VPS:

```sh
dc() { docker compose --env-file .env.vps -f deploy/compose.vps.yml "$@" </dev/null; }
```

DB 1 GiB/1 CPU; bot 512 MiB/1 CPU; Prometheus dan Grafana masing-masing 512 MiB/0,5 CPU;
DNS 128 MiB/0,25 CPU. Batas aktual mengikuti manifest. Log 3 × 10 MB/container;
Prometheus retensi 7 hari/1 GB tidak membatasi seluruh pemakaian disk/WAL.
DB/monitoring memakai volume persisten. Port loopback: web 18080, metrics 19100,
Prometheus 19090, Grafana 13000. DB tidak diekspos ke host.

CoreDNS melayani bot melalui DoT Cloudflare, network `172.31.253.0/29`, IP
`172.31.253.2`; Docker tetap meresolusi service `db`. Pastikan subnet kosong pada
instalasi baru. CoreDNS memakai root dengan hanya capability `NET_BIND_SERVICE`,
read-only dan `no-new-privileges` karena file capability binary port 53.
Periksa resolver sebelum mulai bot:

```sh
nsenter -t "$(docker inspect -f '{{.State.Pid}}' trading-bot-paper-dns-1)" -n dig +short @127.0.0.1 A stream.binance.com
```

Jawaban `139.255.196.196`/`182.23.79.195` berarti halaman blokir. Jangan
mengganti resolver global sebagai perbaikan. Log WS mencatat tahap
`dns|connect|tls|upgrade`.

## Update atau trial baru yang telah diizinkan

1. Catat source/digest/config hash, modal/ledger, mode dan waktu; jalankan
   `python3 deploy/vps-baseline-check.py`, simpan selisih yang sudah ada.
2. Buat backup `bash deploy/vps-backup.sh`. Hentikan bot lama secara graceful;
   finish lebih dahulu jika eksperimen harus menyelesaikan order/inventori.
3. Fetch dari GitHub dan checkout commit yang cocok dengan digest CI.
   Update env melalui helper, tanpa mencetak file rahasia; `dc pull`.
4. Uji migrasi pada DB salinan. Trial baru memakai DB/wallet baru dengan modal
   eksplisit, bukan resume saldo lama. Set `BOT_DB_NAME` sebelum migrasi target.
5. Jalankan `dc run --interactive=false --rm --no-deps bot migrate status`,
   lalu `migrate up` hanya pada target yang sudah diperiksa. Managed perlu
   migrasi 4/5; jangan down pada DB berisi history/dana.
6. Mulai infrastruktur yang diperlukan, cek DNS, baru mulai bot. Jangan
   menjalankan worker legacy dan managed bersamaan pada wallet/mode yang sama.
7. Periksa source/digest log, mode paper, saldo awal, indikator siap, feed 1m/4h,
   order/proteksi, persistence, Telegram dan resource. `/healthz` hanya cek DB.
   Bandingkan baseline/snapshot proyek lain sesudah perubahan.
8. Catat waktu mulai aktual dan hasil pemeriksaan. Parameter dibekukan;
   perubahan strategi mendapat versi eksperimen baru, tanpa promosi live otomatis.

Config managed bootstrap diberi hash; mengedit YAML lama lalu resume bisa
menghasilkan ketidakcocokan. Ikuti [runtime managed](managed-runtime.md).
Testnet memerlukan key Spot Testnet dan config khusus; fee testnet nol bukan
model biaya live. Futures memerlukan key/config Demo terpisah dan masih ditunda.

## Backup, restore dan rollback

`deploy/vps-backup.sh` menyimpan semua DB proyek beserta config privat, commit,
daftar arsip dan checksum; mempertahankan tujuh backup sukses terbaru. Dump
utama `database.dump`, trial lain `database-<nama>.dump`. Validasi daftar isi dan
checksum tidak menggantikan restore drill. Backup satu server tidak melindungi
kehilangan VPS; salinan luar server ditunda user sampai live.

Restore hanya ke DB sementara yang diizinkan, tanpa menimpa trial. Untuk
TimescaleDB: buat DB/extension, panggil `timescaledb_pre_restore()`, jalankan
`pg_restore --exit-on-error`, lalu `timescaledb_post_restore()`. Bandingkan
schema/jumlah baris/ledger dengan sumber, baru hapus DB sementara. Peringatan
circular foreign-key pada katalog TimescaleDB pernah muncul; jangan menganggap
restore lulus hanya karena warning tersebut dikenal. Bukti historis ada di
[operasi VPS](testing/vps-ops.json) dan [start P2-9](testing/p2-9-vps-start.json).

Rollback mengembalikan commit dan image digest yang cocok **setelah** memeriksa
kompatibilitas schema, menghentikan pemilik wallet baru dan menyelesaikan order
managed. Rollback image tidak mengembalikan DB. Jangan down migrasi 4/5 yang
sudah menyimpan history. Referensi rilis lama ada di [handover VPS](handover-vps.md).

## Timer dan observasi

Unit `deploy/systemd/trading-bot-paper-*` hanya untuk proyek ini. Observe mencatat
sampel tiap lima menit, retensi 14 hari; backup harian sekitar 03.15 UTC dan
`Persistent` menangani jadwal yang terlewat. Unit membatasi CPU/memori.

`vps-observe.py` memeriksa container, metrics, feed, monitoring dan disk; hasil
bukan jaminan akuntansi/order benar dan tidak melakukan recovery otomatis.
Periksa aturan gap/universe bila config berubah. Detail kesalahan ada di
`journalctl -u trading-bot-paper-observe.service` atau service backup.

Saat trial selesai, matikan observe timer; pertahankan backup serta DB sesuai
kebutuhan. Status saat ini: observe dan backup active untuk trial riset. Jangan mengaktifkan
ulang keduanya secara membabi buta dari runbook lama.
