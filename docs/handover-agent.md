# Handover agent — Presale dan listing baru

## Mulai di sini (status 12 November 2026)

- [Plan 4](plan-4.md) presale/listing baru adalah satu-satunya roadmap aktif.
  P4-1–P4-4 sudah diimplementasi; build, unit/race test, replay paper dan satu
  pass nyata CEX/DEX lulus 10 November. Belum dijalankan: `analyze` (inference
  berbayar), crawl presale via Spider, evaluasi model, pilot, wallet/transaksi.
- Git: `main` berisi merge trading-bot dan perbaikan Plan 4, **belum di-push**
  (sekitar 53 commit di depan `origin/main`). Push hanya atas izin user.
- Menunggu keputusan user: sumber seed daftar presale dan build Spider. Jangan
  memilih sendiri; detail opsi di [checkpoint](testing/plan-4-local.md#verifikasi-10-november-2026).
- Lingkungan: Go hanya tersedia di WSL (`FedoraLinux-44`, shell fish), jadi
  tulis perintah ke file `.sh`. Arsip uji ditulis ke `/tmp`, jangan ke `var/launch`.
- Penamaan (12 November, belum di-build/test ulang): path module Go kini
  `github.com/rizz404/crypto-launch-bot`. Atas izin user (proyek belum ter-deploy)
  semua nama ikut diganti: brand UI, realm login, tag image, image Spider di
  workflow, database `cryptolaunchbot*`, kunci advisory lock, metric/job/alert
  Prometheus `cryptolaunchbot_*`, dashboard Grafana, project compose lokal dan
  path cache/unit skrip riset. Akibatnya volume/DB lokal lama bernama `tradingbot`
  tidak terpakai lagi dan env privat (`.env*`) harus disesuaikan user sendiri.
  Belum diganti: pin digest `trading-bot-spider` di `Dockerfile.catalyst` (image
  dengan nama baru belum pernah di-publish) dan label menu "Trading bot". Sisa
  `trading-bot` di docs merujuk repo asal atau catatan historis.
- Upstream `Rizz404/trading-bot` sesudah `562c73e`: `680a704` (pembanding
  buy-and-hold di dashboard) dan `85def1c` (cache build Dockerfile,
  `deploy/vps-build.sh`) sudah di-cherry-pick 12 November. Tiga commit docs
  mereka sampai `13d8e72` tidak diambil.
- Memori Hindsight untuk sesi ini gagal disimpan; dokumen ini sumber kebenarannya.
- Mulai baca dari [indeks docs](readme.md), lalu [Plan 4](plan-4.md) dan
  [checkpoint pengujian](testing/plan-4-local.md).

**Checkpoint 29 Oktober 2026 (historis).** Commit docs **`839fa2e`** dan
discovery P4-1/P4-2 **`108b3fa`** selesai sesuai permintaan user, lalu P4-3
analisis/policy dan P4-4 paper/simulasi ditulis.

**Merge 10 November 2026:** `Rizz404/trading-bot` sampai `562c73e` (49 commit
sesudah basis bersama `f6ffcd6`) di-merge atas permintaan user. Masuk: UI
bersama, website trading `/trade` multi-mode, marketview Binance, strategi
manual, login email/password, perbaikan catalyst (simbol nonempty, Telegram
tanpa WATCH/REJECT), gateway nexlabs dan penghapusan Bybit. Plan 4 repo itu
menjadi [website trading](trading-website.md) dengan label `TW-*`; dokumen
detail repo itu ada di `agents/` ([handover trading-bot](agents/handover-agent.md)).
Status VPS di sana checkpoint repo lain dan tidak diperiksa dari repo ini.

Resolusi konflik merge (`c48b653`):

- Versi presale repo ini dipertahankan untuk `plan-4.md`, `testing/plan-4-local.md`,
  `handover-agent.md` dan `README.md` (README ditambah ringkasan merge).
- `plan-4.md`, `agents/plan-4.md` dan `testing/plan-4-local.md` milik trading-bot
  menjadi `trading-website.md`, `agents/trading-website.md` dan
  `testing/trading-website-local.md`; judul dan label tahap `P4-x` diganti `TW-x`.
- trading-bot memindahkan dokumen lama ke `agents/` dan `agents/archive/`.
  Tautan banner arsip di file-file itu, `archive.md`, `legacy-readme.md`,
  `handover-agent-legacy.md` dan dokumen presale disesuaikan ke path baru.
- `strategy-2.md` memakai isi trading-bot dengan banner arsip repo ini.
  `testing/readme.md` memakai pengantar presale dan tabel trading-bot.
  `readme.md` dan `archive.md` ditambah entri website trading, panduan dan
  handover trading-bot.
- `.env.launch.example` mengikuti gateway baru `route-gateway.nexlabs.cloud`.
  Makefile ter-merge otomatis; target `launch` tetap ada.
- Pemeriksaan sesudah merge: tidak ada tautan lokal putus di `docs/`,
  `docs/agents/` dan `docs/testing/`, dan tidak ada kerusakan encoding.

**Verifikasi 10 November 2026:** atas permintaan user, build, seluruh unit test,
race test, replay paper fixture dan satu pass nyata CEX/DEX dijalankan dan
lulus setelah tiga perbaikan (`c2a0863`). Discovery presale masih menghasilkan
0 kandidat karena daftar PinkSale dirender JS; user menunda pemilihan sumber
seed dan build Spider. Hasil serta gap: [checkpoint](testing/plan-4-local.md#verifikasi-10-november-2026).

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
- Folder kerja kini `D:\kodingan\personal\crypto-launch-bot`, remote
  `Rizz404/crypto-launch-bot`.
  Commit docs/discovery dan checkpoint P4-3/P4-4 diminta user;
  gunakan `git log` untuk hash checkpoint terbaru. Tidak ada
  push/deploy, perubahan DB atau runtime VPS.

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
pada checkpoint-nya; status itu historis. Tidak ada pemeriksaan server. Jangan menyimpulkan proses sekarang
aktif/mati atau checkout sekarang lulus dari laporan lama.

## Langkah berikutnya

1. Selesaikan sisa penerimaan P4-2–P4-4. Tes/build dan satu pass CEX/DEX sudah
   lulus 10 November. Yang tersisa: keputusan user soal sumber seed daftar
   presale (Telegram resmi, API tidak terdokumentasi, atau lainnya), build Spider
   pinned dan crawl presale, pass berulang untuk coverage/budget/recovery,
   serta pool DEX yang sudah punya metadata pasar.
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
