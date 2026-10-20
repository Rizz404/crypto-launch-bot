# Plan 2 — Multi-instance dan pengembangan platform trading

Tanggal: **20 Oktober 2026**. Status: **aktif; implementasi dimulai dari P2-0,
audit awal dan dua perbaikan reconciliation lokal selesai; P2-1 berikutnya**. [Plan 1](plan.md) ditutup sebagai siklus
pengembangan pertama atas permintaan user. Pekerjaan yang belum selesai dibawa
ke sini; penutupan Plan 1 **tidak berarti semua pengujian lulus atau live siap**.

## Tujuan dan keputusan

Satu akun exchange dapat memiliki beberapa **instance bot**, setiap instance
memiliki modal sendiri dan menjalankan beberapa strategi. Modal dibagi dari
satu sumber, bukan digandakan untuk setiap proses/strategi. Laba, rugi, fee,
order, inventori dan batas risiko dapat ditelusuri pada ketiga tingkat.

User telah menyetujui arah fitur dan meminta dokumentasi disusun dahulu,
kemudian implementasi mengikuti fase di bawah. Tidak perlu meminta persetujuan
ulang untuk pekerjaan kode lokal dalam cakupan ini; deployment baru dan live
mengikuti batas otorisasi serta prasyarat operasional yang berlaku.

Urutan utama: pencatatan modal dan posisi → reservasi/reconciliation → risiko
serta kontrol → dashboard → evaluasi strategi → testing menyeluruh → fixing →
uji VPS. Build fitur didahulukan, dengan fase testing dan fixing khusus di akhir
sesuai pola kerja user. Cek build/format dan regression test untuk perbaikan
keamanan dana tetap dilakukan saat scope terkait berubah.

Baca [rancangan multi-instance](multi-instance.md) sebelum mengubah schema,
portfolio, engine atau sesi. Pilihan strategi tetap
[ETH momentum 4h, BTC harian sebagai pembanding](strategy-decision-2026-10-20.md).
Tidak ada trial baru dijalankan sebagai bagian penyusunan dokumen ini.

## Batas cakupan dan kondisi awal

- Prioritas implementasi: **Binance Spot**, satu wallet USDT per akun/mode.
  Model data tetap membedakan akun fisik, mode dan market.
- Instance adalah portofolio virtual di aplikasi, **bukan container wajib**.
  Satu pengelola akun mengoordinasikan saldo, broker, event dan rate limit.
  Dua worker tidak boleh menjadi pemilik tulis wallet yang sama sekaligus.
- Kode kini masih satu sesi engine, ledger per exchange/market, posisi per
  simbol. Beberapa strategi bisa dikonfigurasi, tetapi modal/posisi antar-instance
  belum terisolasi. Fitur Plan 2 tidak boleh diklaim sudah tersedia.
- VPS terakhir diverifikasi: bot testnet stop normal 20 Oktober 18.52 WIB,
  tujuh order dibatalkan, inventori dipertahankan, timer observasi inactive.
  DB, DNS, monitoring dan backup tetap aktif. Tidak ada perintah ke VPS pada
  pekerjaan dokumentasi ini. [Handoff operasional](handoff-vps.md).
- Paper ETH di laptop tetap dilarang; marker penghentian dan data dipertahankan.
- Mainnet belum disetujui. Penutupan Plan 1 tidak menghapus kebutuhan keputusan
  strategi/risk dan persetujuan live eksplisit.
- OKX tetap dihapus. Bybit/xspread ditunda; futures menunggu kredensial Demo.
  Saham ter-tokenisasi sudah dikecualikan; keputusan ini tidak dibuka ulang.
- Commit satu baris per scope/dependensi; secret tidak masuk Git/log/memori.
  Rilis lewat GitHub → CI/GHCR → image digest, bukan salin source laptop.

## Daftar fitur yang disepakati

