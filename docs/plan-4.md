# Plan 4 — Bot presale dan listing baru

## Keputusan dan status

**Satu-satunya roadmap aktif sejak 29 Oktober 2026.** Repo ini berganti arah,
bukan membuat repo terpisah. Pekerjaan pembukaan Plan 4 hanya dokumentasi.
Plan 1–3 ditutup sebagai roadmap aktif; penutupan tidak berarti seluruh fase
lama lulus. Backlog lama tidak otomatis menjadi prasyarat Plan 4.

Target versi pertama mencakup discovery, analisis dan eksekusi otomatis
presale, DEX serta CEX, termasuk meme, dengan implementasi bertahap. Tidak
ada izin uang asli. Pilihan integrasi discovery dan policy paper terbaru ada
pada checkpoint di bawah.
Universe: presale mendatang/aktif dan listing berumur maksimal 30 hari pada
masing-masing venue; coin lama dengan listing baru tetap dihitung.

Fondasi Go → Spider/API/feed/on-chain → arsip → verifikasi → OmniRoute/AI →
validator risiko → eksekusi → monitoring. Rincian: [strategi](presale-strategy.md),
[arsitektur](presale-architecture.md), [sumber](presale-sources.md),
[risiko](presale-risk.md), [evaluasi](presale-evaluation.md).

## Checkpoint P4-1/P4-2 — 29 Oktober

Atas permintaan user, docs di-commit lebih dahulu pada **`839fa2e`**, lalu
riset P4-1 dan penulisan implementasi P4-2 dilanjutkan. P4-1 memilih BSC 56,
PinkSale, PancakeSwap V2 dan Binance Spot; [matriks dan kontrak](presale-integrations.md)
memuat bukti referensi, budget serta gap akses. CLI [discovery/report](presale-runtime.md)
tersedia sebagai development, tanpa AI, Telegram, wallet atau transaksi.

Checklist P4-2 di bawah menandai **kode yang ditulis**, bukan runtime yang
lulus. Tes/build dan akses sumber melalui aplikasi belum dijalankan;
[checkpoint lokal](testing/plan-4-local.md). Penerimaan runtime P4-2 tetap
terbuka; jangan mulai pilot atau menyatakan integrasi nyata lulus dari checklist.

## Checkpoint P4-3 — 29 Oktober

P4-1/P4-2 di-commit **`108b3fa`** atas permintaan user. User kemudian memilih
profil paper 1.000 USDT, dua exposure, presale/DEX 1% full-loss budget, CEX
10% nominal/0,5% risiko rencana, halt harian 2% dan drawdown 8%, serta
12 request AI/hari WIB dan 2.048 output token/request. Ini keputusan baru
Plan 4, bukan pewarisan otomatis P3.

Kode/schema/prompt, policy versioned, sizing paper, validasi lifecycle,
kuota durable dan CLI analyze sudah ditulis. [Kontrak P4-3](presale-analysis.md)
memuat metode simulasi/evaluasi dan batas biayanya. Belum ada tes/build,
inference/model evaluation nyata atau pilot. P4-2 runtime tetap belum diterima.

## Checkpoint P4-4 — 29 Oktober

User meminta fase berikutnya; implementasi paper P4-4 dilanjutkan tanpa
menutup penerimaan P4-2/P4-3. Tersedia journal virtual terisolasi dan adapter
skenario sintetis CEX/DEX/presale dengan reservasi, receipt kumulatif,
allocation/vesting/refund, finality/reorg dan recovery. Reuse matcher book
CEX dan archive, tanpa mengubah ledger/runtime lama.

[Kontrak paper](presale-paper.md) menjelaskan mode fixture, cost versus mark,
guard dan batasnya. Fixture contoh tiga jalur serta test source ditulis,
tetapi belum dijalankan. Wire quote/receipt sumber nyata dan ABI transaksi
launchpad belum diterima; tidak ada signer, order live atau pilot.

## Fase dan kriteria selesai

### P4-0 — Reset dokumentasi

- [x] Ganti README, indeks dan handover; terbitkan rancangan aktif Plan 4.
- [x] Indeks arsip dan penanda historis menggantikan arahan resume P3.
- [x] Pertahankan path, kode/config dan bukti lama tanpa mengubah runtime.

### P4-1 — Riset dan pembekuan integrasi

- [x] Riset sumber primer, chain, launchpad, DEX, CEX dan provider sesuai
  [protokol sumber](presale-sources.md); catat URL resmi, tanggal verifikasi,
  cakupan, biaya, batas akses dan gap.
- [x] Pilih integrasi awal untuk ketiga jalur; tetapkan urutan implementasi,
  akses yang layak, kebutuhan wallet serta batas kapasitas/penyimpanan.
- [x] Bekukan kontrak identitas, lifecycle, freshness, finality dan adapter
  berdasarkan venue terpilih; jangan memperlakukan ticker sebagai identitas.

