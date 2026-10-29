# Handover agent — Presale dan listing baru

**Checkpoint 29 Oktober 2026.** [Plan 4](plan-4.md) adalah satu-satunya roadmap
aktif. User meminta commit docs lalu P4-1/P4-2; commit docs **`839fa2e`** selesai.
Riset integrasi dan kode development discovery/verifikasi tersedia; tes/build,
integrasi runtime dan pilot belum dijalankan. Mulai dari [indeks docs](readme.md).

## Keputusan terbaru

- Ubah arah repo ini; docs dan bukti lama diarsipkan di lokasi yang tetap.
- Fokus presale mendatang/aktif dan listing DEX/CEX maksimal 30 hari per venue,
  termasuk coin lama yang baru listing dan memecoin.
- Target versi pertama mencakup ketiga jalur eksekusi otomatis secara bertahap.
- Pertahankan Go, Spider dan OmniRoute, tambah rancangan API/feed/on-chain.
- Hasil P4-1 memilih BSC 56, PinkSale, PancakeSwap V2 dan Binance Spot USDT;
  [matriks riset](presale-integrations.md) menjelaskan biaya/akses dan gap.
- Rancang ulang modal, limit risiko dan budget sebelum pilot; angka P3 tidak
  otomatis berlaku. Target otomatisasi bukan izin uang asli.
- User menyatakan masih memakai token Telegram lama dan memilih persiapannya
  nanti. P4-1/P4-2 tidak membutuhkannya; jangan membaca/mengubah/mengirim token.
- Folder kerja kini `D:\kodingan\personal\crypto-launch-bot`, remote Git kosong.
  Commit docs dilakukan atas permintaan. Implementasi lanjutan belum di-commit;
  tidak ada push/deploy, perubahan DB atau runtime VPS.

## Kondisi yang diketahui

Inspeksi statis menemukan collector/arsip Spider, adapter AI/schema, policy
berbasis simbol USDT, market Binance, runner paper, ledger/Sim, dashboard dan
Telegram. Komponen ini kandidat reuse, bukan bukti kesiapan Plan 4. Schema
P3 belum menjadi identitas chain/kontrak atau lifecycle presale/DEX.
Lihat [pemetaan reuse](presale-architecture.md#kode-saat-ini-dan-reuse).

P4-2 menambahkan `cmd/launch`/`internal/launch`, fungsi discovery jaringan/Spider
tambahan pada catalyst, ignore `var/launch` dan target build CLI. Tidak ada
dependency/migrasi/config uang baru. Sumber API dan RPC dibatasi allowlist;
presale tetap Spider/robots pinned, tanpa fallback HTTP. Archive immutable
terpisah; laporan memisahkan status accepted-for-research/held/rejected dan
selalu mematikan entry. Pemakaian archive lama mengharuskan Linux/WSL/container.
[Panduan CLI](presale-runtime.md), [checkpoint](testing/plan-4-local.md).

Handover lama mencatat komponen/report P3 pernah dirilis dan pilot belum aktif
pada checkpoint-nya; status itu historis. Tidak ada pemeriksaan server atau
pelaksanaan tes baru dalam tugas ini. Jangan menyimpulkan proses sekarang
aktif/mati atau checkout sekarang lulus dari laporan lama.

## Langkah berikutnya

1. Selesaikan penerimaan P4-2 saat ada tugas/izin verifikasi: tes/build relevan,
   crawl PinkSale, RPC/GoPlus/Binance, ABI/field/finality, error/recovery serta
   coverage/budget. Jangan menyatakan lulus dari gofmt atau checklist kode.
2. Gap v1 terlihat di laporan: crawl maksimal lima halaman, tail pool terbatas,
   rotasi simbol CEX, sale ABI/factory belum terverifikasi, tanggal pool bukan
   trade pertama, dan policy holder/likuiditas/kemampuan exit belum ditetapkan.
   DEX/presale ditahan; tidak mengarang mapping kontrak dari ticker Binance.
3. Lanjutkan policy dan evaluasi AI P4-3 setelah bukti penerimaan terkait cukup.
   Bekukan policy sebelum pilot. Gunakan
   [strategi](presale-strategy.md), [risiko](presale-risk.md) dan
   [evaluasi](presale-evaluation.md), bukan daftar resume P3.

## Perlindungan proyek lama

- Jangan resume pilot P3 karena membaca instruksi lama. Izin rilis/commit/push
  historis tidak menjadi izin otomatis untuk Plan 4.
- Pertahankan kode, WIP, konfigurasi, database, ledger, history, modal dan T0
  eksperimen lama; jangan stop/restart, reset atau clean sebagai bagian pivot.
- VPS berbagi layanan lain; jangan restart host/Docker atau deploy ulang
  komponen lama untuk memasang Plan 4. Proses/DB/wallet baru kelak terisolasi.
- Jaga invariant ledger lama: satu pemilik tulis, reservasi sebelum I/O,
  dedup settlement dan inventori per pemilik. Jangan down migrasi DB berisi data.
- Secret tidak masuk docs, prompt, log atau memori. Jangan membaca env privat
  untuk memperbarui narasi dokumentasi.
- Jangan menjalankan tes, build, aplikasi, server atau runtime verification
  kecuali diminta atau tugasnya memang verifikasi tersebut.

[Arsip](archive.md) mempertahankan seluruh panduan lama. Teks handover sebelum
pivot disimpan di [handover historis](handover-agent-legacy.md); checkpoint
teknis di sana bukan antrean kerja aktif.