| ID | Fitur | Hasil yang harus tersedia | Tahap |
|---|---|---|---|
| MI-01 | Banyak instance | Buat, ubah nama, arsipkan instance dengan identitas stabil | P2-1/4 |
| MI-02 | Modal per instance | Alokasi dari cadangan akun; tidak ada saldo ganda | P2-2 |
| MI-03 | Banyak strategi | Beberapa strategi/pair per instance | P2-3/4 |
| MI-04 | Anggaran strategi | Pool bersama secara default, batas per strategi opsional | P2-2/3 |
| MI-05 | Reservasi dana | Order pending mengunci quote/base hingga status pasti | P2-3 |
| MI-06 | Kepemilikan posisi | Dua strategi pada koin sama tetap terpisah | P2-1/3 |
| MI-07 | Laporan bertingkat | Akun/instance/strategi: saldo, PnL, fee, posisi, drawdown | P2-5 |
| MI-08 | Transfer modal | Transfer internal saldo bebas dengan jurnal, tanpa menjual paksa | P2-2/4 |
| MI-09 | Kontrol bertingkat | Pause/resume/cancel/close pada scope yang dipilih | P2-4 |
| MI-10 | Pemulihan | Restart mempertahankan alokasi, reservasi, order dan posisi | P2-3/4 |
| UX-01 | Pilihan stop | Pause entry, cancel pending, flatten dan stop proses dibedakan | P2-4 |
| UX-02 | Risiko bertingkat | Limit strategi → instance → akun; batas akun selalu berlaku | P2-4 |
| UX-03 | Pemeriksaan saldo | Selisih exchange dicatat, aktivitas terdampak ditahan | P2-0/3 |
| UX-04 | Dashboard instance | Ringkasan dan detail instance/strategi beserta status kesehatan | P2-5 |
| UX-05 | Alasan bot menunggu | Sinyal, warmup, dana, minimum order, risiko, koneksi | P2-4/5 |
| UX-06 | Perbandingan setelah biaya | Bandingkan strategi, tunai dan buy-and-hold | P2-6 |
| UX-07 | Versi konfigurasi | Audit parameter, alokasi, kontrol dan versi trial | P2-1/4 |
| UX-08 | Backtest → paper → live | Definisi strategi bisa dipakai ulang, ledger tetap terpisah | P2-6/9/10 |
| UX-09 | Notifikasi pilihan | Scope, tingkat pesan, ringkasan dan penggabungan pesan berulang | P2-5 |
| UX-10 | Template instance | Duplikasi konfigurasi tanpa saldo/posisi/riwayat lama | P2-5 |

Seluruh fitur tabel masih **belum diimplementasikan sebagai multi-instance**.
Kontrol, laporan dan risk Plan 1 yang sudah ada menjadi fondasi, bukan bukti
bahwa scope instance telah tersedia.

## Pemindahan pekerjaan dari Plan 1

