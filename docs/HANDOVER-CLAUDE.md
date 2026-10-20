# Handover Codex → Claude — Plan 2

Tanggal: **20 Oktober 2026**. User meminta menyelesaikan tahap development lokal
yang sedang dikerjakan dan memperbarui dokumentasi sebelum pindah ke Claude
karena limit Codex. **Baca dokumen ini dahulu**, lalu [Plan 2](plan-2.md),
[panduan managed](managed-runtime.md), dan [laporan pengujian](testing/plan2-local-2026-10-20.md).

## Posisi pekerjaan

Implementasi lokal sudah mencakup P2-1 sampai P2-6: akun → instance → strategi,
modal bersama, ledger/reservasi, pemulihan, kontrol/risiko bertingkat,
dashboard/Telegram/template, dan CLI backtest portofolio. Pengujian/fixing
P2-7/P2-8 sudah dilakukan; status final dan keterbatasannya ada pada laporan tes.
**Bukan seluruh Plan 2 selesai:** browser QA, release/trial VPS P2-9,
keputusan live P2-10, dan fitur bersyarat D-01/02/03 masih terpisah.

Tidak ada deploy, push, trial prospektif baru, atau order mainnet pada pekerjaan
ini. Backtest historis dan fixture lokal adalah pengujian software, bukan trial
ETH prospektif di laptop. Jangan menjalankan `bot run` untuk mencoba dashboard.

## Implementasi yang perlu dipahami

- `internal/managed`: **ledger finansial otoritatif** dalam satu state JSONB
  PostgreSQL. `managed_wallets` + jurnal + daftar review legacy pada migrasi 4.
  Row lock mengatomikkan transfer, reservasi, fill, idempotency dan checkpoint.
- Satu `Runtime` dan satu broker fisik per wallet Spot; satu engine/view portfolio
  per binding. View engine diturunkan dari ledger, **bukan modal baru**.
  Dua binding/instance boleh memegang ETH yang sama tanpa menjual milik lain.
- `scopedBroker` menyimpan reservasi sebelum request exchange. Reject pasti
  melepaskan dana; timeout ambigu mempertahankan dana dan menghentikan runtime.
  Request cancel belum melepaskan reservasi sampai acknowledgment terminal.
- Financial inbox durable diperbarui sebelum event dikirim ke engine.
  Checkpoint engine dan penghapusan inbox satu transaksi. Checkpoint
  `BeforeBroker` menyimpan rencana entry/SL/TP/trailing sebelum I/O.
  Checkpoint tanpa reservasi ledger berarti submission belum pernah tercapai;
  startup menolaknya secara lokal. Posisi directional tanpa proteksi mendapat
  fallback exit. Grid/DCA/MM menyimpan state operasional; indikator di-warmup.
- `internal/engine/checkpoint.go`: akun view, order, fill ID, plan, timer,
  refs dan state strategi. Warmup tidak mengirim order. Paused Spot sell tetap
  diizinkan untuk mengurangi posisi.
- `broker.Sim.MatchCandle`: settlement tiap fill sebelum kandidat selanjutnya,
  agar stop dan target tidak sama-sama menjual satu posisi pada satu candle.
  Runtime hanya mencocokkan candle **1m**, bukan OHLC 4h/1d yang sudah berlalu.
- `cmd/bot/managed.go`: branch baru bila `wallets` ada; config lama tetap bekerja.
  Lease PostgreSQL juga dipakai path legacy baru. **Binary lama yang sudah ada
  di VPS tidak mengenal lease ini**, jadi jangan paralelkan worker lama/baru.
- Session/run merekam konfigurasi dan hash. Crash menandai run lama interrupted
  pada restart; shutdown tidak menjual atau membatalkan order secara implisit.
- Komisi Spot yang hilang dipulihkan dari `/api/v3/myTrades`, bukan fill sintetis
  fee nol. Raw commission dipertahankan; fee BNB dinilai dengan feed BNBUSDT dan
  ditelusuri sebagai debit aset asli. Fixture bukan bukti fee transaksi riil.

## Batas fitur dan hal yang harus dilanjutkan

1. **QA browser belum selesai:** inventory CUA mengembalikan nol browser.
   HTTP/auth/CSRF/CSP/template diuji; belum ada bukti screenshot mobile/desktop
   light/dark. Preview fixture tersedia lewat test opt-in, tanpa bot/feed/DB.
2. Schema memakai agregat JSONB, bukan satu tabel SQL per entitas rancangan.
   Ownership tersimpan pada binding/order/fill/holding. Jangan menambahkan
   modal dari portfolio view ke saldo ledger atau menafsirkan legacy
   `strategy_runs.instance_id` sebagai instance baru.
3. Migrasi legacy **tidak mengadopsi dana atau inventori otomatis**. Semua sesi
   lama ditandai `review`. Kepemilikan lama ambigu, terutama dust dan tujuh order
   DB `new` yang sebenarnya sudah dibatalkan setelah recorder berhenti.
   Review/adopsi nyata dan tes DB salinan VPS masih P2-9. Jangan mengubah bukti
   historis untuk membuat angka tampak cocok.
