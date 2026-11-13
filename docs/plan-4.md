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

## Checkpoint verifikasi — 10 November

Setelah merge trading-bot, user meminta uji P4-2–P4-4 dengan test dan sumber
publik gratis. Build, seluruh unit test (29 paket), race test, replay paper
fixture serta satu pass nyata CEX dan DEX lulus. Tiga bug ditemukan dan
diperbaiki di `c2a0863`: exchangeInfo melebihi batas 4 MB, simbol lama yang
ID 0-nya tidak tersimpan hanya ditahan, dan state BSC terpangkas sebelum pass
selesai. Discovery presale belum bisa menghasilkan kandidat karena daftar
PinkSale dirender JavaScript; user memilih menunda sumber seed. Rincian dan
batasnya ada di [checkpoint lokal](testing/plan-4-local.md#verifikasi-10-november-2026).

## Checkpoint sumber dan pass nyata — 12 November

Sumber seed presale diputuskan user: dua kanal Telegram sebagai petunjuk tautan
(satu resmi, satu ditandai tidak resmi), halaman detail PinkSale dan chain
sebagai bukti. Spider pinned di-build lokal dan crawl presale lulus. Ditambah
verifikasi clone/factory kontrak sale, GeckoTerminal sebagai metadata pool
kedua, dan venue keempat four.meme (kind `curve`, sebatas discovery). Dua pass
nyata di semua jalur lulus pada hari itu; pool yang semula ditandai asing
terbukti fair launch PinkSale dengan implementasi ketiga. Belum ada sale
presale yang masih terbuka, sehingga jalur presale belum pernah menghasilkan
kandidat selain `rejected`. Rincian:
[checkpoint lokal](testing/plan-4-local.md#verifikasi-12-november-2026).

## Checkpoint policy v2 — 13 November

User menyerahkan keputusan lanjutan ke agent dengan arahan memilih praktik
terbaik. Hasilnya: `sale_type` menjadi fakta kandidat presale; policy
`launch-paper-v2` menambah `curve_fraction` 1% (setara presale/DEX) dan prompt
`launch-analyst-v2` mengenal curve serta fair launch; paper memperlakukan trade
curve seperti swap DEX. Pass hari kedua dan pass `-limit 10` lulus. Dua request
`analyze` gagal di transport karena host gateway di env perangkat ini sudah
tidak berlaku, jadi kualitas model tetap belum teruji. Rincian:
[checkpoint lokal](testing/plan-4-local.md#pass-ketiga-analyze-dan-policy-v2--13-november-2026).

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
  gap/recovery, coverage dan budget. 10 November: tes lulus, pass nyata CEX/DEX
  lulus setelah perbaikan. 12 November: Spider di-build, crawl presale dan
  curve lulus, dua pass semua jalur lulus. Tersisa: coverage beberapa hari,
  sale presale yang masih terbuka, recovery/budget pada pass berulang.

Selesai ketika kandidat dapat ditelusuri sampai sumber, informasi basi/konflik
tidak meloloskan entry, dan penemuan tidak bergantung daftar ticker statis P3.

### P4-3 — Analisis dan policy simulasi

- [x] Tulis schema/prompt AI pada lifecycle baru dengan bukti, kontra-argumen,
  invalidation serta usulan entry/hold/exit atau aksi presale yang sesuai.
- [x] Sepakati modal virtual, sizing, exposure termasuk dana terkunci, halt,
  budget layanan/AI, interval monitoring dan batas biaya berdasarkan P4-1.
- [x] Bekukan `launch-paper-v1` dan `launch-evaluation-v1`; user memilih profil
  serta kuota melalui pertanyaan terstruktur. Pilot belum diaktifkan.
  13 November: `launch-paper-v2` menambah curve 1% atas keputusan agent yang
  didelegasikan user.
- [ ] Verifikasi kode/tes dan kualitas model nyata pada ketiga jalur; sumber
  P4-2 tetap harus diterima sebelum pilot. Checklist kode bukan hasil runtime.
  10 November: unit test lulus. 13 November: dua request inference gagal di
  transport (host gateway lama), belum ada keluaran model yang dinilai.

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
  10 November: tes/race dan replay fixture (idempotent) lulus; sumber
  quote/receipt nyata belum.

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
di `git log`. Tidak ada push, inference berbayar, deploy atau transaksi.
Tes/build dan pass CEX/DEX nyata baru dijalankan 10 November atas permintaan
user. Langkah berikutnya (status 12 November): pass berulang beberapa hari,
tipe sale sebagai fakta kandidat, lalu inference atas izin user; daftar
lengkapnya di [handover](handover-agent.md). Data,
ledger, modal dan T0 tetap dipertahankan; status server belum diperiksa.
[Handover](handover-agent.md) dan [arsip](archive.md) menyimpan jejak historis.
