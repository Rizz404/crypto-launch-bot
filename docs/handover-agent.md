# Handover agent — Presale dan listing baru

**Checkpoint 29 Oktober 2026.** [Plan 4](plan-4.md) adalah satu-satunya roadmap
aktif. Perombakan ini dokumentasi saja; tidak ada implementasi atau verifikasi
runtime baru. Mulai dari [indeks docs](readme.md).

## Keputusan terbaru

- Ubah arah repo ini; docs dan bukti lama diarsipkan di lokasi yang tetap.
- Fokus presale mendatang/aktif dan listing DEX/CEX maksimal 30 hari per venue,
  termasuk coin lama yang baru listing dan memecoin.
- Target versi pertama mencakup ketiga jalur eksekusi otomatis secara bertahap.
- Pertahankan Go, Spider dan OmniRoute, tambah rancangan API/feed/on-chain.
- Riset dulu untuk memilih chain/launchpad/DEX/CEX awal. Rancang ulang modal,
  limit risiko dan budget sebelum pilot; angka P3 tidak otomatis berlaku.
- Target otomatisasi bukan izin uang asli. Pekerjaan ini tidak mencakup kode,
  config, database, runtime VPS, tes/build, commit/push atau deploy.

## Kondisi yang diketahui

Inspeksi statis menemukan collector/arsip Spider, adapter AI/schema, policy
berbasis simbol USDT, market Binance, runner paper, ledger/Sim, dashboard dan
Telegram. Komponen ini kandidat reuse, bukan bukti kesiapan Plan 4. Schema
P3 belum menjadi identitas chain/kontrak atau lifecycle presale/DEX.
Lihat [pemetaan reuse](presale-architecture.md#kode-saat-ini-dan-reuse).

Handover lama mencatat komponen/report P3 pernah dirilis dan pilot belum aktif
pada checkpoint-nya; status itu historis. Tidak ada pemeriksaan server atau
pelaksanaan tes baru dalam tugas ini. Jangan menyimpulkan proses sekarang
aktif/mati atau checkout sekarang lulus dari laporan lama.

## Langkah berikutnya

1. Kerjakan P4-1 sesuai [protokol sumber](presale-sources.md): verifikasi sumber
   resmi, akses, tarif/kuota, kemampuan dan gap dengan tanggal pemeriksaan.
2. Pilih integrasi awal untuk presale, DEX dan CEX beserta urutan implementasi.
   Belum ada pemilihan chain/venue atau langganan berbayar pada checkpoint ini.
3. Bekukan kontrak dan policy sebelum implementasi/pilot terkait. Gunakan
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
