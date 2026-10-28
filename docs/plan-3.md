# Plan 3 — Riset katalis dan hold berbasis tesis

**Rencana aktif sejak 23 Oktober 2026 atas permintaan user.** Plan 2 ditutup
sebagai siklus pengembangan; pekerjaan yang belum terlaksana diteruskan ke sini.
Penutupan bukan kelulusan semua pengujian atau izin uang asli. Dokumentasi
baseline sudah di-commit sebagai `bc321b8`; development komponen katalis dan
replay paper lokal dimulai sesudahnya. Belum ada worker/pilot AI yang aktif.

Plan 3 mencakup dua jalur: menyelesaikan penerimaan/riset platform lama dan
membangun eksperimen katalis dengan hold berbasis tesis. Paper pembanding lama
tetap memakai ledger, parameter dan T0 sendiri; tidak dihentikan, diubah atau
digabung modalnya hanya karena pergantian plan. Sumber status operasi:
[handover VPS](handover-vps.md); bukti lama: [Plan 2](plan-2.md) dan
[indeks pengujian](testing/readme.md).

## Klarifikasi alur dan handover 26 Oktober

User menegaskan **daftar website crypto → scraping/crawling dengan Spider →
AI dengan konteks data pasar menentukan kapan buy/hold/sell → validasi risiko
→ eksekusi paper**. Spider menjadi collector utama. Universe riset tidak
terkunci ke tiga simbol contoh; proteksi tetap berjalan independen dari AI.
Implementasi dijeda untuk handover 26 Oktober lalu dilanjutkan 28 Oktober.
CLI kini memakai Spider; extractor/cache/arsip/proxy memiliki race test fixture
lokal yang lulus. Image khusus dibangun/smoke; binary dengan patch flush stdout
(2.53.9-p3.1) lulus protokol/proxy TLS fixture tiga kali tanpa jaringan keluar.
CI disiapkan, egress adversarial/situs publik belum diuji. Tidak ada instalasi/start P3 di VPS.

[Handover implementasi dan urutan lanjut](catalyst-spider-handover.md) mencatat
hasil inspeksi kapasitas VPS, usulan container/project tersendiri, file WIP,
temuan cache/ukuran arsip/proxy serta batas bukti model v3 versus kode v4.

## Checkpoint development 23 Oktober

**Pembaruan 26 Oktober:** user mengisi key OmniRoute dan mengizinkan evaluasi
model nyata di `https://route-gateway.fbe.co.id/v1`. `.env.catalyst` privat
(0600, diabaikan Git) sekarang memilih `cx/gpt-6-luna-low`. CLI `analyze`
membaca env tersebut atau `-env-file`, dengan environment proses lebih tinggi.
Evaluasi tujuh kasus sintetis memakai inference nyata: Luna 7/7, cadangan
`openrouter/anthropic/claude-haiku-5.5` 6/7 (satu HTTP 403). Rekomendasi combo
`p3-research` memakai urutan priority; combo belum dibuat atau diuji failover.
[Bukti dan batas evaluasi](testing/catalyst-model-evaluation.md).
Tidak mengubah proses/config/ledger P2 atau memasang layanan baru.

Pilihan awal 9router dan penundaan AI pada 23 Oktober merupakan riwayat;
arahan evaluasi OmniRoute di atas menggantikannya. Satu inference per keputusan
cukup untuk jalur awal, dengan validasi deterministik sebelum aksi paper.

| Area | Hasil lokal | Penerimaan yang masih terbuka |
|---|---|---|
| P3-0 | Kontrak development, batas proses dan titik integrasi ditetapkan | Freeze policy pilot dan resource budget nyata |
| P3-1 | Spider default CLI; extractor/cache/arsip/proxy fixture dan image/binary patched lulus lokal; Compose config lulus, CI disiapkan | Egress adversarial, kontrak situs nyata, event dan retensi; CI belum dijalankan di GitHub |
| P3-2 | Adapter OmniRoute tanpa tools, strict JSON Schema dan evaluasi model nyata pada tujuh kasus sintetis | Shadow prospektif, baseline tanpa AI, combo/failover dan biaya nyata |
| P3-3/P3-4 | Gate entry, reuse managed/Sim, checkpoint, trailing, expiry/revocation dan full exit diuji sintetis | Registry verifikasi tepercaya, feed/book/ATR nyata, durability/outage dan runner |
| P3-5 | CLI report keputusan/sumber/token dan replay berlabel fixture | Dashboard/Telegram/metrics AI dan kontrol pilot |
| P3-6 | Pengujian lokal komponen serta regression repo | E2E pasar/model nyata, DB sementara pilot dan review operasional |
| P3-7 | Belum dimulai | Kesiapan, izin deployment/start, T0 baru dan pengamatan prospektif |

Kode: `internal/catalyst`, CLI `cmd/catalyst`, policy contoh
`configs/catalyst.policy.json`. [Kontrak dan penggunaan](catalyst-runtime.md),
[bukti/checkpoint pengujian](testing/plan-3-local.md). Tabel ini tidak menandai
seluruh fase selesai hanya karena fixture berjalan. P3-R/P3-O P2 belum dikerjakan.

## Prioritas dan perlindungan eksperimen P2

Penegasan user 23 Oktober: eksperimen P2 yang sedang berjalan dibiarkan;
kode, konfigurasi, database, ledger, history dan bukti tetap disimpan agar
riset P2 dapat dilanjutkan sewaktu-waktu. Status selesai/arsip hanya menutup
siklus rencana, bukan menghapus kemampuan atau menghentikan proses P2.

