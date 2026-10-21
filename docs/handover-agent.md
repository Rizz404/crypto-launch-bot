# Handover agent

Untuk Codex, Claude dan agent berikutnya. Checkpoint **21 Oktober 2026**;
verifikasi checkout sebelum bekerja, jangan menganggap commit lokal sudah dirilis.

## Status dan prioritas

- Implementasi managed P2-1–P2-6 serta pengujian/fixing P2-7/P2-8 tersedia.
  Rincian penerimaan dan pekerjaan terbuka ada di [Plan 2](plan-2.md).
- P2-9 sempat berjalan di VPS, lalu ditarik user pada 21 Oktober 20.30 WIB.
  Bot exited 0; kas virtual tetap 110,791048 USDT; posisi/order/eksekusi kosong.
  Review tujuh hari dibatalkan, bukan lulus. Tidak ada strategi pengganti aktif.
- Fokus: [riset strategi terbaru](strategy-2.md). ETH momentum lama menjadi
  referensi; BTC harian dan pullback BTC/ETH masih kandidat evaluasi.
- Mainnet/P2-10 belum disetujui. Futures menunggu Demo key; Bybit/xspread
  ditunda, OKX dihapus dan saham tokenisasi tetap dikecualikan.

## Lanjutkan dari sini

1. Periksa `git status --short` dan `git log -12 --oneline`; pertahankan perubahan
   yang belum di-commit. Jangan commit/push tanpa permintaan user.
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
  Secret tetap di VPS dan tidak masuk chat, repo, log atau shared memory.
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
