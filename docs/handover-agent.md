# Handover agent — Presale dan listing baru

**Checkpoint 29 Oktober 2026.** [Plan 4](plan-4.md) adalah satu-satunya roadmap
aktif. Commit docs **`839fa2e`** dan discovery P4-1/P4-2 **`108b3fa`** selesai
sesuai permintaan user. Lanjutan terbaru menulis analisis/policy P4-3 sesuai
profil yang dipilih, lalu paper/simulasi P4-4; tes/build, evaluasi model baru,
integrasi runtime dan pilot belum dijalankan. Mulai dari [indeks docs](readme.md).

## Keputusan terbaru

- Ubah arah repo ini; docs dan bukti lama diarsipkan di lokasi yang tetap.
- Fokus presale mendatang/aktif dan listing DEX/CEX maksimal 30 hari per venue,
  termasuk coin lama yang baru listing dan memecoin.
- Target versi pertama mencakup ketiga jalur eksekusi otomatis secara bertahap.
- Pertahankan Go, Spider dan OmniRoute, tambah rancangan API/feed/on-chain.
- Hasil P4-1 memilih BSC 56, PinkSale, PancakeSwap V2 dan Binance Spot USDT;
  [matriks riset](presale-integrations.md) menjelaskan biaya/akses dan gap.
- User menyetujui profil baru: paper 1.000 USDT, dua exposure, presale/DEX
  1% budget full-loss, CEX 10% nominal/0,5% risiko, halt harian 2% dan drawdown
  8%; 12 request AI/hari WIB dan 2.048 output token/request. Lihat
  [kontrak P4-3](presale-analysis.md). Target otomatisasi bukan izin uang asli.
- User menyatakan masih memakai token Telegram lama dan memilih persiapannya
  nanti. P4-1/P4-2 tidak membutuhkannya; jangan membaca/mengubah/mengirim token.
- Folder kerja kini `D:\kodingan\personal\crypto-launch-bot`, remote Git kosong.
  Commit docs/discovery dan checkpoint P4-3/P4-4 diminta user;
  gunakan `git log` untuk hash checkpoint terbaru. Tidak ada
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

P4-3 menambah `launch-paper-v1`, helper sizing, context paper eksplisit,
schema/prompt `launch-analyst-v1`, validator lifecycle/kutipan dan aksi CLI
`analyze`. Reuse transport gateway dan kuota durable catalyst melalui API
tambahan; jalur P3 tetap memakai prompt/schema lama. Template `.env.launch.example`
tidak berisi key; tidak membuat/membaca env terisi selama implementasi.
`report/collect` tetap tanpa inference; analyze baru berjalan saat diminta
eksplisit, bukan oleh task implementasi ini. Semua entry tetap false.

P4-4 menambahkan journal `var/launch-paper`, replay sintetis dan adapter
fixture CEX/DEX/presale. Book matcher CEX dan archive dipakai ulang, ledger
P2/P3 tidak diubah. Cost basis/mark dibedakan; unknown valuation menahan entry,
partial claim hanya membuka quantity yang sudah diterima. Mode ini tidak
menerima output AI sebagai order otomatis atau mengirim transaksi venue.
[Kontrak paper](presale-paper.md), [fixture contoh](../configs/launch.paper.fixture.json).

Handover lama mencatat komponen/report P3 pernah dirilis dan pilot belum aktif
pada checkpoint-nya; status itu historis. Tidak ada pemeriksaan server atau
pelaksanaan tes baru dalam tugas ini. Jangan menyimpulkan proses sekarang
aktif/mati atau checkout sekarang lulus dari laporan lama.

## Langkah berikutnya

1. Selesaikan penerimaan P4-2–P4-4 saat ada tugas/izin verifikasi: tes/build relevan,
   crawl PinkSale, RPC/GoPlus/Binance, ABI/field/finality, error/recovery serta
   coverage/budget. Jangan menyatakan lulus dari gofmt atau checklist kode.
2. Gap v1 terlihat di laporan: crawl maksimal lima halaman, tail pool terbatas,
   rotasi simbol CEX, sale ABI/factory belum terverifikasi, tanggal pool bukan
   trade pertama, dan policy holder/likuiditas/kemampuan exit belum ditetapkan.
   DEX/presale ditahan; tidak mengarang mapping kontrak dari ticker Binance.
3. Verifikasi schema/prompt pada model nyata, baseline, journal recovery dan
   adapter sumber nyata sebelum monitoring/pilot P4-5. Policy/metode P4-3 dibekukan;
   tidak boleh dinaikkan diam-diam oleh model atau config. Gunakan
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
