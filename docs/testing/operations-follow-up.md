# Lanjutan operasi P2/P3 — 29 Oktober 2026

> **Status akhir:** rilis source `d30b93b` dan pemasangan timer selesai 29 Oktober
> 22:23 WIB. [Laporan rilis](paper-operations-release.md) menggantikan status
> lokal/belum dipasang pada checkpoint persiapan berikut; rinciannya tetap
> dipertahankan sebagai riwayat diagnosis dan verifikasi.

User menyetujui urutan diagnosis/perbaikan feed P2, pause entry dua EMA 1m,
backup/observer P3, QA dashboard dan riset strategi terbatas. Paper saja.
User kemudian memberi izin eksplisit **commit, push, deploy dan pemasangan
timer P3**. Checkpoint laporan ini sebelum rilis; source/digest dan pemeriksaan
pascarilis akan dicatat terpisah.

## P2: entry dijeda, data dipertahankan

Pada **21:10:28 /21:10:30 WIB**, kontrol HTTP aplikasi dengan auth/CSRF
menjeda instance `ema-base-1m` dan `ema-trend-1m`. Ini operasi `pause`, bukan
cancel, flatten atau finish. Saat aksi keduanya flat; kas tidak berubah.
Empat instance lain tetap running. T0, capital, config hash dan parameter sama.

[Bukti pause](research-stage-two-pause.json),
[pemeriksaan 21:29 WIB](research-stage-two-pause-verification.json).
Jumlah detail hot turun dari 214 menjadi 210 karena empat fill sudah berada
di `managed_archive`; total tetap 214. Tidak ada penghapusan history.
[Review sebelum pause](research-stage-two-review.md) tetap snapshot historis.

## Diagnosis dan patch feed

Jalur `WSStream → StreamKlines → Feed → Runtime.Record` sinkron; kanal Binance
menampung 1.024 event. Dokumentasi dan source dependency `coder/websocket`
v1.8.15 memastikan pong ditangani pada jalur baca. Consumer yang tertahan dapat
menghentikan pembacaan/control frame saat kanal penuh. Ini mekanisme yang
sesuai gejala, **belum pembuktian profil CPU atau root cause tunggal di VPS**.

Pekerjaan berulang yang ditemukan di managed runtime:

- Checkpoint engine yang sama tetap membuka transaksi, mengunci row dan
  mengurai/mengodekan seluruh ledger sebelum mendapati no-op. Patch memeriksa
  kesamaan dari snapshot yang sudah dibaca terlebih dahulu. Lease tetap dicek,
  dan transaksi untuk perubahan nyata tetap berjalan.
- Setiap harga menjalankan sync/checkpoint pada binding simbol lain. Patch
  melewati binding yang tidak memakai simbol itu; parent risk halt tetap
  memerintahkan penutupan pemilik simbol lain. Sim settlement dan drain inbox
  tetap dijalankan sebelum routing.
- Health managed sekarang memeriksa setiap seri setelah callback consumer
  selesai, bukan hanya DB/lease. Seri belum siap, bertimestamp masa depan atau
  umur close melebihi satu timeframe + dua menit menghasilkan 503.
  Gauge freshness juga diperbarui sesudah consumer selesai.

Regression memeriksa transaksi no-op, perubahan nyata/lease loss, routing
simbol, parent halt lintas simbol, consumer yang macet, seri yang hilang dan
data stale/future. **Belum ada pengukuran perbaikan latency/reconnect VPS**:
P2 masih menjalankan image lama. Jangan menulis bahwa Pong timeout sudah sembuh.

## Backup dan restore P3

Backup sekali jalan dijalankan sebagai operasi operator, tanpa memasang helper
baru permanen atau mengganti image aplikasi. Worker P3 saja dihentikan singkat
agar DB, inbox tesis, bukti dan marker kuota berada pada checkpoint konsisten.
Container yang sama dinyalakan kembali; tidak ada recreate atau reset T0.

- Backup **21:18:57 WIB**:
  `/opt/trading-bot-catalyst/backups/backup-20261029T141857Z`.
- Snapshot sebelum/sesudah berjarak **3,91 detik**. Ini batas pengamatan operasi,
  bukan pengukuran presisi downtime. **37 container lain identik**, termasuk P2.
- Checksum tujuh berkas cocok. Dump direstore ke
  `tradingbot_catalyst_restore_verify`; schema versi 5, satu wallet, delapan
  jurnal dan nol record managed archive. Proyeksi finansial/config/T0 cocok
  dengan sumber sebelum/sesudah restore.
- Arsip diekstrak ke direktori sementara sendiri: 283 file, **271 record
  content-addressed lolos hash**, budget 1 slot pada 28 Oktober dan 4 slot
  pada 29 Oktober tetap ada. DB dan direktori restore sementara sudah dihapus.
- Modal tetap **1.000 USDT**, tanpa holding/pending. DB/volume aktif tidak
  menjadi target restore. Bukti: [backup/restore](catalyst-backup-restore.json).

Helper lokal `deploy/catalyst-backup.sh` mempertahankan proses yang sebelumnya
running, termasuk pada kegagalan dump; fixture error menguji jalur tersebut.
Backup menyimpan env privat di VPS (0700/0600), tidak dikirim ke laptop/chat.
Tidak ada penghapusan backup lama otomatis; disk guard menahan backup ketika
ruang di bawah 2 GiB. Backup terjadwal akan menghentikan worker P3 singkat;
operasi/restart tersebut perlu dihitung saat meninjau kontinuitas pilot.

## Observer dan alert P3

