# Handover agent — Presale dan listing baru

## Mulai di sini (status 12 November 2026)

- [Plan 4](plan-4.md) presale/listing baru adalah satu-satunya roadmap aktif.
  P4-1–P4-4 sudah diimplementasi; build, unit/race test, replay paper dan satu
  pass nyata CEX/DEX lulus 10 November. Crawl presale via Spider lulus
  12 November. Belum dijalankan: `analyze` (inference berbayar), evaluasi model,
  pilot, wallet/transaksi.
- Git: `main` berisi merge trading-bot, rename, cherry-pick upstream dan sumber
  baru Plan 4. Periksa `git status -sb`: bila masih di depan `origin/main`,
  pekerjaan 12 November belum di-push. Push hanya atas izin user.
- Pindah perangkat: yang tidak ikut Git adalah `.env`/`.env.catalyst` (nama DB
  di `TB_DATABASE__URL` kini `cryptolaunchbot`), binary Spider (bangun ulang,
  lihat [panduan CLI](presale-runtime.md#perintah)) dan arsip uji lokal
  (lihat [pass berulang](presale-runtime.md#pass-berulang)).
  Volume/DB lokal lama bernama `tradingbot` tidak dipakai lagi.
- Pass kedua 12 November malam lulus di semua jalur dan pool `0xdbcc…4fd9`
  selesai diselidiki: fair launch PinkSale dengan implementasi ketiga
  `0x07f6…7d45`, kini masuk daftar dikenal. Lihat
  [checkpoint](testing/plan-4-local.md#pass-kedua-dan-implementasi-fair-launch-12-november-malam).
- 13 November: `sale_type` (`presale`/`fairlaunch`) menjadi fakta kandidat dan
  [pass hari kedua](testing/plan-4-local.md#pass-hari-kedua--13-november-2026)
  lulus; cursor DEX, rotasi CEX dan first-seen bertahan antar-pass. Semua sudah
  di-push atas izin user.
- 13 November, lanjutan: user berkata "lanjut terus, pilih yang best practice,
  tanya hanya jika benar-benar crucial" dan mengizinkan `analyze`, policy
  curve/fair launch dan limit lebih besar. Hasil:
  [policy `launch-paper-v2`](presale-analysis.md#policy-yang-dibekukan) dengan
  curve 1%, prompt `launch-analyst-v2`, paper curve setara DEX, pass `-limit 10`.
  Rincian di [checkpoint](testing/plan-4-local.md#pass-ketiga-analyze-dan-policy-v2--13-november-2026).
- 14 November: policy `launch-paper-v3` menetapkan batas exit DEX/curve
  (pajak, holder, likuiditas, biaya curve). Pajak, holder dan gas curve dibaca
  dari chain. Pass nyata: semua tetap `held` karena alasan policy yang konkret,
  bukan lagi `holder_liquidity_and_exit_policy_pending`. Lihat
  [checkpoint](testing/plan-4-local.md#policy-exit-v3-dan-data-curve-dari-chain--14-november-2026).
- 14 November: `analyze` nyata pertama berhasil di perangkat Windows. Luna-low
  ditolak validator (format `asset_key`); `cx/gpt-6.1-sol-medium` dan
  `cc/claude-sonnet-5-5-medium` valid. Lihat
  [checkpoint](testing/plan-4-local.md#analyze-nyata-pertama-dan-pilihan-model--14-november-2026).
- 14 November, lanjutan: pass `collect` keempat jalur berjalan otomatis di
  `plesk-newest` tiap 4 jam (`crypto-launch-collect.timer`, `/opt/crypto-launch`,
  tanpa AI). Set evaluasi 40 kasus `scripts/launch-model-eval` sudah ada; 5
  lulus, 35 menunggu kuota hari berikutnya. Lihat
  [runtime](presale-runtime.md#pass-terjadwal-di-server) dan
  [checkpoint](testing/plan-4-local.md#pass-terjadwal-dan-set-evaluasi-14-november-lanjutan).
- **Pilot shadow `launch-pilot-v1` berjalan sejak T0 2026-11-14T16:41Z** di
  server, tiap pass 4 jam, tanpa AI. Entry baru berhenti 28 November 16:41Z.
  Parameternya dibekukan; jangan mengubah angka di tengah pilot. Lihat
  [evaluasi](presale-evaluation.md#pilot-shadow-launch-pilot-v1-dibekukan-14-november-2026)
  dan [checkpoint](testing/plan-4-local.md#ci-hijau-dan-pilot-shadow-dimulai-14-november-malam).
- 14 November, CI: image kini dipublikasikan sebagai `ghcr.io/rizz404/crypto-launch-bot`
  (bot) dan `ghcr.io/rizz404/crypto-launch-bot-launch` (Plan 4 + Spider); Spider
  dari `crypto-launch-bot-spider@sha256:092dfbbd…`. Job container yang gagal sejak
  12 November karena izin package repo lama kini hijau (run `43613585398`). Image
  lama `ana-core`/`ana-scout` milik `trading-bot` dan server tetap memakainya.
- (Historis, perangkat kantor 13 November) `analyze` gagal di transport karena
  `TB_CATALYST_BASE_URL` di `.env.catalyst` perangkat kantor masih host lama.
  User perlu menggantinya ke `https://route-gateway.nexlabs.cloud/v1` (atau
  membuat `.env.launch`); agent tidak mengubah env privat. Dua slot AI 13
  November WIB sudah terpakai tanpa hasil.
- Langkah berikut: ulangi pass (`-limit 10`) pada hari-hari berikutnya ke arsip
  yang sama; presale belum pernah menemui sale terbuka. Sesudah env diperbaiki,
  ulangi `analyze` 1–2 kandidat. Estimasi bolak-balik kurva (Helper3
  `tryBuy`/`trySell`) sudah menjadi fakta kandidat. Pekerjaan kode berikutnya:
  policy holder/likuiditas/exit yang kini menahan semua kandidat DEX, presale
  dan curve (ambangnya keputusan policy), lalu kurs BNB/USDT dan gas supaya
  quote nyata bisa menjadi route quote paper.
- Seed presale (12 November): kanal pengumuman resmi `t.me/s/pinkecosystem_ann`
  menjadi petunjuk tautan, halaman detail PinkSale dan chain tetap buktinya.
  Atas keputusan user ditambah: `PinkSaleTracking` (tidak resmi, ditandai
  `lead_official: 0`), verifikasi clone/factory kontrak sale, GeckoTerminal
  sebagai metadata pool DEX, dan venue keempat four.meme (kind `curve`, sebatas
  discovery; analisis/paper belum mengenalnya). Pass nyata lulus untuk presale,
  DEX dan curve; belum ada sale presale yang masih terbuka. Lihat
  [checkpoint](testing/plan-4-local.md#sumber-tambahan-12-november-lanjutan).
- Lingkungan: sejak 14 November proyek lintas platform. `go test ./...` dan
  test Python lulus di Windows dan Linux; CI menambah job Windows. Lihat
  [checkpoint](testing/plan-4-local.md#dukungan-windows--14-november-2026).
  WSL tidak lagi wajib (perangkat kantor masih bisa memakai `FedoraLinux-44`
  dengan `GOTOOLCHAIN=go1.27.2`). Arsip uji jangan ke `var/launch`; di perangkat kantor arsip dan binary ada di
  `~/.local/state/crypto-launch-bot/lv` (WSL) karena `/tmp` terhapus saat restart.
- Penamaan (12 November; build, unit, integrasi dan test alert lulus sesudahnya): path module Go kini
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
selalu mematikan entry. Archive jalan di Linux dan Windows sejak 14 November.
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