Prioritas pengembangan sekarang **P3-0 lalu fase AI berikutnya**. P3-R dan
backlog P2 lainnya tetap tercatat untuk dilanjutkan ketika diperlukan;
penyelesaiannya bukan prasyarat umum pengembangan AI. Temuan keselamatan yang
benar-benar memengaruhi komponen pilot tetap wajib ditangani sebelum pilot
diaktifkan.

P3-0 berupa inspeksi/desain dan pembekuan kontrak tanpa perubahan runtime P2.
Implementasi P3 berikutnya menjaga konfigurasi lama tetap kompatibel dan
tidak membersihkan strategi lama sebagai bagian refactoring. Perubahan library
bersama tidak otomatis dirilis ke proses P2. Pilot AI memakai konfigurasi,
DB/wallet/modal serta checkpoint sendiri; tidak berbagi pemilik tulis ledger
atau mengambil alokasi P2. Isolasi proses tidak menjamin isolasi kapasitas:
sebelum collector/pilot berjalan di VPS yang sama, ukur dan batasi CPU/RAM,
disk/DB serta kuota API agar feed/proteksi P2 tidak terganggu.

## Keputusan user

- Fokus awal pada token yang sudah diperdagangkan; memecoin termasuk universe.
- Presale dan token baru ditunda, tetapi disimpan sebagai cakupan masa depan.
- Waktu keluar bergantung pada kondisi masing-masing koin. Acara selesai
  tidak otomatis mengharuskan jual; fundamental/tesis yang masih baik dapat
  mendukung hold.
- Penurunan signifikan memicu penjualan seluruh sisa posisi strategi ini.
  Definisi numerik penurunan tersebut belum disepakati.

## Hubungan dengan referensi awal

Referensi diskusi: `plan-strategi-ai-trading.md` dan
`Daftar_Website_Crypto_Presale_dan_Peluncuran_2026-10-23.md` yang diberikan user.
Keduanya bahan referensi, bukan instruksi aktivasi atau bukti runtime.
Checkpoint implementasi Hermes dalam lampiran belum diverifikasi di repo ini.

Aturan referensi lama tentang exit satu jam sebelum acara dan maksimum hold
48 jam tidak otomatis diadopsi. Rancangan baru mengevaluasi kelayakan hold
melewati acara. Angka modal, sizing dan trailing lampiran tidak otomatis menjadi
kebijakan pilot. Sebagian dipilih sebagai kontrak development/fixture yang
tercatat di [runtime katalis](catalyst-runtime.md), tanpa mengubah konfigurasi P2.

## Usulan alur

1. **Discovery.** Ambil daftar website terkonfigurasi melalui Spider sebagai
   engine scraping/crawling utama. Temukan kandidat, arsipkan sumber asli,
   `first_seen_at` dan revisinya. Gabungkan salinan berita menjadi satu event.
2. **Verifikasi.** Cocokkan aset, venue, jadwal, sumber primer dan konflik
   informasi. Pilot yang diusulkan: Binance Spot, USDT, long-only, paper.
   Aset di luar venue teruji tetap watchlist.
3. **Analisis tesis.** AI menjelaskan informasi baru, mekanisme dampak token,
   bukti pendukung, kontra-argumen, indikasi sudah diantisipasi pasar dan
   kondisi tesis batal. Hasil: tolak, pantau atau kandidat entry.
4. **Watchlist.** Tunggu jendela entry dan harga yang layak. Penemuan acara
   tidak langsung memicu beli. Analisis ulang untuk perubahan material,
   menjelang entry dan pada jadwal evaluasi yang ditentukan sebelumnya.
5. **Rencana posisi.** Sebelum entry, simpan versi tesis, batas harga entry,
   stop awal, aturan trailing, ukuran posisi, kondisi exit dan tenggat review.
   Profil risiko mengikuti volatilitas dan likuiditas dalam batas kebijakan;
   AI tidak dapat memperbesar batas risiko.
6. **Validasi dan entry.** Program memeriksa bukti, freshness, spread, depth,
   biaya, saldo, reservasi, exposure dan identitas keputusan. Semua lolos
   baru order paper. Harga keburu melampaui batas berarti tunggu atau lewatkan.
7. **Hold dan pantau.** Program menjaga proteksi harga independen dari AI.
   Collector memantau revisi katalis, sedangkan AI menilai perubahan tesis.
   Saat acara berlangsung/selesai, evaluasi apakah alasan hold masih valid;
   hold lanjutan perlu bukti baru/terkini dan tenggat review berikutnya.
8. **Full exit.** Stop/trailing tersentuh, tesis batal yang tervalidasi atau
   aturan risiko dapat memicu jual seluruh sisa posisi milik strategi.
   Fundamental yang dianggap baik tidak mengalahkan proteksi harga.
9. **Rekonsiliasi dan rotasi.** Pastikan fill, fee, sisa posisi dan reservasi
   selesai sebelum dana dipakai lagi. Kandidat baru harus lolos semua gate;
   tetap tunai diperbolehkan. Re-entry perlu evaluasi baru, bukan langsung
   membeli kembali posisi yang baru terkena stop.
10. **Evaluasi eksperimen.** Catat seluruh kandidat, penolakan, keputusan,
    perubahan tesis, fill, biaya dan hasil. Bandingkan AI dengan aturan event
    sederhana tanpa AI serta benchmark dengan modal/risiko yang sebanding.

## Fundamental dan memecoin

Usulan: pisahkan bukti penggunaan produk/permintaan token dan tokenomics dari
indikator komunitas, narasi dan perhatian pasar. Untuk memecoin, aktivitas
komunitas tidak otomatis membuktikan fundamental produk atau permintaan yang
bertahan. Klaim penggunaan tetap perlu diverifikasi pada tiap aset.