4. Buat/clone/edit binding menyimpan konfigurasi paused. **Restart diperlukan**
   untuk menyiapkan feed/indikator binding baru atau parameter yang diubah.
   Resume/reset ditolak bila engine masih memakai konfigurasi lama. Perubahan
   batas risiko dan cap dapat diterapkan langsung melalui kontrol.
5. Transfer hanya saldo bebas antar-instance/cadangan. Deposit/withdrawal riil
   dan perubahan dana akun bukan aksi otomatis. Selisih exchange membekukan
   entry; tidak membagi koreksi secara sembarang. Arsip instance memerlukan
   tunai dikembalikan ke cadangan dan posisi/order kosong, termasuk dust.
6. Grafik ringkas menyimpan maksimum 720 sampel per jam (30 hari), bukan sistem
   analitik historis tanpa batas. Data eksekusi/jurnal tetap disimpan.
7. CLI baru sudah diuji historis singkat dan stress biaya, **bukan riset baru
   multi-tahun atau holdout baru**. Model OHLC tidak mencakup queue/latensi/L2.
   Profil ETH-only dibekukan; split ETH/BTC pada contoh hanya demo pembagian dana.
8. Pengujian notifikasi nyata, reconnect/storage outage di VPS, digest 07.00 WIB,
   whitelist chat lain, backup/restore dan resource bersama masih verifikasi
   eksternal. Jangan mengklaimnya lulus hanya karena unit test lokal lulus.

## Batas operasional yang tetap berlaku

- VPS shared Plesk: jangan mengubah aplikasi lain. Alur rilis GitHub → CI/GHCR
  → digest, bukan menyalin source laptop ke server.
- Testnet lama berhenti 20 Oktober **18.52 WIB**, tujuh order dibatalkan,
  query exchange nol open order; inventori dipertahankan. Timer review tanggal
  21 dibatalkan. Checkpoint server ini tidak diperiksa ulang pada development ini.
- Trial ETH di laptop dilarang. Marker/data `var/paper-eth/` dipertahankan.
  DB/container ETH tidak dinyalakan pada tes ini.
- Mainnet belum disetujui. Jangan menyimpulkan izin live dari perintah
  “selesaikan development”. P2-10 perlu keputusan dana/risk dan persetujuan live.
- Binance Spot prioritas. Bybit/xspread ditunda; futures menunggu Demo key.
  Saham tokenisasi tetap dikecualikan. Backup luar VPS ditunda sampai live.
- Shared Hindsight bank `pribadi`; recall saat mulai, jangan menyimpan secrets.
  Ikuti Conventional Commits satu baris, satu logical scope, tanpa trailer.

## Langkah pertama Claude

1. Baca `git status --short` dan `git log -12 --oneline`; commit lokal belum
   berarti sudah push/deploy. Lihat laporan tes untuk hasil akhir sesi Codex.
2. Selesaikan QA browser fixture dan review kode baru, terutama recovery
   checkpoint/inbox, partial OCO, unknown order, serta laporan risiko.
3. Jika ada perubahan, jalankan regression yang relevan dan lint/vet/race;
   DB test harus terpisah dengan nama `_test`. Jangan memakai database trial.
4. Persiapkan P2-9 sebagai rilis/trial baru dengan konfigurasi, DB, baseline dan
   rollback jelas. Jangan resume eksperimen testnet yang sudah ditutup.

Perintah, format input CLI dan prosedur recovery ada di
[managed-runtime.md](managed-runtime.md). Jangan mengandalkan ringkasan chat
sebagai pengganti status file atau bukti pengujian.

## Commit sesi Codex (lokal)

| Commit | Scope |
|---|---|
| `605ee4f` | schema managed wallet/journal |
| `d7a54d7` | ledger/reservasi awal |
| `876d472` | konfigurasi multi-instance |
| `8a0099c` | checkpoint engine/state strategi |
| `7ee6710` | pemulihan trade/komisi Spot asli |
| `f990561` | checkpoint view dan settlement fill Sim |
| `ebb769c` | runtime ledger/control/dashboard/tes managed |
| `0c46a05` | integrasi bot run/Telegram/lease/config paper |
| `4e7a25a` | CLI backtest modal bersama + packaging |
| `6ce3747` | CI integration dan migrasi keempat |
| `a6f3861` | gitignore ekspor full-state lokal |

Dokumentasi/handover di-commit setelah tabel ini. Ambil hash terakhir dengan
`git log -1 --oneline`; tidak perlu memperbarui hash self-reference di dokumen.
Seluruh pemeriksaan final lulus sesuai laporan tes. Database sementara dihapus,
DB development kembali stopped, tidak ada preview/bot lokal tersisa. Output
mentah smoke test di `/tmp/trading-bot-plan2-backtest/` bersifat sementara;
hanya ringkasan reproducible masuk Git. **Tidak ada push atau deploy.**
