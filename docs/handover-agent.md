# Handover agent

Untuk Codex, Claude dan agent berikutnya. Checkpoint **21 Oktober 2026**;
verifikasi checkout sebelum bekerja, jangan menganggap commit lokal sudah dirilis.

## Status dan prioritas

- Permintaan terbaru: commit seluruh perubahan dan mulai "tes live", dengan
  laporan user hari 1/3/7. Mode paper VPS vs mainnet uang asli sedang ditanyakan;
  belum ada trial baru dimulai atau jadwal kalender review. Baca
  [protokol review dan prasyarat start](handover-vps.md#permintaan-pengujian-berikutnya-dan-review-hari-137).
  T0 harus start baru yang terverifikasi. Komposisi 24/72 belum runnable dan
  modal/risiko mainnet belum ditetapkan. Jangan melanjutkan demo sebagai live.

- Implementasi managed P2-1–P2-6 serta pengujian/fixing P2-7/P2-8 tersedia.
  Rincian penerimaan dan pekerjaan terbuka ada di [Plan 2](plan-2.md).
- P2-9 sempat berjalan di VPS, lalu ditarik user pada 21 Oktober 20.30 WIB.
  Bot exited 0; kas virtual tetap 110,791048 USDT; posisi/order/eksekusi kosong.
  Review tujuh hari dibatalkan, bukan lulus. Tidak ada strategi pengganti aktif.
- Fokus: [riset strategi terbaru](strategy-2.md). ETH momentum lama menjadi
  referensi; BTC harian dan pullback BTC/ETH masih kandidat evaluasi.
- User memperluas riset ke top 20, variasi modal/timeframe dan multi-strategi.
  [Matriks dan kapasitas](strategy-2.md#perluasan-riset-multi-instance--21-oktober-2026):
  plafon 6.528 backtest serial, paper bertahap maksimal 24 instance/72 binding.
  Baru rancangan; tiga strategi, antrean VPS/guard dan throughput belum siap.
  Inspeksi VPS read-only 21 Okt sekitar 21.53 WIB: 8 CPU, available RAM 19,6 GiB,
  disk free 137 GiB. Ini snapshot, bukan kapasitas teruji untuk 72 binding.
- Mainnet/P2-10 belum disetujui. Futures menunggu Demo key; Bybit/xspread
  ditunda, OKX dihapus dan saham tokenisasi tetap dikecualikan.
- Perbaikan Telegram lokal: runtime managed memulihkan `/balance`, `/pnl`,
  `/positions`, `/orders`, `/strategies`, menambah alias `/killswitch` dan
  mendaftarkan `/help`/`/start` pada menu yang sama. Tes command dan payload
  suggestion lulus; belum deploy/publikasi menu nyata atau resume trial.
  Bahasa laporan/kontrol sudah dirapikan, ringkasan dipisah dari rincian dan
  daftar memakai halaman lima item. `/status` membedakan proses aktif dengan
  izin entry menurut pengaturan; tidak mengklaim kesehatan feed/sinyal.
  Revisi format setelah review user: bedakan belum tersedia/dijeda dari registry,
  nama strategi manusiawi, PnL nol diringkas, presisi komisi dan timestamp,
  label basis persentase serta estimasi rugi stop. Legacy belum menyediakan
  modal acuan instance/alasan exit rinci; respons menyatakan keterbatasannya.
  Implementasi dan fixture sudah di-commit sebagai `400e70a`. Pemeriksaan
  persiapan commit: `go test -race ./...`, `go vet -tags integration ./...`,
  lint (0 issues) dan `make build` lulus. Tes yang memerlukan socket localhost
  diulang di luar sandbox setelah gagal karena izin, bukan karena assertion.
  Integrasi DB bertag, monitoring container dan CI/GHCR belum dijalankan ulang.
- Arahan uji Telegram 21 Oktober: gunakan `.env` lokal; user menyatakan env
  lokal/VPS masih sama selama belum live. Token lokal tersedia; isi env tidak
  dibandingkan/disinkronisasi. Sebanyak 32 contoh diterima API (11 dari VPS
  sebelum arahan, 21 sisanya dari lokal; satu duplikat saat peralihan mungkin).
  Izin lokal khusus pesan demo, bukan `bot run`/trial. Prosedur/ekspor contoh:
  [runtime managed](managed-runtime.md#uji-format-telegram-dari-lokal).

## Lanjutkan dari sini

1. Periksa `git status --short` dan `git log -12 --oneline`; pertahankan perubahan
   yang belum di-commit. Jangan commit/push tanpa permintaan user.
   User sudah meminta commit seluruh perubahan sesi ini; push belum diminta.
2. Baca [Strategi 2](strategy-2.md) dan [Plan 2](plan-2.md). Bekukan aturan,
   benchmark, biaya, periode dan risiko sebelum backtest berikutnya.
3. Audit restart bersama pukul 10.23 WIB dan WARN shutdown sebelum trial baru.
   Bukti dan kondisi server ada di [handover VPS](handover-vps.md).
4. Jangan mengaktifkan ulang trial lama. Trial berikutnya memakai ledger/DB baru,
   modal total eksplisit dan prosedur [deployment](deployment.md).

## Batas kerja

- Bot/trial ETH tidak boleh berjalan di laptop. Pertahankan marker/data
  `var/paper-eth/`; jangan memakai `bot run` untuk mencoba dashboard.
- Jangan menjalankan test, build, server atau verifikasi runtime otomatis
  kecuali diminta atau tugasnya memang pengujian/verifikasi tersebut.
- VPS Plesk berbagi layanan produksi. Sentuh hanya project `trading-bot-paper`
  di `/opt/trading-bot-paper`; jangan restart host/Docker atau mengubah proyek lain.
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