Kandidat memecoin tetap dapat dipelajari berdasarkan katalis/narasi, dengan
bukti yang sesuai, likuiditas memadai dan batas exposure lebih kecil yang
ditetapkan sebelum pilot. Tren harga atau popularitas saja tidak cukup untuk
menyimpulkan aset aman di-hold. Data yang tidak tersedia dicatat sebagai
ketidakpastian, bukan diisi oleh model.

## Proteksi penurunan yang diusulkan

- Stop awal membatasi risiko sejak entry; trailing mengikuti puncak harga
  valid setelah entry untuk membatasi pengembalian laba belum terealisasi.
- Batas harga proteksi hanya naik atau tetap, tidak dilonggarkan karena AI
  masih optimistis. Parameter dibekukan sebelum entry; pergantian versi
  kebijakan harus tercatat dan tidak otomatis diterapkan pada posisi lama.
- Jarak stop/trailing mempertimbangkan volatilitas dan likuiditas. Jika ruang
  yang dibutuhkan melampaui batas risiko, kecilkan ukuran atau tolak entry.
- Trigger penurunan memakai data venue yang tervalidasi. Sumber harga, ambang,
  frekuensi pemantauan dan perlakuan data basi belum diputuskan.
- Jual semua adalah target exit, bukan jaminan fill penuh atau harga tertentu.
  Gap, slippage, partial fill, minimum order dan kegagalan jual harus ditangani.
- Usulan fail-safe: timeout AI tidak mematikan stop; tesis kedaluwarsa tidak
  diperpanjang otomatis. Kebijakan review terlambat/data basi harus dibekukan
  sebelum aktivasi, termasuk kapan menutup posisi.

## Keputusan yang masih terbuka

- Definisi katalis yang layak dan syarat khusus kandidat memecoin.
- Definisi terukur fundamental/tesis yang cukup untuk hold setelah acara.
- Rumus jarak stop/trailing, batas entry dan validasi harga pemicu.
- Jadwal review, kedaluwarsa tesis dan perlakuan kesempatan yang stagnan.
- Modal paper, sizing, batas posisi/exposure dan sleeve memecoin.
- Model fill, biaya, kondisi pasar tidak tersedia dan recovery.
- Pembagian collector/AI/engine serta penggunaan ulang komponen repo.

## Cakupan masa depan

**Presale, TGE dan token yang baru mulai diperdagangkan** merupakan eksperimen
terpisah yang ditunda. Alurnya kelak perlu membedakan pembelian sale, alokasi,
vesting, claim dan awal perdagangan; memeriksa syarat partisipasi, kontrak,
likuiditas serta rute jual. TGE tidak otomatis berarti aset dapat dijual.
Eksekusi DEX, risiko kontrak/transfer, gas dan MEV memerlukan adapter dan model
risiko tersendiri. Daftar website lampiran dapat menjadi referensi discovery.

## Arsitektur dan penggunaan ulang

Alur tanggung jawab: collector → arsip bukti → paket event terverifikasi →
analisis AI terstruktur → validator deterministik → engine paper → ledger →
laporan. Collector tidak memiliki hak order; model tidak diberi shell,
kredensial trading atau hak mengubah deployment/risk. Teks halaman dan keluaran
model adalah input tidak tepercaya, bukan instruksi sistem.

| Komponen yang ada | Penggunaan dalam Plan 3 | Batas saat ini |
|---|---|---|
| `internal/marketdata`, adapter Binance | Instrumen, candle, feed dan backfill | Book/depth historis dan fill berbasis antrean belum dibuktikan |
| `internal/managed` | Ownership, reservasi, jurnal, batas akun dan reconciliation | Jangan membuat ledger finansial kedua dari keputusan AI |
| `internal/engine/checkpoint.go` | Pemulihan entry/proteksi/trailing dan exit | State event/tesis dan kontrak strategi baru masih perlu dirancang |
| `internal/risk` | Sizing desimal dan pembatas exposure | Biaya/price impact untuk pilot harus dikalibrasi; jangan menganggap buffer lama cukup |
| `internal/broker/sim.go` | Baseline simulasi yang sudah tersedia | Fill berbasis candle, bukan simulator order book; tidak cukup membuktikan eksekusi pasar tipis |
| Web, metrics, recorder, Telegram | Kontrol, observasi dan notifikasi | Tampilkan alasan keputusan, bukti, freshness dan proteksi; satu poller per token |

Usulan implementasi minimum: collector/analisis terpisah dari jalur proteksi,
dengan kontrak keputusan sempit menuju engine. Pilihan proses sidecar atau
modul Go dibekukan pada P3-0 setelah memeriksa titik integrasi. Lampiran Hermes
bukan mandat membuat engine SQLite baru. Pilot AI memakai DB/wallet/modal
paper terpisah dari eksperimen lama; library yang ada boleh digunakan ulang.
Jangan menambah bahasa/service/vector database bila kebutuhan belum terbukti.

