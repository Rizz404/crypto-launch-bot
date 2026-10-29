# Rilis operasi paper P2/P3 — 29 Oktober 2026

**Selesai pada 22:23 WIB**, source **`d30b93b`**, setelah izin eksplisit user
untuk commit, push, deploy dan memasang timer P3. [Seluruh CI lulus](https://github.com/Rizz404/trading-bot/actions/runs/40899450590):
race/vet/lint/build, PostgreSQL/recovery, monitoring, kedua image dan protokol
Spider. [Bukti lengkap](paper-operations-release.json).

## Source dan image

| Worker | Image tetap | Mulai proses (WIB) |
|---|---|---|
| P2 | `ghcr.io/rizz404/trading-bot@sha256:fbf28da706e06aebf022e964e12ee1e29183eb948a5b6261dd7617b5a7f7665e` | 29 Oktober 22:23:14 |
| P3 | `ghcr.io/rizz404/trading-bot-catalyst@sha256:d43b809876dcc4f8422e25383fb3f83e8764df405f893ae04e925bef47742a76` | 29 Oktober 22:23:21 |

Source label, ID image, binary P2 dan checkout kedua VPS cocok dengan commit
`d30b93b33307bad5a8736a6685b7e556aaee6040`. Spider tetap
`spider_cli 2.53.9-p3.1`. P2 `mode=paper`, `allow_live=false`; P3 tetap Sim/paper.
Tidak ada build/salin aplikasi laptop, perubahan schema atau migrasi target.

Rilis awal `d33dd84` sempat dipasang ke P2 **22:04 WIB**. Readiness 503
mengungkap bootstrap seri 4h yang sudah mutakhir di DB, sekaligus masih adanya
beban dispatch idle. P3 belum diganti pada tahap itu. Patch lanjutan melewati
dispatch tanpa handler/due timer dan mengisi frontier health saat tidak ada
bar baru yang perlu dikirim. Race/vet/lint dan CI diulang untuk source final.
Kedua rollout P2 adalah recreate terencana; `RestartCount=0` container final
berarti tidak ada restart otomatis, bukan tidak pernah melakukan deploy.

## Ledger dan isolasi

Proyeksi finansial/config/status sebelum dan sesudah cocok persis, termasuk
kas tiap instance, funded, status pause, parameter, holding dan pending.

| Akun | Modal USDT | Equity USDT | Total fill | T0 tetap (WIB) |
|---|---:|---:|---:|---|
| P2 | 747,83957456237531 | 740,40558346906799 | 214 (210 hot +4 arsip) | 22 Oktober 19:58:45 |
| P3 | 1.000 | 1.000 | 0 | 28 Oktober 22:53:13 |

Dua EMA 1m tetap **paused**, empat instance P2 lain running. Tidak ada holding
nonzero, pending event atau order nonterminal pada kedua akun. Hash config
aktif P2 tetap `8687ca0c…22d893`; kuota P3 tetap 1 slot pada 28 Oktober dan
4 slot pada 29 Oktober. Tidak ada inference baru yang dipaksakan saat rilis.

**36 container selain dua worker identik** sebelum/sesudah. Layanan host
`psa`, `nginx`, `httpd`, `sw-cp-server` dan Docker tetap pada state yang sama.
Tidak restart host/Docker daemon atau mengubah proyek Plesk lain.

Backup P2 pra-rilis `backup-20261029T145749Z` lolos checksum dan restore ke
DB scratch (schema 5, satu wallet, 618 jurnal, empat fill arsip); proyeksi
finansial cocok dan scratch dihapus. Peringatan circular-FK TimescaleDB tidak
dijadikan bukti lulus: restore benar-benar dijalankan. Backup P3 baru
`backup-20261029T152320Z` dibuat ketika worker sudah dihentikan untuk deploy,
sehingga tidak menambah restart terpisah. Restore flat DB/arsip P3 sebelumnya
tetap dirujuk pada [bukti restore](catalyst-backup-restore.json).

## Hasil awal feed P2

Kedua health endpoint 200. Observer P2 pukul **22:25 WIB** melaporkan
`problems: []`. Candle 1m BTC/ETH/SOL maju dari open **22:24 ke 22:29 WIB**.
Pemeriksaan 22:30 WIB menemukan **tujuh bar berurutan per simbol**, tanpa gap
sejak open 22:23 WIB. Umur open 118 detik pada akhir menit adalah bar tertutup
terbaru, bukan keterlambatan tambahan 118 detik setelah penutupan.

| Sampel singkat | Source awal `d33dd84` | Source final `d30b93b` |
|---|---:|---:|
| Commit DB/detik | 84,81 | 18,92 |
| CPU bot (% satu core) | 59,73% | 27,39% |
| CPU DB (% satu core) | 64,30% | 9,50% |
| Health | 503 | 200 |

Sampel commit sekitar 15 detik dan CPU merupakan pembacaan singkat; waktu,
fase startup dan query verifikasi berbeda. Ini indikasi perbaikan, bukan
benchmark terkontrol atau batas kapasitas puncak. Log sekitar enam menit
sesudah start final mencatat **nol `Pong timeout`/disconnect**, satu backfill
selesai. Belum membuktikan masalah tidak akan berulang selama 24 jam/tujuh hari.
Penutupan candle 4h berikutnya pukul 23:00 WIB belum tercakup checkpoint ini.

## Timer dan status riset P3

Kedua timer P3 **enabled dan active/waiting**. Observer dipicu otomatis pada
**22:23:26 dan 22:28:27 WIB**; backup berikutnya menurut systemd
**30 Oktober 10:48:38 WIB** (jadwal 03:45 UTC + jitter).

Observer sengaja exit 1/unit failed dengan `research_error` dan
`latest_review_invalid`: review WATCH terakhir sebelum patch mengirim simbol
kosong dan ditolak validator. Ini **alert nyata yang dipertahankan**, bukan
kegagalan scheduler atau health feed. Tiga sumber sehat, timer/quote segar,
modal tetap 1.000 dan kuota hari ini 4/12. Schema baru wajib simbol nonempty,
tetapi belum ada inference nyata baru yang membuktikan kepatuhan provider.
Tidak mengubah review lama atau memaksa panggilan tambahan untuk menghijaukan
alert. Pantau keputusan baru berikutnya; proteksi/feed tetap independen.

QA Chromium pascarilis pada 1440×900 dan 390×844 membuktikan alasan keputusan
arsip dan error validasinya kembali terlihat sesudah restart, tanpa overflow
horizontal. [Bukti/screenshot baru](catalyst-browser-release.json).
Hanya GET; tidak menguji aksi form, Safari atau perangkat fisik.

Review formal P3 hari 1 tetap **29 Oktober 22:53 WIB** dari T0 ledger, belum
jatuh tempo pada pengamatan ini. Seluruh bukti di atas adalah verifikasi rilis
awal, bukan kelulusan seluruh penerimaan operasi atau izin uang asli.
