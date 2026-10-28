# Handover VPS

Checkpoint operasi P2 **23 Oktober 2026**, pilot paper P3 aktif **28 Oktober
22.53 WIB**. Prosedur perubahan ada di
[deployment](deployment.md); rencana aktif di [Plan 3](plan-3.md), dengan
pembanding lama tetap mengikuti [Strategi 2](strategy-2.md).

## Pilot paper P3 aktif — 28 Oktober 22.53 WIB

User mengizinkan deploy setelah lanjutan pilot. Worker paper P3 berjalan
terus-menerus (`restart: unless-stopped`) di project `trading-bot-catalyst`
dengan DB, resolver DoT dan wallet sendiri. **Uang virtual saja**, tanpa order
sungguhan. P2 tidak direstart; identitas container P2 identik sebelum/sesudah.

| Item | Nilai |
|---|---|
| Source berjalan | `1779878` (pesan start Telegram), sebelumnya `05ce48a`/`41d22b0`; start awal `43b6f61` 22.53 WIB |
| Image | `ghcr.io/rizz404/trading-bot-catalyst@sha256:4579285aee19eeab366b4f87f6eb08d0340f8be8b1a5f2b4833b5ac3a3904cdf` (redeploy ±23.35 WIB, Spider `spider_cli 2.53.9-p3.1`) |
| Spider artefak | `ghcr.io/rizz404/trading-bot-spider@sha256:480faea43abdb2b2e8bd6cf136ecd250e8c37039e925f0bdb38f14385a318e19`; CI katalis turun ±12 → ±1,7 menit |
| CI | [40717858676](https://github.com/Rizz404/trading-bot/actions/runs/40717858676) dan [40720835367](https://github.com/Rizz404/trading-bot/actions/runs/40720835367) lulus seluruh job; [40724154160](https://github.com/Rizz404/trading-bot/actions/runs/40724154160) untuk `05ce48a` |
| Wallet / DB | `paper:catalyst-catalyst-paper-v1` / `tradingbot_catalyst_paper_v1`, role `catalyst`, volume `trading-bot-catalyst_db-data` |
| Modal / risiko | 1.000 USDT virtual, 0,5%/trade, maks 2 posisi, halt harian 2% / drawdown 8% |
| AI | `cx/gpt-6-luna-low` (served `gpt-6-luna`), prompt v5, maks 12 panggilan/hari WIB |
| Policy hash | `b11fc4a1b144dd71c41dc1ffdf0a9f3e85edbd63b91bb1ec3f5d55899aa85f12` |
| Sumber | Ethereum blog, Anza blog (SOL), Dogecoin blog; masing-masing seed + 4 artikel terbaru |
| Dashboard | `127.0.0.1:18081` (SSH tunnel), user `pilot`; password di `.env.catalyst` VPS mode 600 |
| Telegram | Sender-only memakai bot dan chat ID P2 (pilihan user); tidak polling/menu, pesan berawalan `P3 paper`; start dan setiap eksekusi |
| Env | `.env.catalyst` (model, dashboard, token/chat Telegram disalin dari env/config P2 di VPS) dan `.env.catalyst-image` (digest + password DB dibangkitkan di VPS); tidak disalin dari P2 |

Preflight dengan image baru: `market` membaca ETH/SOL/DOGE book + ATR nyata,
`collect` sukses tiga sumber. Siklus riset pertama memanggil model sekali
(10.058 token input / 436 output): **WATCH DOGEUSDT**, tidak ada event terjadwal.
Review itu ditolak validator (`citation source outside policy`) karena mengutip
bukti ETH/SOL; `41d22b0` mengizinkan kutipan lintas aset hanya untuk WATCH,
entry/hold tetap wajib bukti primer aset sendiri. Setelah redeploy, siklus
berjalan tanpa error dan tanpa panggilan baru (bukti tidak berubah); kuota
terpakai 1/12 pada 28 Oktober.

Belum: backup/restore DB dan arsip P3, observer/alert P3, QA browser
dashboard. Review pilot hari 1/3/7: **29 Oktober / 31 Oktober / 4 November
22.53 WIB**. Tidak ada entry sampai sumber primer memuat jadwal mainnet/produk
eksplisit (event Hoodi sekarang testnet dan ditunda).

### Temuan P2 untuk review hari ke-7 (29 Oktober 19.58 WIB)

- Snapshot ledger 28 Oktober ~22.48 WIB: modal 747,84 USDT, ekuitas 741,10
  (−6,74 USDT, −0,9%). Tren ×3 dan EMA 4h belum bertransaksi. EMA 1m dasar
  5 menang/50 kalah (−3,82, komisi 2,97); EMA 1m tren 2/36 (−2,92, komisi 2,06).
  Sebagian besar rugi adalah komisi; usulan: nonaktifkan dua EMA 1m saat review.
- Websocket Binance P2 terputus ~9×/jam sejak 23 Oktober, mayoritas
  `StatusPolicyViolation ... "Pong timeout"` dan gagal menulis pong; candle 1m
  sempat tertinggal ±5 menit. Perlu perbaikan kode + restart P2 dengan izin user.

## Rilis komponen P3 — 28 Oktober

User mengizinkan commit, push dan deploy. [Seluruh CI lulus](https://github.com/Rizz404/trading-bot/actions/runs/40699358329),
termasuk race/vet/lint/build, integrasi DB/recovery, monitoring serta image/
protokol Spider. Image khusus ditarik dari GHCR; source label dan checkout cocok.

| Item | Hasil terverifikasi |
|---|---|
| Source rilis | `2998f386f5aead3cc19e0f2dee1c0abe735c2618` |
| Image khusus | `ghcr.io/rizz404/trading-bot-catalyst@sha256:cee992d6f6cd908569e072d98d07831d04b2efab26f006bf74e2a3e2997ce79d` |
| Project / direktori | `trading-bot-catalyst` / `/opt/trading-bot-catalyst` |
| Container | `trading-bot-catalyst-catalyst-1`, exited 0 sesuai aksi sekali jalan |
| Start / selesai | 28 Oktober 21:00:44,059 / 21:00:44,575 WIB |
| Batas / isolasi | 0,5 CPU, 512 MiB, pids 128; UID 65532, read-only, cap-drop ALL, no-new-privileges, tanpa port publik |
| State P3 | Volume arsip sendiri; default report shadow, tiga sumber `missing` karena belum pernah collect |
| Bukti server | `evidence/release.json`, `report.json`, `deployment.log`, `containers-before.txt`, `containers-after.txt` |

**Ini pemasangan komponen/report, bukan worker AI atau start trial.** Tidak ada
crawl publik, inference, scheduler, DB/wallet/modal P3 atau T0 pilot. Gate situs,
egress adversarial, retensi, input pasar/event, runner dan penerimaan pilot tetap
terbuka. Dua halaman fixture CI bukan benchmark kapasitas situs nyata.

Metadata **35 container yang sudah ada identik** sebelum/sesudah. Bot P2 tetap
image `02376651f56e...`, started `2026-10-23T13:52:17.164470608Z`, restart 0.
Tidak restart P2/Docker/host, mengubah env/config/ledger/T0 P2, migrasi database
atau menyalin secret laptop. Ini pemeriksaan identitas/isolasi, bukan audit
saldo/feed terkini P2. Bukti lengkap: [rilis P3](testing/catalyst-release.json).

## Pergantian rencana 23 Oktober

Plan 2 ditutup sebagai siklus atas permintaan user; backlog operasi/review
diteruskan ke [P3-O](plan-3.md#p3-o--penerimaan-operasi-dan-tindak-lanjut-audit),
ekspansi ke P3-R, dan live/exchange bersyarat ke P3-LIVE/P3-D. Pergantian plan
tidak mengubah proses, ledger, modal, parameter atau T0 tahap dua. Strategi AI
belum aktif; pilotnya kelak membutuhkan DB/wallet terpisah dan izin start.

## Inspeksi penempatan Spider — 26 Oktober

Atas izin user dilakukan inspeksi read-only kapasitas dan container. Host 8 vCPU,
RAM tersedia ~15,7 GiB, disk kosong 121,23 GiB; sampel CPU bot P2 68,42% dari
satu core, DB 52,77%. Bot container restart 0/OOM false. Ini bukan audit ledger,
feed atau benchmark puncak; tidak memperbarui saldo/T0 historis di bawah.

Usulan: container/Compose project `trading-bot-catalyst` dan direktori
`/opt/trading-bot-catalyst` terpisah, belum dibuat/diinstal. Tidak mengubah P2,
Plesk, gateway, Docker daemon atau layanan lain. Bukti:
[preflight kapasitas](testing/catalyst-vps-preflight.json); detail dan pekerjaan
sisa: [handover Spider](catalyst-spider-handover.md).

## Status terakhir P2

### Rilis notifikasi transaksi — 23 Oktober 2026

**Aktif sejak 20.52 WIB:** runtime `9c18962` memasang perbaikan notifikasi
`afdab52` dan koreksi lint tes. [CI lengkap sukses](https://github.com/Rizz404/trading-bot/actions/runs/39963665890),
image `ghcr.io/rizz404/trading-bot@sha256:02376651f56e2291c41fef664a44691a9279686b4a89288a2dba020528e097ca`.
Image ini menggantikan versi pada checkpoint 22 Oktober di bawah.

Notifikasi beli/jual mengikuti eksekusi yang sudah tersimpan, termasuk parsial;
level instance `info` diperlukan. Penjualan memakai realisasi ledger. Ringkasan
07.00 WIB kini mencakup aktivitas periode 07.00–07.00 WIB, realisasi pada
eksekusi, komisi serta PnL sejak awal. Koreksi komisi terlambat mengikuti total
ledger; MM tetap diringkas. Cache dedup peringatan dibatasi satu menit dan
memisahkan tingkat pesan sehingga warning tidak menahan error berteks sama.

Backup pra-rilis `backup-20261023T134412Z` lolos checksum dan restore ke DB
sementara. Image baru cocok dengan source; status migrasi 1–5 pada salinan dan
target sudah applied. DB sementara telah dihapus. Hanya container bot yang
diganti dengan `up -d --no-deps bot`; tidak ada perubahan schema, strategi,
alokasi atau penghapusan history. Config privat/hash tetap sama.

Snapshot sebelum/sesudah restart cocok persis: **6 instance/18 binding**, kas
**746,92125354748169 USDT**, 22 eksekusi tersimpan, tanpa posisi terbuka/pending,
halt false dan attention kosong. T0 eksperimen tetap 22 Oktober 19.58.45 WIB.
Sebanyak 32 container selain bot (28 proyek lain + empat infrastruktur bot)
tidak berubah dan lima service host tetap active. Health dan build-info cocok
dengan mode paper/versi baru; pemeriksaan awal tidak menemukan WARN/ERROR baru.
Observer 20.54 WIB `problems: []`; candle 1m BTC/ETH/SOL setelah restart
tersimpan tanpa gap. Backup pascarilis `backup-20261023T135257Z` disimpan
terpisah dari pra-rilis dan checksum cocok.

Bukti: [rilis notifikasi](testing/telegram-notification-release.json). Ini
pemeriksaan awal, bukan bukti operasi jangka panjang. Penerimaan di aplikasi
Telegram, pesan eksekusi alami berikutnya, ringkasan 24 Oktober 07.00 WIB dan
restore backup pascarilis belum diverifikasi. Pesan Telegram tetap best effort;
history lama yang sudah selesai tidak dikirim ulang saat restart.

### Tahap dua — pembanding frekuensi

**Rilis koreksi 22 Oktober 20:21 WIB:** source `25a3ad5` (perbaikan runtime
`740c475`) sudah di-push, [CI lengkap sukses](https://github.com/Rizz404/trading-bot/actions/runs/39785037592)
dan image `sha256:1cc0cdecc604ea7c602998f4e0c161dd45904ee9fb16ac8d8f05ba976fb45a8f`
dipasang lewat pull GHCR dan checkout GitHub. Manifest CPU1 kini berasal dari
commit, bukan edit server yang belum di-commit. Config privat/hash/modal dan
T0 ledger tetap sama. Image `37ffd6a` pada tabel berikut adalah versi saat start,
bukan image terkini.

Sesudah sampel start awal, observer mendeteksi feed tertinggal: proses/health
tetap aktif, tetapi candle1m terakhir13:03 UTC tidak mengikuti waktu pasar.
Jalur `drain` memuat dan mengurai seluruh ledger lagi untuk setiap binding
directional, dipanggil berulang pada loop binding tiap update. Perbaikan memakai
snapshot yang sudah dimuat untuk pemeriksaan idle, dan memuat ulang setelah
close yang dapat mengubah dana/eksekusi. Pemeriksaan proteksi tetap dijalankan.
Regression membuktikan satu pembacaan ledger pada drain idle serta saldo view
tetap mengikuti transfer; tes managed/config dan seluruh CI lulus.

Backup sebelum koreksi `backup-20261022T131455Z` lolos checksum dan restore ke
DB scratch (6/18, capital747,83957456237531, schema5); image baru memeriksa
status migrasi scratch sebelum target disentuh. Scratch dihapus. Tidak ada
migrasi baru, reset modal, perubahan parameter atau penghapusan history.
Mulai observasi kapasitas berkelanjutan setelah rilis ini; pemeriksaan awal
beberapa menit sebelumnya bukan kelulusan24jam. Bukti rilis terbaru:
[research-stage-two-release.json](testing/research-stage-two-release.json).

Pemeriksaan pascarilis 20:25–20:26 WIB: observer `problems: []`, semua seri
gap 0; candle 1m BTC/ETH/SOL maju dari open 13:21 ke 13:24 UTC, tanpa restart/OOM.
Sampel 30,31 detik: bot CPU 56,30% dari satu core/RAM 78,44 MiB, tanpa throttling
baru; DB CPU 54,56%/RAM 221,58 MiB dan tiga periode throttled. Ini beban singkat,
bukan batas puncak teruji. Masih ada akses DB/no-op yang perlu dikurangi sebelum
skala besar. Dashboard auth 200, health ok, 28 container lain identik dan lima
service host tetap aktif. Backup akhir `backup-20261022T132410Z` checksum cocok.

Ledger 20:26 WIB membuktikan dua fill simulasi SOLUSDT pada `ema-base-1m` dan
`ema-trend-1m`, masing-masing 0,265 SOL, dengan stop loss dan take profit open.
Pending 0, halted false dan attention kosong. Dua log `order rejected` ETHUSDT
beralasan `parent position count limit exceeded`, sesuai batas satu posisi
per instance yang sudah terisi SOL; bukan error koneksi exchange. Siklus exit
lengkap dan profitabilitas belum diverifikasi. Entry ini sesudah sampel awal
yang mencatat nol order, jadi jangan memakai snapshot start sebagai status kini.

User menyetujui penerapan rencana pada 22 Oktober setelah pemeriksaan kapasitas
host/saldo. **Tahap dua aktif sejak 22 Oktober 2026 19:58:45 WIB**
(`12:58:45.651873337 UTC`), container mulai `12:58:42.822178305 UTC`.
Ini menggantikan trial aktif tahap satu di bawah; histori tersebut tetap disimpan.

| Item | Tahap dua |
|---|---|
| Konfigurasi publik | `configs/managed.research-stage-two.yaml` |
| DB / wallet | `tradingbot_research_stage_two` / `paper:research-stage-two` |
| Instance | `tren-500k`, `tren-2000k`, `tren-5000k`, `ema-base-1m`, `ema-trend-1m`, `ema-trend-4h` |
| Binding / stream | 18 binding; enam stream BTC/ETH/SOL 1m/4h |
| Modal | 747,83957456237531 USDT virtual; Rp13,5 juta pada kurs pembanding Rp18.052 |
| Risiko | 1%/trade, posisi 25% dan satu posisi per instance; enam posisi akun |
| Biaya simulasi | 0,1%/sisi, slippage 2 bps |
| Kuota bot | 1 core / 512 MiB; DB tetap 1 core / 1 GiB |
| Source/image | Image terbit `37ffd6a`, digest `sha256:fc47df3e382be6e746a4b5b3658291c1330d3d4dc911b6fb8b777945c6411b72` dipertahankan |
| Config privat SHA256 | `8687ca0ca42629b642920c2d27cacbaf2cc3533cb1d5ad48673c3b027822d893` |
| Config ledger hash | `af4c6a5018dfac769769c92cb7b88b95a0000e5fbfebf57e78e09117c2bb4b36` |
| Backup sebelum | `backup-20261022T125650Z`, seluruh checksum cocok |

Aturan EMA memakai 12/26, RSI14 <=70, ATR14 stop2/target3, exit cross lawan,
tanpa short/trailing. Filter EMA100 pada dua pembanding, tanpa filter pada
baseline. Seluruh instance baru Rp2 juta virtual; bandingkan dengan Tren
Rp2 juta sejak T0 bersama. Tidak mereproduksi persis saldo/universe paper awal.

Preflight: restore backup tahap satu ke DB scratch berhasil, modal/config/kas
cocok dengan sumber dan tidak ada holding/order/eksekusi; scratch dihapus.
Migrasi target kosong 1–5 memakai image yang sama berhasil. Source runtime
tidak disalin/dibangun ulang; konfigurasi trial dibuat dari konfigurasi server
yang sudah ada. Manifest Compose server memiliki perubahan kuota bot 0,5 →1,
sama dengan diff lokal; jangan menghilangkannya pada checkout/deploy berikut.
`.env.vps` memilih DB tahap dua, config aktif UID65532 mode600. Tidak ada
commit/push baru. Backup pasca-start `backup-20261022T130313Z` memuat empat
database proyek dan config, seluruh checksum cocok.

Awal start: enam instance dan 18 binding running, tidak halted/attention,
reserve/order/holding/eksekusi kosong, alokasi tepat. Snapshot 28 container
proyek lain identik sebelum/sesudah. Observer pertama `12:59:19 UTC` tidak
melaporkan masalah; health DB dan startup stream berhasil. Ini pemeriksaan awal,
bukan bukti operasi 24 jam atau profitabilitas. Sampel 31,79 detik sebelum
restart: CPU bot 34,53% dari satu core, RAM75,2MiB, DB CPU32,89%/RAM220,16MiB,
tidak ada periode throttled baru; 9,44 revisi ledger/menit dan 237,05 commit
DB/detik. Enam seri masing-masing punya 300 candle warmup dan candle1m baru
tersimpan. Dashboard dengan auth200. Pengukuran ini bukan benchmark puncak.

Restart terkontrol pukul20:02 WIB memulihkan enam instance/18 binding, saldo,
spec dan waktu awal ledger tanpa reset. Belum menguji recovery dengan order
atau posisi terbuka pada trial ini. Observer20:03 WIB `problems: []`; snapshot
akhir 28 container lain identik, hash ledger tahap satu sesudah stop tetap sama.
Log shutdown memuat sembilan `store candle` dengan `context canceled`, dan
run ditandai `interrupted`. Feed pulih tanpa gap terdeteksi; klasifikasi run
dan log saat penghentian tetap temuan terbuka, bukan alasan mengedit history.
Bukti lengkap: [start tahap dua](testing/research-stage-two-start.json).

Tahap satu dihentikan setelah dipastikan tidak ada posisi/order; kas tetap
415,46643031243073 USDT, config disimpan sebagai
`configs/config.research-stage-one-stopped.yaml`, DB lama tidak dihapus/diubah
untuk alokasi baru. Run lama mencatat `interrupted` pada penghentian terkontrol
`12:58:41 UTC`; penyebab klasifikasi shutdown perlu ditinjau, jangan mengubah
label via SQL atau menyebutnya crash yang sudah dibuktikan. Review tahap satu
22/24/28 Oktober digantikan review tahap dua **23/25/29 Oktober 19:58:45 WIB**.
Tidak ada reminder AI atau ekspansi otomatis yang dibuat.

Target riset 44 instance/132 binding tetap bertahap. Yang berjalan sekarang
6/18; EMA15m/1h, varian modal lain dan kelompok pullback belum diaktifkan;
squeeze, mean reversion dan rotasi belum diimplementasikan. Kenaikan berikutnya
menunggu pengukuran kapasitas dan minimal 24 jam tahap ini termasuk pemeriksaan
persistence/restart sesuai protokol; bukan izin menjalankan 44 sekaligus.

### Arsip checkpoint tahap satu

**Paper riset tahap awal sudah berjalan** setelah user menyetujui push dan
start VPS. T0 ledger **21 Oktober 2026 23:58:57 WIB** (`16:58:57 UTC`);
container mulai dua detik sebelumnya. Ini uang virtual dengan data pasar publik
Binance mainnet; tidak ada order yang dikirim ke exchange.

| Komponen trial aktif | Kondisi |
|---|---|
| Profil | `managed.research-stage-one.yaml` + Telegram whitelist privat |
| Komposisi | 3 instance Tren, 9 binding, BTC/ETH/SOL; 6 stream unik 1m/4h |
| Modal | 415,46643031243073 USDT virtual; perbandingan Rp7,5 juta pada kurs historis Rp18.052 |
| Database / wallet | `tradingbot_research_stage_one` / `paper:research-stage-one` |
| Source / CI | `37ffd6a798bbe0d14bb8b96decfc60c52dfc1ddc`; [semua job lulus](https://github.com/Rizz404/trading-bot/actions/runs/39638221196) |
| Image | `sha256:fc47df3e382be6e746a4b5b3658291c1330d3d4dc911b6fb8b777945c6411b72` |
| Config privat SHA256 | `d2b163e5ca6ae28425950a694b67f4564c0cf4e8c4e0130a04e6dee204149792` |
| Config ledger hash | `a10572ca03406c41bb564141fc4923d4c7b553879018490783262e77fe9d7356` |
| Observe / backup timer | Active; observasi 5 menit dan backup harian, tanpa recovery otomatis |
| Backup sebelum / sesudah | `backup-20261021T164954Z` / `backup-20261021T170053Z` |

Pemeriksaan awal: ledger tiga alokasi cocok, sembilan binding running, tidak
halt/attention/pending, belum ada posisi/order. Enam seri masing-masing mempunyai
300 candle warmup; candle 1m baru masuk. Monitoring sempat `prometheus_target`
pada scrape pertama saat pergantian container, lalu `problems: []`. Menu Telegram
chat terverifikasi lewat `getMyCommands`: 19 command termasuk balance/pnl.
Dashboard 401 tanpa auth dan 200 dengan auth. Pengiriman notifikasi startup tidak
mencatat error; tampilan aplikasi Telegram dan ringkasan harian berikutnya belum
diverifikasi. Timestamp harga adalah batas candle, bisa berada di masa depan
pada candle 4h yang sedang terbentuk; gunakan seri candle/monitoring untuk cek feed.

Backup sebelum deploy valid checksum dan berhasil direstore ke DB scratch;
ledger/jurnal/schema 5 cocok dengan ETH lama. Image baru memeriksa migrasi copy
dan menerapkan migrasi 1–5 pada target kosong. Scratch dihapus setelah cocok;
hash ledger ETH lama tetap sama. File config aktif dimiliki UID 65532 mode 600
agar user non-root container dapat membacanya; jangan mengubahnya menjadi
root:root mode 600. Percobaan migrasi pertama tertahan izin file, sebelum start.
Snapshot 28 container proyek lain identik sebelum/sesudah; lima service host
tetap aktif. Bukti rinci dan batas pemeriksaan: [start riset](testing/research-stage-one-start.json).

Sampel beban 163,51 detik: bot sekitar 56 MiB/512 MiB dan CPU 51% dari satu
core (quota 0,5 core), DB sekitar 189 MiB/1 GiB dan CPU 50%. Sekitar 18 revisi
ledger/menit, state 30 KB, tetapi 543 commit DB/detik karena pembacaan/transaksi
berulang per update/binding. WAL cluster sekitar 371 KB/menit termasuk backup
dan DB lain. Ini sampel singkat, bukan benchmark beban puncak. **Jangan naik
ke 6/12/24 instance** sebelum mengurangi pembacaan/round-trip ledger dan mengukur
ulang, termasuk saat entry/exit dan batas candle 4h. Tidak ada perubahan kode
runtime/parameter tambahan setelah image tersebut untuk menyamarkan beban ini.

### Trial ETH sebelumnya — tetap dihentikan

| Komponen | Kondisi |
|---|---|
| Bot ETH paper P2-9 | Stopped/exited 0 sejak 21 Okt 13:30:39 UTC, atas permintaan user |
| Ledger | `paper:eth-research`, instance `momentum`, binding `momentum/eth4h` |
| Modal | 110,79104808331486 USDT; posisi, order, eksekusi dan arsipnya kosong |
| Database trial | `tradingbot_eth`; DB testnet `tradingbot` tetap dipertahankan |
| Source | `c14aa087141583a9f185c5b8b5f595009988c41c` |
| Image digest | `sha256:5a50dd484438c0739093b6417622bfea35c11f96fd9fb815ad3e6976c3333424` |
| Backup final | `backup-20261021T133042Z` |
| Observe timer | Disabled/inactive |
| Backup timer | Active, harian sekitar 03.15 UTC / 10.15 WIB |
| DB, DNS, Prometheus, Grafana | Running |

Jadwal laporan hari 1/3/7 dan review tujuh hari **dibatalkan**. Jangan resume
trial atau memakai ledger ini sebagai eksperimen baru. Status instance tersimpan
masih `running` karena yang dihentikan prosesnya; ini bukan status container.

Bukti: [start](testing/p2-9-vps-start.json), [stop](testing/p2-9-stop.json),
[review](testing/p2-9-review.json). Trial dihitung sejak 20 Okt 16:24:45 UTC;
start sebelumnya mengalami crash loop warmup dan tidak dihitung.

## Temuan terbuka

### Permintaan pengujian berikutnya dan review hari 1/3/7

Backlog aktif: P3-O1–P3-O3. Bagian berikut merekam permintaan tahap satu dan
protokol review; angka konfigurasi tahap satu adalah arsip. Jadwal aktif tahap
dua dari T0 22 Oktober 19:58:45 WIB: **23/25/29 Oktober 19:58:45 WIB**.

Pada 21 Oktober 2026 user meminta commit semua perubahan, memulai "tes live",
dan akan kembali melapor pada hari 1, 3 dan 7. **User kemudian mengonfirmasi
paper di VPS: harga pasar asli dengan uang virtual, tanpa transaksi uang asli.**
User menyetujui push/deploy/start; hasil aktual tercatat di atas. Demo Telegram
sebelumnya tetap fixture dan tidak menjadi bagian performa trial ini.

Rancangan 24 instance/72 binding belum runnable: rotasi, squeeze, mean reversion,
antrean VPS dan guard kapasitas masih pekerjaan terbuka. Jumlah itu plafon
bertahap, bukan konfigurasi siap diaktifkan. Jangan membuat strategi pengganti
dengan nama yang sama atau memakai ledger trial ETH lama untuk mempercepat start.

Konfigurasi tahap awal yang disiapkan:
[managed.research-stage-one.yaml](../configs/managed.research-stage-one.yaml).
Ini tiga varian komposisi Tren yang sudah ada pada rancangan: `tren-500k`,
`tren-2000k`, `tren-5000k`, masing-masing momentum BTC 4h, momentum ETH 4h dan
pullback SOL 4h. Jadi **3 instance / 9 binding / 3 koin / 6 stream unik**,
bukan aktivasi semua 24 instance atau penggantian strategi yang belum tersedia.
Modal Rp500 ribu + Rp2 juta + Rp5 juta = **Rp7,5 juta virtual total**,
415,46643031243073 USDT pada kurs perbandingan historis Rp18.052. Tidak ada
penggandaan modal per binding, tidak ada dana riil atau leverage.

Profil bersama tahap awal: risk sizing 1% per transaksi, posisi maksimal 25%
ekuitas instance, satu posisi per instance, tiga posisi akun; kill harian 5%
dan drawdown 20%, close-on-kill. Komisi simulasi 0,1% per sisi dan slippage 2 bps.
Ini profil operasional baru untuk perbandingan modal, bukan profil ETH lama
3%/75%, dan belum memiliki hasil evaluasi portofolio atau kalibrasi beban baru.
Aturan sinyal memakai momentum 84×4h >5%/EMA100 serta pullback RSI40/65/EMA200
yang sudah tersedia. Parameter lengkap ada pada config, dibekukan untuk trial.
Alokasi kecil tetap tunduk minimum order; jangan memaksa entry bila tidak layak.

Template tetap mematikan Telegram; deployment mengaktifkannya dengan whitelist
privat yang sudah ada dan satu long-poller. Target DB baru yang disiapkan adalah
`tradingbot_research_stage_one`, wallet `paper:research-stage-one`.
Config SHA256: `8cc676de2af2b37fd1b455611104a7b65867f166f962c660b8161cefc4839371`.
Tes load config, jumlah binding, satu sumber modal dan batas posisi/leverage
lulus. Profil sudah dirilis dan dijalankan setelah persetujuan eksplisit;
kapasitas 24/72 dan evaluasi strategi baru tetap belum teruji.

Sebelum start, catat mode yang dikonfirmasi, strategi/koin/aturan yang benar-benar
siap, modal total/alokasi, batas risiko, database baru, source commit, image
digest, config hash, baseline proyek lain, kapasitas dan temuan operasi terbuka.
Rilis mengikuti [deployment](deployment.md); commit lokal tidak berarti sudah
di-push, diterbitkan CI atau dipasang di VPS. Mainnet memerlukan modal riil,
pilihan strategi dan batas kerugian yang eksplisit; jangan memakai Rp105 juta
virtual atau batas paper sebagai keputusan uang asli.

Jadwal dihitung dari **T0 = start trial baru yang benar-benar diverifikasi**,
bukan waktu commit, pengiriman demo atau start trial ETH lama:

| Review user | Waktu relatif | Fokus agent saat user kembali |
|---|---|---|
| Hari 1 | T0 + 24 jam | Proses/feed, gap, warmup, restart, ledger/order, proteksi, Telegram dan beban CPU/RAM/DB |
| Hari 3 | T0 + 72 jam | Keandalan berkelanjutan, fee/PnL, reservasi, pemulihan, backup dan perubahan beban |
| Hari 7 | T0 + 168 jam | Rekap operasi, seluruh transaksi/biaya, drawdown, jeda tanpa entry, kualitas data dan keputusan lanjut/henti |

User boleh melapor terlambat; catat durasi aktual. Tidak memasang jadwal AI,
polling berkala atau perubahan otomatis di antara laporan tanpa permintaan.
Observasi layanan yang disepakati saat start boleh mengumpulkan bukti, tetapi
bukan alasan agent mengubah parameter/modal atau memulai ulang proses sendiri.
Review tujuh hari bukan bukti profit konsisten atau promosi mode/modal otomatis.

Minta user melapor lebih cepat bila start/stop berulang, peringatan saldo/order
pelindung/`perlu diperiksa`, atau ringkasan 07.00 WIB hilang dua hari berturut.
Pada review, cocokkan bukti VPS dengan Telegram dan timestamp harga; pesan
"proses aktif" atau `healthz` sendiri tidak membuktikan seluruh feed sehat.
Jangan memaksa entry jika sinyal belum muncul. Tanggal kalender review baru diisi
setelah T0 tercatat. Jadwal tahap satu yang sudah digantikan (WIB):

| Review | Waktu |
|---|---|
| Hari 1 | 22 Oktober 2026 23:58:57 |
| Hari 3 | 24 Oktober 2026 23:58:57 |
| Hari 7 | 28 Oktober 2026 23:58:57 |

Jadwal di tabel sudah dibatalkan/digantikan tahap dua, bukan reminder AI
terjadwal atau instruksi untuk mengaktifkan kembali tahap satu.

### Temuan trial sebelumnya

- Bot dan 20 container proyek lain mulai kembali sekitar 21 Okt 03:23:55 UTC.
  Preflight berikutnya menemukan journal Docker: systemd menghentikan Docker
  pukul 03:23:40 UTC. WARN persist bot berisi DB connection terminated by
  administrator pada waktu sama. Ini restart service Docker, bukan reboot host;
  pemicu administrasinya belum ditelusuri. Jangan restart Docker untuk bot ini.
- Baseline historis 17 Oktober karena itu tidak lagi `differences: []`.
  Simpan selisih dan bandingkan kondisi sebelum/sesudah; jangan reset baseline
  untuk menyembunyikan perubahan yang belum dijelaskan.
- Observasi trial: 256 sampel, satu masalah `metrics` pada 03:23:33 UTC.
  Log tersedia memuat 3 WARN Telegram poll dan 1 WARN persist shutdown.
- Backup divalidasi daftar arsip/checksum saat dibuat; penutupan tidak mencakup
  restore drill baru. Backup masih di disk server yang sama.

## Akses dan batas

`ssh -o BatchMode=yes plesk` menuju server Plesk bersama. Cakupan bot adalah
project `trading-bot-paper` di `/opt/trading-bot-paper` dan komponen P3 terpisah
`trading-bot-catalyst` di `/opt/trading-bot-catalyst`. Jangan mengubah
Plesk/domain/firewall/web server/Docker daemon, restart host,
prune global atau menghapus volume. Mainnet belum diizinkan; manifest memaksa
`TB_APP__ALLOW_LIVE=false`.

```text
/opt/trading-bot-paper/
  access/       deploy key read-only dan known_hosts proyek
  repo/         checkout rilis, .env.vps privat, configs/config.yaml
  evidence/     baseline dan sampel observasi
  backups/      tujuh backup sukses terbaru, berisi secret
```

Config aktif adalah profil riset tahap awal dengan Telegram. ETH dihentikan dan
config-nya tersimpan `configs/config.eth-stopped.yaml`; testnet lama di
`configs/config.testnet.yaml`. `BOT_DB_NAME=tradingbot_research_stage_one`.
Secret hanya melalui `deploy/vps-set-env.py` via stdin SSH; jangan mencetak
`.env.vps`. Tool itu menerima `TB_*`, `BOT_IMAGE` dan `BOT_DB_NAME`.

## Inspeksi bila diminta

Dari `/opt/trading-bot-paper/repo` di VPS; perintah berikut tidak memulai bot:

```sh
dc() { docker compose --env-file .env.vps -f deploy/compose.vps.yml "$@" </dev/null; }
dc ps --all
docker inspect -f 'status={{.State.Status}} exit={{.State.ExitCode}} started={{.State.StartedAt}}' trading-bot-paper-bot-1
python3 deploy/vps-baseline-check.py
systemctl is-active trading-bot-paper-backup.timer
```

`vps-observe.py` memeriksa proses/feed trial riset aktif; bot sengaja mati akan
memberi masalah kesehatan. Timer lama diaktifkan kembali untuk trial baru ini.
`/healthz` hanya membuktikan DB, bukan kesehatan feed atau strategi.

Port host loopback: web 18080, metrics 19100, Prometheus 19090, Grafana 13000.
Tunnel dashboard/monitoring saat layanan terkait aktif:

```sh
ssh -N -L 18080:127.0.0.1:18080 -L 13000:127.0.0.1:13000 plesk
```

DNS bot memakai CoreDNS/DoT Cloudflare pada subnet `172.31.253.0/29` karena DNS
host pernah memberi halaman blokir Binance. Jangan mengubah resolver host.
Compose dalam skrip SSH harus memakai `</dev/null`/`--interactive=false` agar
stdin skrip tidak tertelan. Izin hook/auto-mode Claude tidak otomatis berlaku
untuk Codex atau agent lain; periksa sandbox yang aktif.

## Riwayat yang tetap disimpan

- Testnet 18–20 Oktober berakhir normal setelah ±43 jam, tujuh order dibatalkan
  sesudah recorder stop; DB historis masih `new`, exchange final nol open order.
  Inventori dipertahankan. [Analisis](testing/testnet-stop/analysis.md).
- Rollback testnet historis: commit `f709429`, digest
  `sha256:e5b8710bc76da2cd5d581511e19fb7bf9e6a472e0eb61b14fb3c099054fd0438`.
  Ini referensi, bukan instruksi start. Jangan down migrasi `tradingbot_eth`.
- Riwayat operasi awal: [handover awal](testing/vps-handoff.json) dan
  [operasi/restore](testing/vps-ops.json). Prosedur rilis, migrasi dan rollback
  terpusat di [deployment](deployment.md).