Spider adalah engine collector utama sesuai klarifikasi user 26 Oktober.
Gunakan halaman/API/RSS resmi sebagai bahan bila tersedia melalui jalur koleksi
yang terukur; jangan mengganti Spider diam-diam dengan crawler Go. Batasi domain,
redirect, ukuran respons, kedalaman, concurrency dan waktu.
Tolak akses loopback/private network melalui URL hasil crawling untuk mencegah
SSRF. Hormati kebijakan sumber dan robots; kegagalan akses dicatat sebagai gap.
HTTP-first dan browser hanya bila perlu; ukur beban terpisah dari bot lama.
Referensi: [Spider](https://github.com/spider-rs/spider),
[dokumentasi](https://docs.rs/spider/latest/spider/).

## Kontrak data dan keputusan

Ini kontrak konseptual, bukan keharusan satu tabel per entitas.

| Catatan | Isi minimum |
|---|---|
| Bukti | URL asal/final, domain, waktu terima, waktu publikasi bila tersedia, hash, konten yang dapat diarsipkan, status fetch dan revisi |
| Event | ID stabil, aset/venue, tipe katalis, waktu/zona waktu/kepastian, tautan bukti, konflik, status aktif/ditunda/dicabut |
| Tesis | Versi, mekanisme dampak, kontra-argumen, kondisi batal, alasan hold setelah acara, waktu review/kedaluwarsa |
| Keputusan AI | ID unik, versi model/prompt/input, versi event/tesis/policy, waktu, aksi, alasan dan referensi bukti, masa berlaku |
| Rencana posisi | Scope wallet/instance/binding, batas harga, stop/trailing, sizing, biaya dan batas risiko yang dihitung validator |
| Eksekusi/evaluasi | Order, fill, fee, reservasi, peak/stop, alasan exit, latency, biaya inference dan keputusan yang ditolak |

Aksi model dibatasi `REJECT`, `WATCH`, `PROPOSE_ENTRY`, `HOLD`, `PROPOSE_EXIT`.
Model tidak menulis saldo/kuantitas final atau memperpanjang tesis tanpa batas.
Schema menolak field asing, nilai nonfinite, bukti yang tidak ada dan scope
yang salah. Bukti kutipan harus cocok dengan paket input; confidence model
bukan probabilitas keuntungan terkalibrasi.

Validator menolak keputusan basi, versi event berubah, event dicabut, harga
melampaui batas atau instrumen tidak layak. Validasi ulang sesaat sebelum
reservasi/kirim order. Decision ID dan pembaruan posisi diproses idempoten;
dua worker atau retry tidak boleh menghasilkan dua entry. Revisi yang datang
setelah fill memicu evaluasi posisi, bukan membatalkan fakta fill yang terjadi.

## State posisi dan urutan keselamatan

Urutan konseptual: discovered → verified/watchlist → eligible → entry pending
→ holding → closing → closed. Ditolak/kedaluwarsa/dicabut tetap tersimpan.
`needs_attention` dapat menahan kemajuan saat status dana/order belum pasti.

Sebelum entry, proteksi terencana wajib tersimpan. Setelah fill parsial,
proteksi mengikuti kuantitas yang benar-benar dimiliki. Exit prioritas:
proteksi harga/risk → pencabutan tesis tervalidasi → review hold. Hold dari AI
tidak boleh membatalkan exit yang sudah dipicu. Entry baru ditahan saat ada
status order/ownership ambigu; proteksi posisi yang aman tetap berjalan.

Full exit memerlukan cancel/resync order yang mengunci aset, lalu menjual sisa
kuantitas milik binding dengan mempertimbangkan fee base dan lot/minimum order.
Partial fill diproses sekali; sisa gagal jual tetap `closing`/attention dan
di-retry menurut kebijakan terbatas. Dust tidak dihapus untuk membuat flat.
Tidak ada rotasi dana yang masih reserved atau penjualan aset eksperimen lain.

## Rancangan trailing dan hold lanjutan

Usulan awal untuk diuji: bekukan jarak trailing persentase `d` dari volatilitas
candle tertutup sebelum entry; simpan puncak harga valid `H` sesudah entry.
Untuk long, kandidat stop `H × (1 − d)` dan stop efektif adalah nilai tertinggi
antara stop sebelumnya, stop awal dan kandidat tersebut. Ini konsisten dengan
trailing persentase yang sudah dikenal engine; bukan trailing ATR dinamis yang
diam-diam diperlebar ketika volatilitas naik.

Pemicu memakai harga venue dan jenis quote/trade yang dibekukan dalam policy.
Cadence, validasi outlier, stale threshold, batas `d` dan pembulatan tick wajib
ditentukan sebelum pilot. Tidak menunggu candle tutup atau jawaban AI untuk
menjalankan proteksi yang sudah terpenuhi. Native order dan paper trigger perlu
dibedakan; [mekanisme trailing Binance](https://developers.binance.com/en/docs/products/spot/faqs/trailing-stop-faq)
tidak membuktikan adapter repo sudah mendukung setiap jenis order tersebut.

Hold sesudah acara memerlukan evaluasi bukti aktual: apa hasil acaranya, apakah
mekanisme manfaat token terjadi, apa risiko baru dan kapan review berikutnya.
Untuk memecoin, gunakan bukti katalis/narasi dan kualitas pasar yang relevan,
bukan mengarang fundamental produk. AI tidak boleh mengganti alasan awal dengan
cerita baru hanya untuk menghindari realisasi rugi. Perubahan tesis diberi versi
dan tunduk batas policy; stop harga tetap tidak turun.

Tidak ada maksimum hold 48 jam atau exit sebelum acara yang otomatis berlaku.
Namun setiap tesis memiliki tenggat review dan kedaluwarsa terbatas. P3-0 wajib
menentukan jendela review, grace period saat layanan gagal dan aturan keluar
saat bukti tidak dapat diperbarui. Hold tanpa evaluasi tidak menjadi default.

## Kebijakan yang harus dibekukan sebelum pilot

| Kelompok | Keputusan wajib | Status |
|---|---|---|
| Universe | Venue/pair, jenis event, sumber, pengecualian, syarat memecoin | Development ETH/SOL/DOGE; validasi pilot masih terbuka |
| Discovery | Horizon, frekuensi polling, revisi/dedup, kuota sumber | Kontrak dev 14 hari/900 detik; adapter event belum selesai |
| Entry | Jendela entry, deviasi maksimum, spread, depth dan batas price impact | Gate dev tersedia; kalibrasi book/price impact nyata terbuka |
| Proteksi | Rumus/TF volatilitas, stop/trailing minimum-maksimum, sumber pemicu | Kontrak dev 2 ATR, 1–15%; ingestion ATR/quote nyata terbuka |
| Hold | Interval review, masa berlaku tesis, event tertunda, stagnasi dan re-entry | Dev review 6 jam, TTL 300 detik, expiry tanpa grace; registry nyata terbuka |
| Risiko | Modal virtual, risiko per trade, nominal/posisi/exposure dan konsentrasi | Kontrak dev 1.000 USDT, risk 0,5%, cap 20%/40%, meme 2%/4% |
| Halt | Rugi harian/drawdown, zona waktu, pemulihan/reset oleh operator | Dev 2%/8%, hari UTC, sticky; tidak mengubah P2 |
| Kegagalan | Quote basi, feed putus, timeout AI, DB outage, gagal exit | Fail-closed dasar diuji; outage proses/DB pilot belum diuji |
| Biaya | Fee akun/simbol, slippage, book/depth dan biaya model/data | Dev fee 0,1%/sisi, slip 2 bps; tarif nyata belum dikalibrasi |
| Operasi | Batas CPU/RAM/disk/API, alert dan backup/restore | Harus diukur; headroom historis bukan kapasitas pilot |

Kontrak development rinci ada di [runtime katalis](catalyst-runtime.md).
Nilai development belum menjadi persetujuan aktivasi atau konfigurasi final
pilot; perubahan mendapat versi/hash baru. Stop bukan jaminan batas rugi saat
gap; sizing harus memperhitungkan biaya dan kerugian total yang mungkin pada
aset kehilangan likuiditas. Mainnet tetap dilarang sampai keputusan P3-LIVE.

## Migrasi pekerjaan Plan 2

Seluruh **14 checkbox terbuka** P2-6/P2-7/P2-9/P2-10 dipetakan di bawah,
ditambah dependensi D-01–D-03, tindak lanjut audit dan batas laporan. Checkbox
arsip Plan 2 tidak dicentang agar tidak mengklaim pekerjaan sudah dilaksanakan.
ID C lama dipertahankan sebagai provenance, bukan antrean duplikat.

### P3-R — Riset dan ekspansi pembanding lama

- [ ] **P3-R1 (P2-6 komisi, C-04/C-13):** periksa tarif maker/taker aktual
  akun/simbol saat relevan, diskon BNB opsional dan stress fee/slippage. Fixture
  base/quote/BNB tetap bukan bukti fee riil; order uang asli menunggu P3-LIVE.
- [ ] **P3-R2 (P2-6 ekspansi 22 Oktober):** teruskan target 44 instance/132
  binding dalam [matriks](strategy-2.md#perluasan-daftar-dan-izin-cpu-terbaru).
  Checkpoint terdokumentasi 6/18 aktif; sisa 38/114. Dari target, 28/84 memakai
  keluarga tersedia dan 16/48 menunggu implementasi. Bukan kuota wajib aktif.
- [ ] **P3-R3 (P2-6 universe, C-08):** verifikasi top-20/Spot dan kelayakan
  histori, bekukan tujuh keluarga strategi, empat timeframe, empat modal,
  trade-off frekuensi/return/drawdown dan komposisi satu modal bersama.
- [ ] **P3-R4 (P2-6 implementasi):** spesifikasi/implementasi rotasi, squeeze
  dan mean reversion; antrean VPS, resource guard dan anggaran API/reconnect.
  Jangan menamai strategi pengganti sebagai strategi yang belum tersedia.
- [ ] **P3-R5 (P2-6 batch):** kalibrasi throughput/ukuran data, evaluasi
  individual dan portofolio bersama serta simpan semua kegagalan. Plafon lama
  6.528 job serial tetap batas rancangan, bukan pekerjaan sudah berjalan atau
  eksperimen independen; data yang pernah dilihat bukan holdout baru.
- [ ] **P3-R6 (P2-6 trial bertahap):** lanjutkan hanya dari tahap yang sudah
  terdokumentasi, tanpa mengulang start 3/9 atau reset T0. Skala setelah minimal
  24 jam observasi tahap berjalan dan gerbang kapasitas/operasi sesuai
  [protokol](strategy-2.md#perluasan-riset-multi-instance--21-oktober-2026).
  DB/wallet baru untuk eksperimen baru; target 44/132 menggantikan plafon lama
  24/72, tetapi tahapan antara tetap mengikuti hasil ukur dan cakupan izin.

Riset lama tetap menjadi pembanding terpisah; bukan prasyarat menyelesaikan
semua 44 instance sebelum mengembangkan collector AI.

### P3-O — Penerimaan operasi dan tindak lanjut audit

- [ ] **P3-O1 (P2-7 Telegram, C-06):** bukti penerimaan nyata scoped,
  whitelist chat lain, digest 07.00 WIB, shutdown dan pesan fill alami.
  Rilis 23 Oktober sudah memasang perbaikan notifikasi; penerimaan aplikasi,
  fill berikutnya dan digest 24 Oktober masih terbuka pada checkpoint
  [rilis](testing/telegram-notification-release.json). Tinjau poll/send failure;
  demo/API sukses bukan bukti penerimaan semua alur.
- [ ] **P3-O2 (P2-9 verifikasi, C-07/C-14/C-16/C-19):** restart dengan
  posisi/order/reservasi aktif, proteksi, feed/reconnect/gap backfill, recorder
  retry/flush saat storage outage; migrasi/restore DB salinan, backup terjadwal,
  memori Grafana dan disk. Restart tanpa posisi serta restore pra-rilis sudah
  terdokumentasi; jangan diulang sebagai syarat baru tanpa perubahan/risiko.
  Restore backup pascarilis 23 Oktober belum diverifikasi. Verifikasi namespace
  ID/mode dan market-data publik pada rilis yang relevan tanpa order mainnet.
- [ ] **P3-O3 (P2-9 tujuh hari, C-08/C-18):** review operasi prospektif
  trial tahap dua pada 23/25/29 Oktober 19:58:45 WIB (hari 1/3/7 dari T0
  22 Oktober). Jadwal ETH dan tahap satu dibatalkan, bukan kelulusan yang
  dipindahkan. Nilai strategi dengan sampel memadai; 30 hari dapat menjadi
  evaluasi awal, diperpanjang bila transaksi sedikit. Tidak ada reminder AI baru.
- [ ] **P3-O4 (P2-0/P2-8 lanjutan, C-01/C-02/C-03/C-05):** tutup atau
  klasifikasikan ketidakpastian tick penolakan OCO, cancel ambigu dan proteksi
  expired; batas rekonstruksi realized PnL/equity/dust; audit terminal saat
  finish. Pertahankan bukti lama dan regression yang sudah ada. Grid/MM
  partial fill, order balasan per level dan reservasi base diuji ulang saat
  jalur terkait berubah, bukan menganggap audit lama belum pernah dilakukan.
- [ ] **P3-O5 (risiko review P2-8):** rekonsiliasi status lima temuan
  [review kode](testing/plan-2-code-review.md#risiko-terbuka-belum-diubah).
  Resolusi order tak pasti dan kompaksi state telah mendapat implementasi
  lanjutan (`/resolve`, `managed_archive`); ukur kecukupan/pertumbuhannya,
  jangan membangun ulang. Periksa attention transien snapshot/fill, pemulihan
  paper order `canceling` dan konsistensi idempotency no-op PG/Memory sebelum
  menyebut tiga temuan lain selesai.
- [ ] **P3-O6 (temuan VPS lanjutan):** telusuri label run `interrupted`
  saat shutdown terkontrol dan log `store candle/context canceled`; klasifikasi
  WARN persist. Akar restart Docker bersama sudah dikaitkan ke penghentian
  service, pemicu administratif tetap belum diketahui. Kurangi akses DB/no-op
  jika pengukuran skala membutuhkannya; sampel pascarilis singkat bukan bukti
  kapasitas panjang. Jangan restart Docker atau mengedit history untuk merapikan.

C-15 dipenuhi sebagai inventaris/reuse test historis, bukan kewajiban menjalankan
ulang semua tes. C-17 (pengecualian saham tokenisasi, pesan stop terdahulu,
backup terdahulu) tetap selesai. C-18 target 72 jam tetap dibatalkan. C-16 punya
checkpoint proteksi pada kode/laporan lokal; penerimaan recovery VPS mengikuti
P3-O2. Perangkat fisik/Safari dan migrasi inventori legacy nyata belum menjadi
bukti selesai; lakukan bila target perangkat atau kebutuhan migrasi memerlukannya.

### P3-LIVE — Keputusan uang asli yang tetap bersyarat

Meneruskan seluruh P2-10/C-09 dan bagian live C-04/C-14; **belum disetujui**.

- [ ] Tinjau bukti strategi, operasi dan temuan terbuka bersama user.
- [ ] Tetapkan modal riil, loss/drawdown, leverage dan scope akun/strategi,
  lalu dapatkan persetujuan eksplisit sebelum transaksi uang asli.
- [ ] Hanya bila disetujui, uji kecil fee riil dan ledger vs exchange; siapkan
  penghentian aman, tanpa kenaikan modal otomatis.
- [ ] Jadwalkan backup luar server pada tahap live sesuai keputusan user;
  backup satu server tetap memiliki risiko kehilangan bersama VPS.
- [ ] Tinjau akses/legal/pajak terkini saat deployment riil relevan; catatan
  lama bukan verifikasi aturan yang sedang berlaku.

### P3-D — Pekerjaan bersyarat dan future reference

- [ ] **P3-D1 (D-01/C-10):** Binance Futures menunggu Demo key dan keputusan
  memakai futures. Order/algo SL/TP, listenKey/reconnect/expiry, event/partial
  fill, leverage/margin/one-way, reconciliation, fixture dan ownership net
  position memerlukan pengujian/desain sendiri. Subledger Spot tidak cukup.
- [ ] **P3-D2 (D-02/C-11):** Bybit private API dan xspread menunggu akses.
  Signing/WS, shared contract test, spot/linear, unified wallet, conditional
  stop/filter/base fee, partial/cancel, namespace ID dan efek margin account-wide
  tetap dibawa. Arbitrase nyata perlu kedua venue teruji; tetap nonaktif.
- [ ] **P3-D3 (D-03/C-12/C-13):** histori funding, backtest dua kaki
  tersinkron, hedge exposure, leg timpang/cooldown/repair dan multi-akun.
  Tambahkan L2/queue/latency MM serta aturan instrumen historis sesuai kebutuhan
  data; jangan mengklaim simulator candle sudah mencakupnya.
- [ ] **P3-D4:** presale/TGE/token baru dan DEX sesuai cakupan masa depan
  di atas. Bukan bagian pilot dan tidak diaktifkan oleh daftar website.

OKX tetap dihapus; saham tokenisasi tetap dikecualikan, bukan backlog untuk
diaktifkan kembali. Penundaan exchange lain tidak menahan pilot Spot.

## Fase implementasi baru dan kriteria penerimaan

Urutan: kontrak → collector → analisis → entry/hold/exit → laporan → pengujian
dan fixing → trial prospektif. Perubahan dana/proteksi tetap memerlukan
regression kecil ketika diimplementasikan. User sudah meminta pelaksanaan
development; pengujian komponen/repo dijalankan pada checkpoint implementasi.
Tidak ada izin menjalankan trial laptop, deploy atau aktivasi pilot dari daftar ini.

### P3-0 — Kontrak pilot dan desain integrasi

- [ ] Bekukan seluruh kebijakan pada tabel di atas, versi eksperimen,
  benchmark dan kriteria evaluasi; identifikasi keputusan user yang dibutuhkan.
- [x] Petakan titik integrasi dan caller/test/config/migrasi yang terdampak;
  pilih reuse engine, batas proses dan kontrak collector/model/validator.
- [ ] Tetapkan modal virtual terpisah, isolasi DB/wallet dan resource budget
  tanpa mengubah parameter/modal eksperimen pembanding.

Selesai bila ada spesifikasi implementasi tanpa parameter keselamatan yang
masih ambigu, beserta batas pengujian dan aktivasi yang jelas.

### P3-1 — Collector dan arsip prospektif

- [x] Bangun collector untuk sedikit URL terkonfigurasi dengan daftar primary,
  interval/cache/revisi/backoff dan arsip immutable; diuji transport fixture.
- [ ] Selesaikan integrasi Spider sebagai engine utama, uji kontrak binary/proxy
  dan ekstraksi artikel; wiring CLI/image/binary patched lulus fixture lokal
  28 Oktober. Egress adversarial/situs publik masih terbuka; belum penerimaan penuh.
- [ ] Verifikasi kontrak sumber nyata dan kebutuhan rendering per situs.
- [ ] Simpan bukti/revisi/dedup dan identitas event/asset; TBA tetap TBA,
  konflik material menahan entry. Catat sumber gagal dan cakupan yang hilang.
- [ ] Terapkan batas URL/redirect/ukuran/waktu, sanitasi dan redaksi secret,
  retensi arsip serta resource/cost metrics.

Selesai bila event yang sama tidak berlipat, perubahan/pencabutan dapat
ditelusuri dan waktu informasi tersedia dapat direkonstruksi tanpa model.

### P3-2 — Analisis tesis dan shadow evaluation

- [x] Bentuk paket bukti terbatas dan adapter model tanpa tools dengan batas
  token/timeout serta dedup berversi; fixture dan inference nyata dengan bukti sintetis diuji.
- [x] Validasi schema, evidence, scope dan freshness; simpan input/output,
  versi serta penolakan. JSON rusak/hallucination/timeout berarti tidak entry.
- [x] Pilih model utama dari evaluasi gateway nyata; hasil dan kandidat fallback
  dicatat di laporan model. Ini belum evaluasi prospektif atau uji combo.
- [ ] Rekam keputusan shadow, termasuk HOLD sah, bersama pembanding event
  sederhana tanpa AI dengan akses informasi dan waktu yang sama.

Selesai bila keputusan dapat diaudit, biaya terukur dan model tidak bisa
mengubah policy atau mengirim order sendiri. Shadow belum membuka posisi.

### P3-3 — Entry paper dan ledger

- [x] Hubungkan keputusan fixture tervalidasi ke koordinator managed/Sim;
  reservasi/dedup/checkpoint memakai komponen lama tanpa saldo baru per binding.
- [x] Validasi snapshot quote/spread/depth dan sizing setelah fee/slippage, aturan
  instrumen serta batas memecoin/konsentrasi. Entry di luar batas ditolak.
- [ ] Hubungkan registry attestation/event dan adapter pasar/ATR nyata;
  jangan memakai flag fixture atau output model sebagai bukti verifikasi.
- [ ] Tetapkan model fill yang memadai: jika masih memakai candle Sim,
  labeli batasnya dan batasi universe; dukungan book/depth harus benar-benar
  tersedia sebelum mengklaim simulasi realistis untuk pasar tipis.

Selesai bila retry/concurrency tidak menggandakan entry, posisi dapat
ditelusuri ke keputusan/bukti dan dana eksperimen lama tidak tersentuh.

### P3-4 — Proteksi, hold, full exit dan recovery

- [x] Persist puncak, stop, thesis version dan review deadline; trailing
  monoton dan bekerja tanpa menunggu AI. Jangan mengubah proteksi posisi lama
  karena parameter default berubah.
- [ ] Review sebelum/sesudah acara dan material update; terapkan expiry,
  stale-data serta outage policy. HOLD tidak mengalahkan exit yang sudah dipicu.
- [ ] Full exit milik binding, cancel/resync, partial fee/fill/dust, retry
  terbatas dan closing/attention; rotasi hanya dari dana yang telah bebas.
- [ ] Pulihkan state setelah restart/crash, keputusan bersamaan, fill terlambat
  dan koneksi putus tanpa stop turun, order ganda atau inventori hilang.

Selesai bila skenario proteksi/tesis/exit/recovery deterministik dan posisi
tidak dinyatakan closed sebelum status aset/order benar-benar selesai.

### P3-5 — Kontrol, laporan dan observasi

- [x] CLI report sumber/keputusan/model/token dan replay dengan label fixture;
  dollar cost belum diketahui ditampilkan null, bukan nol.
- [ ] Reuse dashboard/Telegram: tampilkan event, alasan entry/HOLD/exit,
  URL bukti, waktu review, stop/peak, posisi/dana, biaya serta kesehatan layanan.
- [ ] Pause entry terpisah dari flatten; otorisasi scope dan satu poller
  Telegram. Model tidak dapat reset halt atau menaikkan modal/batas risiko.
- [ ] Metrics discovery→decision→fill, umur quote/tesis, gap sumber, model
  error/cost, proteksi, backlog/retry serta resource collector/DB/engine.

Selesai bila operator mengetahui mengapa bot menunggu atau menutup posisi,
dan proses hidup dibedakan dari feed/proteksi yang benar-benar sehat.

### P3-6 — Pengujian dan fixing

- [ ] Unit/fixture: dedup/revisi/konflik, asset mapping, schema, prompt
  injection/SSRF, keputusan basi dan versi policy/event berubah.
- [ ] Dana/proteksi: double submission, partial fill/base fee, cancel ambigu,
  gap/outlier/stale quote, stop monoton, stop bersamaan pencabutan tesis,
  gagal jual/dust dan larangan menjual aset binding lain.
- [ ] Recovery: crash setelah reservasi/fill sebelum checkpoint, expiry saat
  model mati, DB/feed outage, replay dan shutdown; DB sementara terisolasi.
- [ ] End-to-end collector→model nyata→validator→paper→ledger→notifikasi;
  HOLD sah tidak memaksa transaksi, fixture terkontrol menguji jalur fill/exit.
- [ ] Review auth/control/UI, CI wajib dan perbaiki temuan material dengan
  regression. Simpan bukti dan batasnya di `docs/testing/`, tanpa menimpa JSON lama.

Selesai bila seluruh jalur kritis punya bukti, temuan penghalang aktivasi ditutup,
dan klaim integrasi nyata dibedakan dari fixture. Verifikasi dijalankan saat
tugas implementasi/pengujian mengizinkannya, bukan otomatis saat membaca plan.

### P3-7 — Trial paper prospektif dan evaluasi

- [ ] Tinjau hasil P3-0–P3-6, isolasi dan kapasitas dengan beban collector/model;
  sajikan rencana deployment, backup/rollback dan jadwal sebelum meminta izin start.
- [ ] Sesudah izin yang sesuai, rilis lewat GitHub/CI/GHCR/digest, DB/wallet
  baru, policy hash, capital dan T0 aktual. Jangan menumpang ledger tahap dua.
- [ ] Review operasi hari 1/3/7 dari T0 pilot sendiri; interval ini tidak
  menjadwalkan agent otomatis atau mengganti review pembanding lama.
- [ ] Evaluasi prospective bersih setelah biaya trading/model/data, drawdown,
  exposure, turnover, rejection/no-fill, latency serta reliabilitas proteksi.
  Bandingkan tunai, buy-and-hold, aturan event tanpa AI dan strategi lama
  dengan normalisasi modal/risiko/exposure serta jendela waktu yang jelas.
- [ ] Tentukan lanjut/perbaiki/hentikan atau belum cukup bukti berdasarkan
  kriteria P3-0; jangan memilih hanya event pemenang. Tidak ada auto-promosi live.

Berita historis yang direvisi dan pengetahuan model tentang peristiwa lama
dapat membocorkan masa depan. Backtest berita lama hanya eksplorasi kecuali
tersedia snapshot point-in-time yang memadai; forward/shadow archive menjadi
bukti utama. Target return 50% tetap aspirasi historis, bukan janji Plan 3.

## Panduan melanjutkan dan log

1. Baca handover agent/VPS, cek working tree dan pertahankan perubahan user.
2. Prioritaskan P3-0 lalu fase AI berikutnya tanpa menunggu seluruh backlog P2.
   Gunakan P3-R/P3-O saat tugas terkait pembanding atau penerimaan platform;
   pertahankan eksperimen P2 untuk riset berikutnya. Instruksi arsip tidak
   mengaktifkan ulang trial yang telah dihentikan.
3. Catat progres pada ID di sini, bukti dan keterbatasannya; sinkronkan dokumen
   operasional jika kontrak atau deployment benar-benar berubah.
4. Commit/push/deploy dan uang asli memerlukan cakupan izin masing-masing.
   VPS Plesk tetap dibatasi proyek bot; trial di laptop tetap dilarang.

| Tanggal | Keputusan/perubahan | Status |
|---|---|---|
| 23 Oktober 2026 | Katalis, hold sesuai tesis, memecoin dan full exit; presale/token baru ditunda | Arah user; angka policy belum dibekukan |
| 23 Oktober 2026 | Plan 2 ditutup, backlog dipindah dan Plan 3 dirinci sebagai rencana aktif | Dokumentasi; tidak mengubah kode, VPS, trial atau izin live |
| 23 Oktober 2026 | User menegaskan P2 berjalan dan seluruh aset riset dipertahankan; prioritaskan P3-0 dan fase AI tanpa menunggu backlog P2 | Pengembangan terpisah; aktivasi tetap mengikuti gerbang isolasi/kapasitas |
| 23 Oktober 2026 | Commit baseline docs `bc321b8`; mulai development komponen/CLI terpisah dan replay managed/Sim; urusan AI nyata ditunda user | Bukti lokal di testing/plan-3-local.md; P2 tidak diubah, pilot belum aktif |
| 26 Oktober 2026 | User memilih OmniRoute; env terpisah dan pembacaan env CLI disiapkan | HTTPS API 401 tanpa key; model/key kosong, belum inference/deploy |
| 26 Oktober 2026 | User mengisi key dan mengizinkan evaluasi model; Luna menjadi utama, Haiku kandidat fallback | Tujuh kasus sintetis, strict schema v3; combo/pilot belum aktif, P2 tetap |
| 26 Oktober 2026 | User meluruskan Spider sebagai collector utama dan mengizinkan implementasi/inspeksi VPS, lalu meminta handover | Draft belum tuntas; inspeksi read-only; tidak install/deploy/start P3 atau mengubah P2 |
| 28 Oktober 2026 | Integrasi lokal Spider: CLI, lima halaman, cache/arsip/proxy, image terpisah dan patch stdout p3.1; CI disiapkan | Race/vet/lint, build/smoke image, Compose config dan tiga putaran binary TLS fixture lulus; egress adversarial/situs publik terbuka; tanpa commit/push/deploy atau perubahan runtime P2 |