Selesai ketika matriks pilihan dan alasan penolakannya tersedia, tidak ada
jalur presale/DEX/CEX tanpa integrasi sasaran, dan dependensi akses diketahui.

### P4-2 — Discovery dan verifikasi

- [x] Tulis discovery kandidat dinamis dan arsip bukti/first-seen/revisi/gap.
- [x] Pisahkan aset, sale dan listing/pool; deduplikasi informasi, bukan identitas
  listing yang memang berbeda. Tangani jadwal, konflik dan identitas ambigu.
- [x] Tulis pemeriksaan pasar/on-chain serta laporan terima/tahan/tolak;
  seluruh entry tetap mati. Verifikasi pra-transaksi menjadi kewajiban P4-4.
- [ ] Terima hasil tes dan sumber nyata: parsing/robots, API/RPC, finality,
  gap/recovery, coverage dan budget; belum dijalankan pada checkpoint ini.

Selesai ketika kandidat dapat ditelusuri sampai sumber, informasi basi/konflik
tidak meloloskan entry, dan penemuan tidak bergantung daftar ticker statis P3.

### P4-3 — Analisis dan policy simulasi

- [x] Tulis schema/prompt AI pada lifecycle baru dengan bukti, kontra-argumen,
  invalidation serta usulan entry/hold/exit atau aksi presale yang sesuai.
- [x] Sepakati modal virtual, sizing, exposure termasuk dana terkunci, halt,
  budget layanan/AI, interval monitoring dan batas biaya berdasarkan P4-1.
- [x] Bekukan `launch-paper-v1` dan `launch-evaluation-v1`; user memilih profil
  serta kuota melalui pertanyaan terstruktur. Pilot belum diaktifkan.
- [ ] Verifikasi kode/tes dan kualitas model nyata pada ketiga jalur; sumber
  P4-2 tetap harus diterima sebelum pilot. Checklist kode bukan hasil runtime.

Selesai ketika output AI tervalidasi, tidak dapat menaikkan batas policy, dan
ketiga jalur punya aturan simulasi serta parameter yang disepakati.

### P4-4 — Simulasi dan adapter eksekusi

- [x] Tulis simulasi presale terpisah dari fill DEX/CEX; dokumentasikan bagian
  yang tidak dapat dimodelkan secara andal, jangan mengarang fill/return.
- [x] Tulis adapter paper/sintetis ketiga jalur secara bertahap, terpisah dari
  ledger/proses lama; catat intent, reservasi, status eksternal dan settlement.
- [x] Tulis penanganan timeout/status unknown, restart, partial fill, claim/refund,
  vesting, finality/reorg dan transaksi gagal tanpa aksi finansial ganda.
- [ ] Terima hasil tes/regresi, recovery dan sumber quote/receipt nyata sebelum
  pilot. Implementasi fixture bukan bukti integrasi venue atau runtime lulus.

Selesai ketika pengujian relevan pada [rencana evaluasi](presale-evaluation.md)
punya bukti baru dan tidak ada adapter yang diam-diam memakai uang asli.

### P4-5 — Monitoring dan penerimaan pilot

- [ ] Dashboard/Telegram menampilkan sumber/gap, alasan keputusan, saldo
  tersedia/terkunci, posisi, status transaksi dan biaya per jalur.
- [ ] Proteksi dan rekonsiliasi berjalan independen dari latensi/gagalnya AI;
  hentikan entry pada gap kritis tanpa memalsukan keberhasilan exit.
- [ ] Setujui kapasitas host, DB/wallet terisolasi, T0, durasi, metrik,
  kriteria stop dan jadwal review sebelum mulai pilot prospektif.

Selesai ketika pilot dijalankan dengan izin baru dan hasil per jalur dilaporkan
termasuk kegagalan serta keterbatasan; keberhasilan satu jalur tidak mewakili tiga.

### P4-6 — Gerbang live

- [ ] Review hasil pilot dan batas model simulasi; selesaikan temuan kritis.
- [ ] Tetapkan budget uang asli, chain/venue, kewenangan signer, batas exposure,
  kill switch, backup/recovery dan prosedur insiden.
- [ ] Dapatkan persetujuan eksplisit aktivasi live untuk integrasi dan limit
  yang konkret. Kelulusan tes/paper tidak memberikan izin ini.

## Batas dan langkah berikutnya

Pekerjaan berikutnya adalah penerimaan P4-2–P4-4 sebelum monitoring/pilot P4-5,
bukan resume pilot P3. Commit docs dan discovery sudah dibuat sesuai permintaan;
checkpoint P4-3/P4-4 kemudian diminta user untuk di-commit; hash terbaru ada
di `git log`. Tidak ada push, inference berbayar,
runtime aplikasi, tes/build, deploy atau transaksi dalam lanjutan ini. Data,
ledger, modal dan T0 tetap dipertahankan; status server belum diperiksa.
[Handover](handover-agent.md) dan [arsip](archive.md) menyimpan jejak historis.
