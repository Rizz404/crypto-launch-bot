# Handover VPS

Checkpoint terverifikasi **21 Oktober 2026 20.30 WIB**. Prosedur perubahan ada di
[deployment](deployment.md); keputusan aktif di [Strategi 2](strategy-2.md).

## Status terakhir

| Komponen | Kondisi |
|---|---|
| Bot ETH paper P2-9 | Stopped/exited 0 sejak 21 Okt 13:30:39 UTC, atas permintaan user |
| Ledger | `paper:eth-research`, instance `momentum`, binding `momentum/eth4h` |
| Modal | 110,79104808331486 USDT; posisi, order, eksekusi dan arsipnya kosong |
| Database trial | `tradingbot_eth`; DB testnet `tradingbot` tetap dipertahankan |
| Source | `c14aa087141583a9f185c5b8b5f595009988c41c` |
| Image digest | `sha256:5a50dd484438c0739093b6417622bfea35c11f96fd9fb815ad3e6976c3333424` |
| Backup final | `backup-20261021T133042Z` |
| Observe timer | Disabled/inactive |
| Backup timer | Active, harian sekitar 03.15 UTC / 10.15 WIB |
| DB, DNS, Prometheus, Grafana | Running |

Jadwal laporan hari 1/3/7 dan review tujuh hari **dibatalkan**. Jangan resume
trial atau memakai ledger ini sebagai eksperimen baru. Status instance tersimpan
masih `running` karena yang dihentikan prosesnya; ini bukan status container.

Bukti: [start](testing/p2-9-vps-start.json), [stop](testing/p2-9-stop.json),
[review](testing/p2-9-review.json). Trial dihitung sejak 20 Okt 16:24:45 UTC;
start sebelumnya mengalami crash loop warmup dan tidak dihitung.

## Temuan terbuka

### Permintaan pengujian berikutnya dan review hari 1/3/7

Pada 21 Oktober 2026 user meminta commit semua perubahan, memulai "tes live",
dan akan kembali melapor pada hari 1, 3 dan 7. **Arti live sedang diklarifikasi:
paper VPS dengan uang virtual atau mainnet dengan uang asli.** Belum ada start,
deploy, push, perubahan kredensial/mode, atau pembukaan order dari permintaan
ini. Jangan menyimpulkan demo Telegram adalah trial yang sudah berjalan.

Rancangan 24 instance/72 binding belum runnable: rotasi, squeeze, mean reversion,
antrean VPS dan guard kapasitas masih pekerjaan terbuka. Jumlah itu plafon
bertahap, bukan konfigurasi siap diaktifkan. Jangan membuat strategi pengganti
dengan nama yang sama atau memakai ledger trial ETH lama untuk mempercepat start.

Sebelum start, catat mode yang dikonfirmasi, strategi/koin/aturan yang benar-benar
siap, modal total/alokasi, batas risiko, database baru, source commit, image
digest, config hash, baseline proyek lain, kapasitas dan temuan operasi terbuka.
Rilis mengikuti [deployment](deployment.md); commit lokal tidak berarti sudah
di-push, diterbitkan CI atau dipasang di VPS. Mainnet memerlukan modal riil,
pilihan strategi dan batas kerugian yang eksplisit; jangan memakai Rp105 juta
virtual atau batas paper sebagai keputusan uang asli.

Jadwal dihitung dari **T0 = start trial baru yang benar-benar diverifikasi**,
bukan waktu commit, pengiriman demo atau start trial ETH lama:

| Review user | Waktu relatif | Fokus agent saat user kembali |
|---|---|---|
| Hari 1 | T0 + 24 jam | Proses/feed, gap, warmup, restart, ledger/order, proteksi, Telegram dan beban CPU/RAM/DB |
| Hari 3 | T0 + 72 jam | Keandalan berkelanjutan, fee/PnL, reservasi, pemulihan, backup dan perubahan beban |
| Hari 7 | T0 + 168 jam | Rekap operasi, seluruh transaksi/biaya, drawdown, jeda tanpa entry, kualitas data dan keputusan lanjut/henti |

User boleh melapor terlambat; catat durasi aktual. Tidak memasang jadwal AI,
polling berkala atau perubahan otomatis di antara laporan tanpa permintaan.
Observasi layanan yang disepakati saat start boleh mengumpulkan bukti, tetapi
bukan alasan agent mengubah parameter/modal atau memulai ulang proses sendiri.
Review tujuh hari bukan bukti profit konsisten atau promosi mode/modal otomatis.

