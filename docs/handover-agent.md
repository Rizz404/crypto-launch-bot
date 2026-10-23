# Handover agent

Untuk Codex, Claude dan agent berikutnya. Checkpoint dokumentasi **23 Oktober 2026**;
verifikasi checkout sebelum bekerja, jangan menganggap commit lokal sudah dirilis.

## Status dan prioritas

- **Rencana aktif: [Plan 3](plan-3.md).** User menutup Plan 2 sebagai siklus
  pada 23 Oktober; seluruh pekerjaan terbuka dipetakan ke P3-R/P3-O/P3-LIVE/
  P3-D. Plan 1/2 menjadi arsip, bukan antrean kedua atau bukti seluruh tes lulus.
  Arah baru: katalis terverifikasi, memecoin yang sudah diperdagangkan, hold
  selama tesis valid dan full exit saat proteksi/tesis mengharuskan. Presale/
  token baru ditunda sebagai future reference. Mulai dari kontrak P3-0;
  parameter numerik belum dibekukan, strategi AI belum diimplementasikan.
- **Penegasan prioritas:** P2 yang berjalan dibiarkan dan seluruh kode/config/
  data/bukti dipertahankan untuk riset berikutnya. Kerjakan P3-0 lalu fase AI
  tanpa menunggu backlog P2 selesai. Jangan menghapus strategi lama, mengubah
  runtime P2 atau merilis perubahan shared code ke P2 hanya untuk memasang AI.
  Pilot memakai ledger/modal terpisah; sebelum berjalan di VPS yang sama,
  gerbang kapasitas/API dan temuan keselamatan terkait tetap berlaku.
- **Checkpoint operasi terbaru dari dokumen VPS, 23 Oktober 20.52 WIB:**
  runtime `9c18962`, perbaikan notifikasi `afdab52`, image digest/source serta
  bukti di [handover VPS](handover-vps.md#rilis-notifikasi-transaksi--23-oktober-2026)
  dan [rilis notifikasi](testing/telegram-notification-release.json).
  Snapshot mencatat enam instance/18 binding, kas 746,92125354748169 USDT,
  22 eksekusi, tanpa posisi/pending/halt/attention. Ini checkpoint historis,
  bukan pemeriksaan server saat membaca handover.
- **T0 tahap dua tetap 22 Oktober 19:58:45 WIB**, DB
  `tradingbot_research_stage_two`, wallet `paper:research-stage-two`:
  tiga Tren + EMA dasar 1m/filter 1m/filter 4h, BTC/ETH/SOL. Modal awal
  747,83957456237531 USDT virtual dan CPU1. Review user 23/25/29 Oktober
  pukul 19:58:45 WIB, tanpa reminder AI otomatis. Pergantian plan tidak
  mengubah trial, modal, parameter atau T0. Strategi AI kelak memakai ledger
  dan modal terpisah; jangan mengganti trial berjalan dengan pilot AI.
- Tahap satu sudah dihentikan, ledger/backup dipertahankan; jadwal
  22/24/28 Oktober digantikan jadwal tahap dua. ETH P2-9 dihentikan 21 Oktober
  sebelum tujuh hari, tanpa trade; review tujuh harinya dibatalkan, bukan lulus.
- **Penerimaan tersisa:** pesan fill alami/digest 24 Oktober 07.00 WIB dan
  whitelist, restore backup pascarilis, recovery posisi/order aktif, outage,
  operasi panjang dan klasifikasi shutdown. Tindak lanjut ada di
  [P3-O](plan-3.md#p3-o--penerimaan-operasi-dan-tindak-lanjut-audit).
  Restore pra-rilis/restart tanpa posisi sudah punya bukti; jangan menyatakan
  semua penerimaan selesai hanya karena CI/health awal lulus.
- Ekspansi pembanding tetap target 44/132, termasuk 6/18 pada checkpoint;
  sisa 38/114 dan tiga keluarga strategi belum diimplementasikan. Riset
  top-20, plafon 6.528 job serial serta guard kapasitas/API masuk P3-R.
  [Strategi 2](strategy-2.md) menyimpan rincian; snapshot host lama bukan
  kapasitas terbukti untuk target penuh.
- Mainnet/P3-LIVE belum disetujui. Futures, Bybit/xspread dan funding/L2
  masuk P3-D1–P3-D3 dengan dependensi lama. OKX tetap dihapus dan saham
  tokenisasi tetap dikecualikan.
- Perbaikan Telegram lokal: runtime managed memulihkan `/balance`, `/pnl`,
  `/positions`, `/orders`, `/strategies`, menambah alias `/killswitch` dan
  mendaftarkan `/help`/`/start` pada menu yang sama. Tes command dan payload
  suggestion lulus; sudah dirilis dan 19 command menu chat diverifikasi API.
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
  CI rilis `37ffd6a` kemudian lulus termasuk integrasi DB/pemulihan, monitoring,
  race/vet/lint/build, smoke image dan publikasi GHCR.
- Arahan uji Telegram 21 Oktober: gunakan `.env` lokal; user menyatakan env
  lokal/VPS masih sama selama belum live. Token lokal tersedia; isi env tidak
  dibandingkan/disinkronisasi. Sebanyak 32 contoh diterima API (11 dari VPS
  sebelum arahan, 21 sisanya dari lokal; satu duplikat saat peralihan mungkin).
  Izin lokal khusus pesan demo, bukan `bot run`/trial. Prosedur/ekspor contoh:
  [runtime managed](managed-runtime.md#uji-format-telegram-dari-lokal).

## Lanjutkan dari sini

1. Periksa `git status --short` dan `git log -12 --oneline`; pertahankan perubahan
   yang belum di-commit. Jangan commit/push tanpa permintaan user.
   User sudah mengizinkan commit dan push/deploy untuk trial ini; perubahan
   strategi/modal atau deployment berikutnya tetap harus mengikuti cakupan izin.
2. Baca [Plan 3](plan-3.md); mulai P3-0 untuk kontrak AI atau ID P3-R/P3-O
   untuk pembanding/operasi. [Strategi 2](strategy-2.md) menyimpan protokol lama.
   Bekukan aturan, benchmark, biaya, periode dan risiko sebelum eksperimen baru.
3. Saat user kembali, review trial aktif dari T0 tersebut dan bandingkan bukti
   VPS/Telegram dengan [handover VPS](handover-vps.md). Restart bersama lama
   terbukti berkaitan penghentian service Docker, bukan reboot host; pemicunya
   belum ditelusuri. Jangan restart Docker untuk proyek bot.
4. Jangan mengaktifkan ulang trial ETH lama atau mengubah parameter trial aktif.
   Trial berikutnya memakai ledger/DB baru dan prosedur [deployment](deployment.md).

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