`deploy/catalyst-observe.py` memeriksa container, health runtime/feed, umur
timer/quote/riset/sumber, source gap, error/review invalid, halt/attention,
kuota AI, disk dan umur backup. **12 slot tepat bukan pelanggaran**; lebih
dari 12 ditandai. Tidak ada recovery otomatis, inference, kontrol trading atau
pesan chat. Alert berupa daftar `problems`, exit 1 dan unit systemd failed.
Snapshot terakhir disimpan atomik; journal systemd menyimpan riwayat alert.

Uji sekali jalan pada Python 3.9 VPS **21:22 WIB** menghasilkan exit 1 dengan
`latest_review_invalid` meskipun runtime/feed sehat. Ini temuan nyata, bukan
tes yang gagal; helper menangkap respons WATCH kosong yang ditolak validator.
[Bukti observer](catalyst-observer-verification.json).

Unit P3 sendiri sudah disiapkan: observe tiap lima menit, backup harian sekitar
**10:45–10:55 WIB** (03:45 UTC + jitter), CPU/memori dibatasi. **Timer belum
dipasang/diaktifkan**; harus diambil dari source GitHub yang disetujui.
Backup luar server tetap ditunda sampai live sesuai keputusan user.

## P3: hasil awal dan QA

[Pemeriksaan 21:14 WIB](catalyst-pilot-review.json), sebelum backup:
modal/kas 1.000 USDT, nol fill/holding/order/pending, tanpa halt/attention.
Tiga sumber sukses. Kuota hari WIB 29 Oktober 4/12; total lima inference sejak
start, 50.292 token input /2.055 output. Lima keputusan WATCH, tiga valid dan
dua invalid (kutipan lintas aset pada rilis awal; simbol kosong pada review
terakhir). Tidak ada event yang memenuhi syarat entry. Angka ini observasi
prospektif awal, **belum review formal hari 1 pukul 22:53 WIB**.

Schema output lokal kini mewajibkan simbol nonempty, sesuai validator yang
sudah ada. Syarat event/entry/HOLD, risiko, policy hash, kuota dan model tidak
dilonggarkan. Belum ada inference nyata dengan schema baru; provider masih
dapat gagal memenuhi schema dan validator tetap otoritatif.

Chrome DevTools tidak menemukan executable Chrome. Fallback Playwright/
Chromium sementara berhasil membuka **dashboard dan research di VPS** melalui
SSH tunnel pada viewport 1440×900 dan 390×844. Semua empat navigasi 200,
tidak ada overflow dokumen horizontal. Auth tanpa credential menghasilkan 401
pada halaman/state/research; security headers terpasang. Hanya GET di origin
tunnel yang diizinkan, tanpa submit form. Kredensial hanya dipakai di memori;
tidak disimpan pada artefak.

Screenshot memperlihatkan keputusan terakhir kosong sesudah restart walaupun
review ada di arsip. Patch HTTP lokal memakai keputusan arsip saat status
proses belum memiliki keputusan; regression juga membuktikan tampilan itu
tidak mengubah state worker/trading. Format halaman research masih sederhana
dan timestamp UTC panjang; tidak dirombak untuk perubahan operasional ini.

[Hasil browser dan screenshot](catalyst-browser-qa.json). Screenshot adalah
image rilis lama `1779878`, **bukan bukti visual patch lokal sudah terpasang**.
Belum menguji Safari, perangkat fisik atau aksi form di browser.

## Verifikasi lokal dan rilis berikutnya

Sesudah perubahan Go final:

- `go test -race ./...` lulus.
- `go vet -tags integration ./...` lulus.
- `golangci-lint run ./...` lulus, 0 issues.
- Tiga fixture Python ops lulus; syntax Bash dan validasi unit systemd lulus.
- CI ditambah pemeriksaan helper ops; workflow GitHub **belum dijalankan**.

Tes PG lifecycle lokal tidak diulang. Restore nyata P3 di atas membuktikan
pemulihan backup saat flat, bukan recovery worker dengan posisi/order aktif.
Tes fixture Go juga bukan bukti load/reconnect prospektif setelah rollout.

Izin commit/push/deploy/pemasangan timer P3 sudah diberikan user.
Ikuti GitHub → CI/GHCR → digest tetap, backup/baseline,
deploy worker terkait saja dan pemasangan unit P3 dari checkout tersebut.
Verifikasi ulang pause EMA, kas/T0, source/digest, health/freshness, beban,
observer dan identitas proyek lain. Tidak mengaktifkan kandidat strategi baru
atau mainnet sebagai bagian rilis ini.


## Temuan saat rollout awal

CI source `d33dd84` lulus seluruh job. P2 dipasang 29 Oktober **22:04:40 WIB**;
P3 tetap pada image sebelumnya ketika readiness P2 belum 200. Health baru
mengungkap celah startup seri 4h: bila DB sudah memuat bar tertutup terakhir,
backfill tidak mengirim bar baru sehingga frontier health belum terisi. Patch
lanjutan mengisi frontier dari bar tersimpan saat tidak ada delivery yang perlu
dijalankan, tanpa mengulang callback/indikator. Regression meliputi kondisi itu.

Inspeksi lanjutan juga memperkecil jalur idle: intrabar tidak menjalankan engine
yang tidak memiliki PriceHandler, dan timer hanya mendispatch engine yang
memang due. Pencocokan Sim, inbox, mark/Observe dan parent halt tetap dijalankan;
strategi katalis tetap price-driven dan timer satu detik. Regression timer due
serta uji dua directional binding membuktikan hanya dua Load pada tick idle,
tanpa transaksi checkpoint per binding. Patch lanjutan memerlukan CI/image baru
sebelum rollout diteruskan; ini bukan klaim Pong timeout sudah hilang.
