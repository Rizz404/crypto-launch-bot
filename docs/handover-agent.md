# Handover agent

Untuk Codex, Claude dan agent berikutnya. Checkpoint **29 Oktober 2026**;
verifikasi checkout dan waktu bukti sebelum bekerja.

## Status dan prioritas

**Rilis 29 Oktober 22:23 WIB selesai**, source **`d30b93b`**, setelah izin
commit/push/deploy/pemasangan timer P3. [Seluruh CI lulus](https://github.com/Rizz404/trading-bot/actions/runs/40899450590).
Mulai dari [laporan rilis](testing/paper-operations-release.md) dan
[bukti](testing/paper-operations-release.json). Commit dokumentasi sesudahnya
bukan berarti image dengan SHA docs itu sudah dibangun/dipasang.

- **Plan 3 tetap roadmap aktif repo Rizz404/trading-bot.** User meluruskan
  29 Oktober bahwa Plan 4 dari memori Windows adalah proyek/repo GitHub lain.
  Fakta sumber Hindsight dikoreksi; jangan mengarsipkan Plan 3 karena memori itu.
- **Review tujuh hari P2 selesai:** modal 747,83957456 → equity 740,40558347
  USDT, PnL −7,43399109 (−0,9941%). Seluruh rugi dari dua EMA 1m; tiga Tren
  dan EMA 4h belum bertransaksi. Komisi 5,77387876 sudah termasuk realized.
  [Review](testing/research-stage-two-review.md), [pause](testing/research-stage-two-pause.json).
- Dua EMA 1m sudah pause entry **21:10 WIB**. Empat instance lain running.
  Sesudah rilis, kas/status/parameter/T0/holding/pending cocok dengan sebelum
  rilis; total 214 fill (210 hot +4 arsip). **T0 tetap 22 Oktober 19:58:45 WIB**,
  DB `tradingbot_research_stage_two`, wallet `paper:research-stage-two`.
- P2 source `d30b93b`, worker mulai 29 Oktober 22:23:14 WIB. Health dan observer
  200/`problems: []`; tujuh bar 1m berurutan per BTC/ETH/SOL tanpa gap pada
  pemeriksaan 22:30. Sampel singkat commit DB 84,81 → 18,92/detik, CPU bot
  59,73% → 27,39%, DB 64,30% → 9,50% satu core. Nol Pong timeout/disconnect
  pada log sekitar enam menit. **Belum bukti 24 jam/tujuh hari atau peak load.**
- P3 source sama, worker mulai **29 Oktober 22:23:21 WIB**, ledger T0 tetap
  **28 Oktober 22:53:13 WIB**. Modal/kas 1.000 USDT, nol fill/holding/pending,
  sumber Ethereum/Anza/Dogecoin sehat dan kuota 29 Oktober tetap 4/12.
  Risiko tetap 0,5%/trade, dua posisi, halt harian 2%/drawdown 8%; hari risiko UTC,
  kuota AI WIB, output 2.048 token/call. Model `cx/gpt-6-luna-low`, prompt v5.
- **Timer backup/observer P3 sudah enabled/active.** Observer terpicu otomatis
  22:23:26 dan 22:28:27 WIB. Backup berikutnya 30 Oktober 10:48:38 WIB.
  Backup/restore flat P3 lulus; backup P2 pra-rilis juga direstore ke salinan.
  36 container lain dan lima layanan host identik; tidak restart host/Docker.
- **Alert riset P3 masih nyata:** review WATCH terakhir sebelum patch memiliki
  simbol kosong, sehingga observer exit 1/unit failed (`research_error`,
  `latest_review_invalid`). Runtime/feed tetap 200, keputusan arsip terlihat
  kembali setelah restart. Schema baru wajib simbol nonempty; belum ada
  inference baru sesudah patch. Jangan menulis ulang review/reset marker atau
  menghabiskan kuota hanya untuk menghijaukan alert.
- QA Chromium desktop/mobile sudah memeriksa halaman aktual lewat SSH tunnel,
  auth dan keputusan arsip. Aksi form, Safari/perangkat fisik, recovery posisi/
  order aktif, outage dan penerimaan Telegram alami/digest tetap terbuka.
  Backup luar server tetap ditunda sampai live sesuai keputusan user.
- Dua belas replay biaya pada kandidat beku selesai. Pullback BTC/ETH masuk
  shortlist eksplorasi, tanpa aktivasi baru atau holdout murni.
  [Riset terbatas](testing/strategy-cost-review/readme.md). Ekspansi 44/132,
  top-20 dan keluarga strategi baru tetap P3-R; mainnet belum disetujui.
- Handover pilot/Spider menyimpan riwayat pause/WIP 28 Oktober. Gunakan status
  rilis 29 Oktober di atas untuk melanjutkan; jangan mengulang bootstrap.

## Lanjutkan dari sini

1. Periksa `git status --short` dan source/runtime di handover VPS. Pertahankan
   perubahan belum di-commit; jangan reset/clean atau menyalin aplikasi laptop.
2. Monitor source `d30b93b`: freshness/gap, reconnect/Pong, CPU/DB dan observer.
   Penutupan 4h berikutnya 29 Oktober 23:00 WIB belum tercakup bukti awal ini.
   Pause EMA 1m dipertahankan; jangan mengubah parameter/modal/T0/history.
3. Review P3 hari 1 **29 Oktober 22:53 WIB** (belum jatuh tempo pada checkpoint
   22:30), lalu 31 Oktober/4 November 22:53 WIB. Bedakan runtime/feed, sumber,
   keputusan valid dan performa. Periksa keputusan baru untuk menutup alert
   lama tanpa memaksa inference di luar dedup/kuota.
4. Periksa backup terjadwal pertama dan observer sesuai kebutuhan. Timer sudah
   aktif; jangan memasangnya atau mengulang restore flat lagi tanpa perubahan/
   risiko baru. Recovery posisi/order aktif dan outage tetap pekerjaan lanjut.
5. Tidak mengaktifkan ulang ETH/tahap satu yang dihentikan, mengaktifkan
   kandidat baru atau mainnet. Eksperimen berikut memakai versi/periode dan
   ledger/modal terpisah sesuai [deployment](deployment.md).

## Batas kerja

- Bot/trial ETH tidak boleh berjalan di laptop. Pertahankan marker/data
  `var/paper-eth/`; jangan memakai `bot run` untuk mencoba dashboard.
- Jangan menjalankan test, build, server atau verifikasi runtime otomatis
  kecuali diminta atau tugasnya memang pengujian/verifikasi tersebut.
- VPS Plesk berbagi layanan produksi. Batasi perubahan ke `trading-bot-paper`
  di `/opt/trading-bot-paper` atau P3 `trading-bot-catalyst` di
  `/opt/trading-bot-catalyst`, sesuai scope tugas; jangan restart host/Docker
  atau mengubah proyek lain. Rilis P3 tidak menjadi alasan merilis ulang P2.
- Rilis lewat GitHub → CI/GHCR → digest tetap; jangan salin source laptop.
  Secret lokal/VPS tetap di file env masing-masing dan tidak masuk chat, repo,
  log atau shared memory. Uji pesan Telegram memakai env lokal sesuai arahan
  user; jangan menyalin env sebagai bagian pengujian.
- Backup di luar VPS ditunda sampai live sesuai keputusan user; backup saat ini
  masih satu server. Tidak ada izin live dari kelulusan test atau penutupan plan.

## Kontrak kode yang perlu dijaga

- `internal/managed` adalah ledger otoritatif JSONB/jurnal. Engine per binding
  hanyalah view; jangan menggandakan modal. Satu pemilik tulis per wallet/mode.
- Reservasi disimpan sebelum I/O; cancel belum melepas dana sebelum ack terminal.
  Inbox finansial, checkpoint dan dedup mencegah fill/fee dibukukan dua kali.
- Warmup tidak mengirim order; Sim mencocokkan candle 1m dan settlement per fill.
  Dua strategi boleh memegang koin sama, tetapi hanya menjual inventori sendiri.
- Migrasi legacy tidak mengadopsi saldo/inventori otomatis. ID run legacy berbeda
  dari ID instance. Jangan down migrasi 4/5 pada DB berisi history managed.
- Binding baru/edit perlu restart untuk feed/indikator; buat/clone mulai paused
  tanpa modal. Stop proses mempertahankan order/posisi, berbeda dari finish.
- Komisi memakai trade asli, termasuk aset base/BNB. Dust tidak boleh dihapus
  untuk memalsukan status flat. Recovery lengkap: [runtime](managed-runtime.md).

## Bukti dan keterbatasan

[Test lokal](testing/plan-2-local.md), [QA browser](testing/plan-2-browser-qa.md),
[review kode](testing/plan-2-code-review.md), [start VPS](testing/p2-9-vps-start.json),
[stop VPS](testing/p2-9-stop.json) dan [review trial](testing/p2-9-review.json).
Hasil test itu historis; tidak dijalankan ulang oleh pembaca dokumen.

Firefox fixture belum membuktikan Safari/perangkat fisik, Telegram whitelist/
digest, fee riil, storage outage atau ketahanan operasi panjang. Penilaian
strategi belum memiliki holdout murni atau bukti return 50% konsisten.
