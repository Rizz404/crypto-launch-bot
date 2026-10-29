# Handover agent

Untuk Codex, Claude dan agent berikutnya. Checkpoint **29 Oktober 2026**;
verifikasi checkout dan waktu bukti sebelum bekerja.

## Status dan prioritas

**Lanjutan yang disetujui user 29 Oktober:** dua EMA 1m sudah pause entry pada
21:10 WIB. Backup/restore P3 dan QA Chromium desktop/mobile selesai; observer
sekali jalan mendeteksi review invalid. Patch feed/health, schema simbol,
tampilan keputusan arsip dan helper/timer P3 siap lokal, **belum commit/push/
CI/deploy** pada checkpoint ini. User sudah mengizinkan commit/push/deploy dan
pemasangan timer P3; lanjutkan rilis dalam scope tersebut. Dua belas replay
biaya selesai, tanpa aktivasi strategi baru.
Mulai dari [laporan lanjutan](testing/operations-follow-up.md) dan
[riset terbatas](testing/strategy-cost-review/readme.md).


- **Plan 3 tetap roadmap aktif repo Rizz404/trading-bot.** User meluruskan
  29 Oktober bahwa Plan 4 dari memori Windows adalah proyek/repo GitHub lain.
  Fakta sumber Hindsight dikoreksi; jangan mengarsipkan Plan 3 karena memori itu.
- **Review tujuh hari P2 selesai:** snapshot VPS 29 Oktober 20:34 WIB,
  equity 740,40558347 USDT, modal 747,83957456, PnL −7,43399109 (−0,9941%).
  EMA dasar/tren 1m merugi; tiga Tren dan EMA tren 4h belum bertransaksi.
  Komisi 5,77387876 USDT sudah termasuk realized, jangan dikurangkan lagi.
  [Analisis/metode/tindak lanjut](testing/research-stage-two-review.md),
  [bukti 214 fill](testing/research-stage-two-review.json).
- **Penerimaan operasi P2 belum lulus:** candle 1m tertinggal 450 detik,
  mark ledger sekitar 31,5 menit; 140 baris `Pong timeout` pada log 24 jam.
  Health proses tetap 200. Diagnosis feed/consumer dan health freshness prioritas
  berikutnya; root cause belum dibuktikan. Pause dua EMA 1m sudah
  disetujui/diterapkan pukul 21:10 WIB, snapshot setelahnya flat; empat instance
  lain tetap running. Patch belum diuji pada beban VPS.
- P2 source `9c18962`, start 23 Oktober 20:52 WIB, restart 0/OOM false.
  Enam instance/18 binding, DB `tradingbot_research_stage_two`, wallet
  `paper:research-stage-two`. **T0 tetap 22 Oktober 19:58:45 WIB.** Tidak
  ada restart P2; pause entry adalah perubahan kontrol setelah review, tanpa
  mengubah parameter/modal/T0 atau menghapus data.
- **Pilot P3 sudah aktif sejak 28 Oktober 22:53 WIB**, bukan WIP yang belum
  dirilis. Source `1779878`/digest dan identitas di [handover VPS](handover-vps.md).
  Pemeriksaan 29 Oktober: container running, restart 0/OOM false, runtime/feed
  health 200. Snapshot 21:14 WIB: modal/kas 1.000, tanpa fill/posisi, tiga sumber
  sehat, kuota 4/12 hari ini; review formal hari 1 belum jatuh tempo.
- P3: modal paper 1.000 USDT, risiko 0,5%/trade, maksimal dua posisi,
  halt rugi harian 2%/drawdown 8%. Kuota AI **12 panggilan/hari WIB**, maksimal
  2.048 token output/call; hari risiko ledger tetap UTC. DB/wallet/modal terpisah
  dari P2. Model `cx/gpt-6-luna-low`, prompt v5. Sumber Ethereum/Anza/Dogecoin;
  entry tetap memerlukan bukti primer/jadwal yang memenuhi validator.
- **P3 tersisa:** pemasangan timer backup/observer dari rilis GitHub dan
  review hari 1/3/7 pada **29 Oktober /31 Oktober /4 November 22:53 WIB**.
  Backup/restore flat dan QA browser awal selesai; patch UI belum dirilis.
  Jangan start ulang atau reset modal/T0 untuk meneruskan pekerjaan.
- [Handover pilot](catalyst-pilot-handover.md) dan
  [handover Spider](catalyst-spider-handover.md) menyimpan checkpoint historis
  pause/WIP. Baca bagian lanjutan/start; instruksi menunggu resume lama sudah
  digantikan resume user dan rilis/start malam 28 Oktober.
- Telegram P3 sender-only memakai bot/chat P2, tanpa poller/menu tambahan.
  Penerimaan fill alami/digest/whitelist P2 belum lengkap; startup P2 pernah
  dikonfirmasi user. Restore backup pascarilis P2, recovery posisi/order aktif,
  outage dan klasifikasi shutdown tetap di P3-O. Backup luar server ditunda
  sampai live sesuai keputusan user.
- Mainnet belum disetujui. Ekspansi pembanding 44/132, riset top-20 dan
  strategi tambahan tetap P3-R; futures/Bybit/xspread/funding tetap bersyarat
  P3-D. Tidak menghapus strategi/ledger lama untuk memberi tempat pada AI.

## Lanjutkan dari sini

1. Periksa `git status --short` dan `git log -12 --oneline`; pertahankan
   perubahan belum di-commit. Jangan commit/push/deploy hanya karena izin
   historis untuk rilis sebelumnya; ikuti cakupan permintaan saat ini.
2. Baca [laporan lanjutan](testing/operations-follow-up.md). Patch lokal mengurangi
   transaksi no-op dan routing simbol tak terkait, serta memasukkan freshness
   consumer ke health. Race/vet/lint lulus. Setelah izin commit/push eksplisit,
   rilis lewat CI/GHCR dan ukur ulang feed/Pong/beban di VPS; jangan mengklaim
   gejala sudah sembuh sebelum rollout. Pause EMA 1m dipertahankan.
3. Backup/restore P3 flat sudah lulus, worker yang sama kembali aktif pada
   21:18:56 WIB tanpa reset T0. Helper/timer scoped sudah disiapkan dan observer
   diuji sekali jalan. Pasang timer dari checkout GitHub sesudah izin rilis;
   jangan mengulang restore sebagai syarat tanpa risiko baru. Pertahankan
   keterbatasan recovery posisi/order aktif dan QA form/Safari.
4. Review P3 hari 1 mulai 29 Oktober 22:53 WIB; bedakan feed/runtime dari
   kesiapan riset, kuota model, keputusan validator dan eksekusi. Jangan
   menganggap health 200 sebagai bukti seluruh pilot sehat.
5. Tetap simpan ETH/tahap satu yang dihentikan. Jangan mengaktifkan ulang
   trial lama; eksperimen baru memakai ledger/DB baru sesuai [deployment](deployment.md).

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