Minta user melapor lebih cepat bila start/stop berulang, peringatan saldo/order
pelindung/`perlu diperiksa`, atau ringkasan 07.00 WIB hilang dua hari berturut.
Pada review, cocokkan bukti VPS dengan Telegram dan timestamp harga; pesan
"proses aktif" atau `healthz` sendiri tidak membuktikan seluruh feed sehat.
Jangan memaksa entry jika sinyal belum muncul. Tanggal kalender review baru diisi
setelah T0 tercatat; saat ini belum ada jadwal trial baru yang dimulai.

### Temuan trial sebelumnya

- Bot dan 20 container proyek lain mulai kembali sekitar 21 Okt 03:23:55 UTC
  sebelum tindakan penutupan. Penyebab belum dibuktikan; jangan menyebutnya
  reboot host. Snapshot proyek lain sebelum/sesudah penutupan identik.
- Baseline historis 17 Oktober karena itu tidak lagi `differences: []`.
  Simpan selisih dan bandingkan kondisi sebelum/sesudah; jangan reset baseline
  untuk menyembunyikan perubahan yang belum dijelaskan.
- Observasi trial: 256 sampel, satu masalah `metrics` pada 03:23:33 UTC.
  Log tersedia memuat 3 WARN Telegram poll dan 1 WARN persist shutdown.
- Backup divalidasi daftar arsip/checksum saat dibuat; penutupan tidak mencakup
  restore drill baru. Backup masih di disk server yang sama.

## Akses dan batas

`ssh -o BatchMode=yes plesk` menuju server Plesk bersama. Hanya project Compose
`trading-bot-paper` di `/opt/trading-bot-paper` yang boleh dikelola dalam cakupan
bot. Jangan mengubah Plesk/domain/firewall/web server/Docker daemon, restart host,
prune global atau menghapus volume. Mainnet belum diizinkan; manifest memaksa
`TB_APP__ALLOW_LIVE=false`.

```text
/opt/trading-bot-paper/
  access/       deploy key read-only dan known_hosts proyek
  repo/         checkout rilis, .env.vps privat, configs/config.yaml
  evidence/     baseline dan sampel observasi
  backups/      tujuh backup sukses terbaru, berisi secret
```

Config aktif adalah profil managed ETH dengan Telegram; config testnet lama
tersimpan sebagai `configs/config.testnet.yaml`. `BOT_DB_NAME=tradingbot_eth`.
Secret hanya melalui `deploy/vps-set-env.py` via stdin SSH; jangan mencetak
`.env.vps`. Tool itu menerima `TB_*`, `BOT_IMAGE` dan `BOT_DB_NAME`.

## Inspeksi bila diminta

Dari `/opt/trading-bot-paper/repo` di VPS; perintah berikut tidak memulai bot:

```sh
dc() { docker compose --env-file .env.vps -f deploy/compose.vps.yml "$@" </dev/null; }
dc ps --all
docker inspect -f 'status={{.State.Status}} exit={{.State.ExitCode}} started={{.State.StartedAt}}' trading-bot-paper-bot-1
python3 deploy/vps-baseline-check.py
systemctl is-active trading-bot-paper-backup.timer
```

`vps-observe.py` memeriksa proses/feed yang aktif; bot sengaja mati akan memberi
masalah kesehatan. Jangan menyalakan timer observasi trial yang sudah ditutup.
`/healthz` hanya membuktikan DB, bukan kesehatan feed atau strategi.

Port host loopback: web 18080, metrics 19100, Prometheus 19090, Grafana 13000.
Tunnel dashboard/monitoring saat layanan terkait aktif:

```sh
ssh -N -L 18080:127.0.0.1:18080 -L 13000:127.0.0.1:13000 plesk
```

DNS bot memakai CoreDNS/DoT Cloudflare pada subnet `172.31.253.0/29` karena DNS
host pernah memberi halaman blokir Binance. Jangan mengubah resolver host.
Compose dalam skrip SSH harus memakai `</dev/null`/`--interactive=false` agar
stdin skrip tidak tertelan. Izin hook/auto-mode Claude tidak otomatis berlaku
untuk Codex atau agent lain; periksa sandbox yang aktif.

## Riwayat yang tetap disimpan

- Testnet 18–20 Oktober berakhir normal setelah ±43 jam, tujuh order dibatalkan
  sesudah recorder stop; DB historis masih `new`, exchange final nol open order.
  Inventori dipertahankan. [Analisis](testing/testnet-stop/analysis.md).
- Rollback testnet historis: commit `f709429`, digest
  `sha256:e5b8710bc76da2cd5d581511e19fb7bf9e6a472e0eb61b14fb3c099054fd0438`.
  Ini referensi, bukan instruksi start. Jangan down migrasi `tradingbot_eth`.
- Riwayat operasi awal: [handover awal](testing/vps-handoff.json) dan
  [operasi/restore](testing/vps-ops.json). Prosedur rilis, migrasi dan rollback
  terpusat di [deployment](deployment.md).