Sumber: [Sisa Fase 9](plan.md#sisa-fase-9),
[penutupan Fase 9](plan.md#penutupan-fase-9-2026-10-18),
[backlog testing lama](plan.md#backlog-testing-diisi-selama-fase-28-dikerjakan-di-fase-9),
[pertanyaan lama](plan.md#pertanyaan-terbuka-belum-diputuskan),
serta [laporan penutupan testnet](testing/testnet-stop-2026-10-20/analysis.md).

| ID | Pekerjaan/status akhir yang dibawa | Tujuan |
|---|---|---|
| C-01 | Dua kegagalan proteksi ETH; cancel OCO 23 kali gagal, linked BTC expired; penyebab/race belum dipastikan | P2-0/7/8 |
| C-02 | Selisih kuantitas lima DOGE, realized PnL vs balance, dust dan mark equity testnet perlu audit | P2-0/2/3/7 |
| C-03 | Stop terminal: tujuh cancel exchange sesudah recorder stop masih `new` di DB; perlu penyelesaian tercatat | P2-0/4 |
| C-04 | Fee non-nol base/quote/BNB baru unit/fixture, belum bukti transaksi live nyata | P2-2/3/7; real opt-in P2-10 |
| C-05 | Grid/MM partial fill, order balasan per level, reservasi base untuk banyak sell; restart/replay diuji ulang saat scope berubah | P2-3/7 |
| C-06 | Daily summary VPS 07.00 WIB dan whitelist chat lain belum punya bukti final; 12 poll failure/1 send failure perlu tinjauan | P2-5/7/9 |
| C-07 | Retry recorder saat DB putus belum diuji ulang di VPS; flush, storage outage dan backfill gagal perlu cakupan | P2-3/7/9 |
| C-08 | ETH paper prospektif belum berjalan; kandidat BTC pembanding, target 50% konsisten belum terpenuhi | P2-6/9 |
| C-09 | Risk live dan keputusan dana nyata belum ditetapkan; persetujuan Fase 11 lama dipindahkan | P2-10 |
| C-10 | Binance Futures: Demo key, order, algo SL/TP, listenKey/reconnect, mapping event, leverage/one-way, reconciliation dan fixture | D-01 |
| C-11 | Bybit private API, unified wallet, spot/linear, fee base, stop orders, contract test dan xspread nyata tertunda akses | D-02 |
| C-12 | Funding historis, multi-leg backtest tersinkron, funding hedge dan pengukuran exposure dua kaki | D-03 |
| C-13 | Kualitas fill intrabar/order-book, latensi/queue MM/grid dan aturan instrumen historis | P2-6; detail L2 D-03 |
| C-14 | Backup luar server ditunda user sampai live; evaluasi operasional/restore dan resource Grafana diteruskan | P2-9/10 |
| C-15 | Inventaris cakupan lama: indikator, sizing, OMS, simulasi, metrik/golden, migrasi, recorder, CLI, web/auth/CSP, Telegram, monitoring | P2-7 |
| C-16 | Keterbatasan paper resume membatalkan entry pending karena rencana proteksi belum tersimpan | P2-1/3/7 |
| C-17 | Saham tokenisasi sudah dikecualikan, pesan stop VPS sudah dikonfirmasi user, backup terjadwal pernah sukses | Selesai; regression P2-7/9 |
| C-18 | Target testnet 72 jam dibatalkan, aktual 43 jam 9 menit; bukan tes 72 jam lulus dan tidak perlu resume trial lama | Arsip; observasi baru P2-9 |
| C-19 | Verifikasi konektivitas mainnet publik/market data, retry gap backfill, namespace ID dan guard mode tetap diperlukan pada rilis baru; tidak berarti mengirim order mainnet | P2-3/7/9 |

C-15 berarti mempertahankan dan memetakan test yang sudah ada ke perubahan
baru, bukan menganggap semua checklist lama gagal/belum dikerjakan. Audit
replay/grid/mm yang ditutup pada 18 Oktober tetap menjadi bukti historis;
temuan 20 Oktober dan perubahan ownership memerlukan verifikasi baru.

## Fase implementasi dan kriteria selesai

Semua checkbox berikut menunjukkan pekerjaan **Plan 2**, bukan hasil Plan 1.
Setiap tahap mencatat file/commit, bukti validasi, dan keterbatasan sebelum
statusnya berubah. Fase eksternal tidak harus menahan build lokal lainnya.

### P2-0 — Audit temuan transaksi dan akuntansi

- [x] Klasifikasi kode log C-01 dan reproduksi respons OCO `-2010` pada entry/trailing;
  uji fallback exit/partial bracket/status terminal. Penyebab tick tepat dan
  penanganan ambigu menyeluruh tetap diteruskan ke P2-3/P2-7/P2-8.
- [x] Telusuri C-02: lima dust DOGE hilang dan cost kandidat terlalu dini berhasil
  direproduksi/diperbaiki dengan regression test. Rekonstruksi PnL penuh tetap
  terbatas bukti agregat; tidak mengubah snapshot historis.
- [x] Tentukan kontrak akhir eksperimen yang menyimpan konfirmasi cancel dan saldo akhir;
  pertahankan bukti run 7/8/9 tanpa memodifikasi hasil historis.
- [x] Klasifikasikan noise DCA tanpa stop sebagai kebijakan strategi; implementasi
  policy/level pesan eksplisit tetap pada P2-4/P2-5, bukan mematikan error proteksi.

Selesai bila penyebab/ketidakpastian dan reproduksi tercatat, prioritas perbaikan
jelas, serta regression test disertakan pada perbaikan dana yang dilakukan.
Hasil: [audit P2-0](testing/p2-0-audit-2026-10-20.md),
[perhitungan reproducible](testing/p2-0-audit-2026-10-20.json).
Audit awal selesai; masalah lanjutan di laporan tetap terbuka pada fase tujuan.
Kode diperbaiki lokal, belum dirilis; trial lama tetap mati.

### P2-1 — Domain, schema, konfigurasi dan migrasi

- [ ] Pisahkan `exchange_account`, `bot_instance`, `strategy_binding`,
  `run/session`, config revision dan identitas kepemilikan.
- [ ] Tambahkan ownership pada order/fill/posisi/trade/snapshot; ID instance tidak
  berubah saat restart dan tidak disamakan dengan `strategy_runs.instance_id` lama.
- [ ] Schema jurnal alokasi/transfer/fee, reservasi durable dan perintah idempoten.
- [ ] Rancang migrasi data lama ke satu instance legacy per akun/mode; mapping
  ambigu masuk laporan untuk ditinjau, tanpa menduplikasi modal/posisi.
- [ ] Kompatibilitas config lama, validasi ID unik, referensi strategi, mata uang,
  mode dan pembatas akun; rencana rollback schema/data sebelum migrasi nyata.

Selesai bila build/config/schema siap, contoh baru jelas ditandai, dan migrasi
punya rencana uji DB salinan. Schema baru tidak langsung dipasang di VPS.

### P2-2 — Ledger modal dan transfer

- [ ] Dana bot yang didanai eksplisit dipisahkan dari koin/saldo pribadi di wallet.
- [ ] Alokasi cadangan → instance dan transfer instance → instance ditulis atomik
  dengan idempotency key; tidak ada kredit tanpa debit pasangannya.
- [ ] Ledger per strategi mempertahankan PnL/fee/inventori; pool instance dibagi
  tanpa menduplikasi saldo. Default shared pool, batas strategi opsional.
- [ ] Hitung tunai bebas, dana dipesan, nilai inventori dan equity secara jelas;
  PnL unrealized/inventori tidak bisa ditransfer sebagai uang tunai bebas.
- [ ] Transfer/top-up/withdrawal tidak dianggap profit; baseline return/drawdown
  disesuaikan terhadap arus modal dengan kebijakan eksplisit.

Selesai bila contoh pembagian satu modal menghasilkan jumlah yang konsisten
pada akun/instance/strategi, dan transaksi gagal tidak meninggalkan saldo parsial.

### P2-3 — OMS, reservasi, ownership dan reconciliation

- [ ] Satu koordinator order per wallet/mode dengan penguncian/lease persisten;
  reservasi atomik mencakup saldo instance dan kemampuan wallet fisik.
- [ ] Buy mengunci quote plus buffer fee, sell mengunci base milik strategi;
  OCO satu kelompok berbagi reservasi, bukan mengunci kuantitas dua kali.
- [ ] Partial fill, fee base/quote/BNB, replay dan status snapshot diproses sekali;
  cancel request belum boleh melepas dana sebelum status exchange pasti.
- [ ] Strategi berbeda boleh memakai simbol sama dengan subledger terpisah;
  exit tidak menjual inventori pemilik lain. Tanpa internal netting di versi awal.
- [ ] Persist rencana entry/proteksi/trailing, deduplikasi fill dan pemulihan
  grid/DCA/MM beserta keputusan adopt/cancel setelah restart.
- [ ] Transaksi manual, deposit/withdrawal, unknown order dan selisih wallet
  menghasilkan temuan; tahan aktivitas terdampak, jangan membagi koreksi sembarang.
- [ ] Feed dibagi lintas instance, cold/warm start dan backfill gagal tidak
  menghasilkan entry dari state yang belum direkonsiliasi.

Selesai bila pemilik setiap dana/order/fill dapat ditelusuri dan recovery tidak
menggandakan event. Race saldo dan crash diuji menyeluruh pada P2-7.

### P2-4 — Runtime, risiko dan kontrol

- [ ] Lifecycle instance/strategi, start/pause/resume/archive, tanpa menghapus
  ledger atau me-reset risk otomatis. Arsip memerlukan order selesai dan posisi flat.
- [ ] Batas dana/posisi/exposure/kerugian pada strategi, instance dan akun;
  instance kecil tidak bisa melewati batas akun lewat banyak order kecil.
- [ ] Pause hanya menghentikan entry; exit/proteksi dan reconciliation tetap aktif.
- [ ] Aksi terpisah: cancel pending, close positions/flatten, stop untuk resume,
  finish experiment. Scope, dampak dan konfirmasi tampil sebelum aksi dana.
- [ ] Saat close/cancel gagal, status `closing`/`needs_attention`, bukan sukses;
  retry idempoten dan kill/reset tidak menghapus status yang belum pasti.
- [ ] Audit perubahan konfigurasi/modal/kontrol, alasan ditolak/menunggu,
  otorisasi web/Telegram dan pemulihan state setelah crash.

Selesai bila perintah scoped tidak memengaruhi instance lain dan risk parent
selalu berlaku. Scope `account` berarti seluruh instance bot yang terdaftar,
bukan saldo pribadi/order manual yang tidak dikelola bot.

### P2-5 — Dashboard, laporan, notifikasi dan template

- [ ] Akun → daftar instance → detail strategi, status paper/testnet/live jelas.
- [ ] Buat/edit/arsip instance, tambah strategi, atur batas, transfer saldo bebas.
- [ ] Laporan net PnL, fee, drawdown, reservasi/inventori, riwayat transfer,
  kesehatan feed/proteksi/reconciliation dan alasan menunggu.
- [ ] Penanda harga/kurs dan timestamp; tampilan USDT + perkiraan USD/IDR;
  desktop/mobile dan light/dark tetap berfungsi.
- [ ] Telegram scoped, whitelist, tingkat notifikasi, digest harian dan deduplikasi;
  jangan membuat satu long-poller per instance dengan token yang sama.
- [ ] Duplikasi template hanya menyalin konfigurasi, bukan modal, posisi atau hasil;
  preview menampilkan dana dan minimum order sebelum instance diaktifkan.

Selesai bila alur pengguna dapat dijalankan pada data simulasi tanpa memahami
schema teknis, dan kontrol tidak mengarah ke instance yang salah.

### P2-6 — Riset strategi dan evaluasi portofolio

- [ ] Backtest konfigurasi instance multi-strategi dengan satu sumber modal dan
  reservasi bersama, bukan menjumlahkan return tes independen.
- [ ] Bekukan konfigurasi ETH utama/BTC pembanding; versi parameter dan sumber
  data/biaya dicatat. Tentukan uji tambahan tanpa menyebut data lama holdout baru.
- [ ] Bandingkan tunai, buy-and-hold, fee/slippage stress, drawdown, turnover,
  durasi/sampel dan konsentrasi aset. Perbaiki model eksekusi bila material.
- [ ] Selaraskan definisi strategi backtest/paper/testnet/live, tetapi simpan
  saldo, sesi, credentials dan hasil tiap mode secara terpisah.
- [ ] Periksa komisi aktual akun/simbol saat relevan; diskon BNB opsional,
  tanpa menganggap fee jaringan murah sebagai fee trading murah.

Selesai bila laporan menunjukkan hasil portofolio bersama setelah biaya dan
batas bukti. Tidak ada klaim target +50% sudah tercapai konsisten.

### P2-7 — Testing menyeluruh

- [ ] Inventaris/reuse test Plan 1 (C-15): indikator, risk/sizing, SimBroker,
  golden/metrik/split, recorder, CLI, migrasi up/down, adapter dan config.
- [ ] Invariant ledger, dua alokasi serentak, transfer ulang, rollback DB,
  reservation buy/sell/OCO, fee base/BNB dan arus modal.
- [ ] Dua strategi pada simbol sama; fill tertunda/duplikat/out of order, cancel
  timeout, status terminal ambigu, OCO, restart di antara debit dan kirim order.
- [ ] Satu pemilik wallet, kehilangan lease, migrasi/resume data legacy,
  unknown/manual order, selisih saldo dan DB/network outage.
- [ ] Integrasi DB terpisah, paper restart, fixture Binance, gate live;
  race detector, build, vet, lint dan pemeriksaan monitoring sesuai CI.
- [ ] Browser mobile/desktop light/dark, auth/CSP/control/confirmation, mode tanpa
  sesi, akurasi chart dan laporan; Telegram scoped/whitelist/digest/shutdown.
- [ ] Uji isolasi akun/mode/instance: tidak ada perpindahan dana atau perintah
  lintas scope; data test tidak menyentuh database eksperimen/produksi.

Selesai bila bukti per area dan daftar temuan tersimpan di `docs/testing/`,
bukan sekadar seluruh checkbox lama disalin menjadi selesai.

### P2-8 — Fixing dan pemeriksaan ulang

- [ ] Urutkan temuan dana/proteksi → ledger/data → akses → operasi/UI.
- [ ] Setiap bug material diberi reproduksi/regression, lalu jalankan ulang
  pengujian terkait dan CI wajib. Tidak membuka fase rilis dengan temuan kritis.
- [ ] Perbarui panduan migrasi, pemulihan dan batas yang belum terselesaikan.

### P2-9 — Rilis dan trial prospektif VPS

- [ ] Rilis source/image digest cocok melalui CI; cek backup, kapasitas, DNS,
  port/resource limit, baseline proyek lain dan rollback.
- [ ] Uji migrasi pada DB salinan; trial memakai database/modal/waktu awal baru.
  Koordinator wallet baru tidak berjalan bersama bot lama pada akun yang sama.
- [ ] Jalankan paper VPS terisolasi menurut [runbook ETH](eth-paper-vps-reference.md),
  setelah kesiapan kode/operasi dinilai; catat konfigurasi yang benar-benar dipakai.
- [ ] Verifikasi restart/persistence/reservasi, proteksi, feed, Telegram digest
  dan whitelist, backup terjadwal/restore, memori Grafana dan ruang disk.
- [ ] Review operasi setelah tujuh hari aktual; penilaian strategi memerlukan
  sampel cukup. Periode 43 jam testnet lama tidak dihitung sebagai trial baru.
- [ ] Akhiri trial dengan cancel/close sesuai keputusan, audit terminal, backup
  dan timer yang konsisten; tidak ada auto-promosi ke live.

### P2-10 — Keputusan live dan evaluasi setelah aktivasi

**Belum disetujui; ini meneruskan Fase 11 Plan 1.**

- [ ] Bukti strategi, kesiapan operasi dan temuan terbuka ditinjau bersama user.
- [ ] Tetapkan modal riil, batas kerugian harian/drawdown, leverage dan scope
  strategi/akun; persetujuan eksplisit sebelum transaksi uang asli.
- [ ] Bila disetujui, uji kecil fee riil dan kecocokan ledger vs exchange;
  penghentian aman tetap tersedia, tidak meningkatkan modal otomatis.
- [ ] Backup luar server dijadwalkan pada tahap live sesuai keputusan user lama;
  catat risiko backup satu server, jangan diam-diam mengubah waktu kebijakan.
- [ ] Tinjau kewajiban akses/legal/pajak aktual saat relevan dengan deployment
  riil; catatan lama bukan verifikasi aturan terkini.

## Pekerjaan bersyarat yang tetap ditunda

### D-01 — Binance Futures

Menunggu Futures Demo key dan keputusan memakai futures. Bawa seluruh C-10,
termasuk format algo/event, expiry listenKey, mapping trigger/partial fill,
leverage/margin/one-way, hedge exposure dan fixture. Kepemilikan posisi futures
pada net position exchange memerlukan desain terpisah; jangan menganggap
subledger Spot otomatis menyelesaikannya. Wajib lolos sebelum futures dipakai.

### D-02 — Bybit dan arbitrase lintas exchange

Menunggu akses user. Bawa seluruh C-11: shared contract test, signing/WS,
status partial/cancel, spot conditional stop/filter/base fee, linear dan unified
wallet, namespace ID order, serta risiko perubahan margin account-wide.
`xspread` memerlukan dua exchange yang benar-benar bisa diuji; tetap nonaktif.

### D-03 — Funding dan kualitas simulasi lanjutan

Histori funding, dua kaki spot/perp tersinkron, leg timpang/cooldown/repair,
exposure hedge dan simulasi multi-akun masuk setelah kebutuhan datanya siap.
L2 order book/queue/latency untuk MM bukan klaim fitur yang sudah dibangun.
Prioritas ini tidak menghalangi fondasi multi-instance Spot.

## Panduan melanjutkan

1. Baca Plan 2, [rancangan](multi-instance.md), keputusan strategi dan handoff VPS.
2. Audit awal P2-0 sudah tersedia; lanjut P2-1/P2-2 dan tindak lanjut laporan.
   Jangan membangun multi-instance dengan
   menggandakan `bot run` pada wallet/DB lama.
3. Catat progres di fase/ID yang sama, tautkan bukti, dan perbarui dokumen lain
   hanya bila kontrak, status atau prosedurnya berubah.
4. Gunakan Plan 1 sebagai bukti sejarah, bukan antrean tugas aktif. Jangan
   menjalankan worker riset selesai, tes lama, atau live dari instruksi arsip.

## Log progres

| Tanggal | Perubahan | Bukti/status |
|---|---|---|
| 2026-10-20 | Plan 1 ditutup; backlog dan fitur baru dipetakan ke Plan 2; rancangan dan navigasi diperbarui | Dokumentasi; belum implementasi kode/deploy |
| 2026-10-20 | P2-0: klasifikasi log read-only, dua bug reconciliation diperbaiki, regression OCO dan audit reproducible | [Audit](testing/p2-0-audit-2026-10-20.md); unit/race/lint/vet/build dan integration DB sementara lolos; belum deploy |
