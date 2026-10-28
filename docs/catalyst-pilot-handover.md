# Handover pilot katalis

**Dijeda 28 Oktober 2026 sekitar 22.12 WIB, dilanjutkan user 22.18 WIB.**
Putaran lanjutan (bagian [Lanjutan 28 Oktober malam](#lanjutan-28-oktober-malam))
menutup cakupan artikel sumber, sumber SOL, bind listener dan regex CI native.
Masih belum ada commit/push/deploy WIP. Mulai dari dokumen ini, lalu [Plan 3](plan-3.md),
[runtime katalis](catalyst-runtime.md) dan [handover VPS](handover-vps.md).

## Keputusan user dan batas pekerjaan

- User sebelumnya mengizinkan **commit, push dan deploy**. Setelah rilis
  komponen/report, user meminta penyelesaian alur P3 dan berharap selesai malam
  ini. Permintaan pause terbaru menghentikan pekerjaan itu; target malam ini
  belum tercapai sebagai pilot yang aktif.
- Pilihan user melalui pertanyaan terstruktur: **paper 1.000 USDT virtual**,
  risiko **0,5% per trade**, maksimal **2 posisi**, halt rugi harian **2%** /
  drawdown **8%**. Parameter nominal/exposure development lain tetap dirujuk
  pada `DevelopmentEntryPolicy`/`PaperSpec`; bukan izin uang asli.
- **Maksimal 12 panggilan AI per hari**, output tiap panggilan maksimal
  **2.048 token**. Implementasi kuota menggunakan pergantian hari **WIB**;
  risk ledger lama tetap berbatas hari UTC. Tarif uang gateway belum diverifikasi.
  Kuota ini membatasi request aplikasi pada satu arsip, bukan biaya rupiah atau
  inference tambahan yang mungkin dilakukan gateway di balik satu request.
- Akses gateway/key sudah tersedia pada `.env.catalyst` lokal privat; jangan
  menampilkan isinya. Model tetap `cx/gpt-6-luna-low`; belum ada evaluasi nyata
  prompt v5 atau provisioning credential pilot ke VPS.
- P2 yang berjalan, modal, database, parameter dan T0 tetap dipertahankan.
  Bot/trial tidak dijalankan di laptop; pengujian lokal di bawah adalah fixture
  dan database sementara, bukan trial harga pasar nyata.
- Sumber/universe awal ETH/SOL/DOGE adalah profil development terkonfigurasi,
  bukan pembatasan permanen universe riset. Presale/token baru, futures dan
  venue lain tetap mengikuti penundaan Plan 3 sebelumnya.

## Git dan deployment yang benar-benar ada

HEAD lokal/origin terakhir: **`56c2a5c`**, branch `main`. Setelah itu banyak
perubahan kode **modified/untracked**, belum di-commit, di-push atau dibangun
menjadi image rilis baru. Baca `git status --short`; `git diff` biasa tidak
menampilkan isi file untracked. Jangan reset, clean, menghapus atau menimpa WIP.

Rilis VPS yang sudah terbukti masih **source `2998f38`**, project
`trading-bot-catalyst`, `/opt/trading-bot-catalyst`, image
`ghcr.io/rizz404/trading-bot-catalyst@sha256:cee992d6f6cd908569e072d98d07831d04b2efab26f006bf74e2a3e2997ce79d`.
Itu pemasangan komponen dan `report` sekali jalan, **bukan worker/pilot**.
[Bukti rilis](testing/catalyst-release.json) dan CI di sana tetap historis;
kelulusannya tidak mencakup WIP baru. Tidak ada DB/wallet/modal/T0 pilot P3
yang dibuat di VPS, tidak ada worker/scheduler P3 yang diaktifkan.

## Pengambilan sumber nyata sesudah rilis

Satu perintah terbatas dijalankan di VPS pada image rilis lama, dengan arsip
P3 sendiri dan tanpa model/order:

```sh
cd /opt/trading-bot-catalyst/repo
docker compose --env-file .env.catalyst-image -f deploy/compose.catalyst.yml run --rm --no-deps catalyst -action collect -archive /var/lib/catalyst </dev/null
```

Hasil sekitar **21.10 WIB**, command berakhir **exit 1** karena dua sumber gagal:

| Sumber | Hasil |
|---|---|
| Ethereum `https://blog.ethereum.org/feed.xml` | `feed exceeds document limit` |
| Solana `https://solana.com/news` | `robots check failed` |
| Dogecoin `https://foundation.dogecoin.com/blog/` | Berhasil; evidence `905f3da049e1dc2b78176eb030b945cf95255a1d45ceeb7ab67ede53306152ad`, first_seen `2026-10-28T14:10:39.766581855Z` |

Observasi/hasil tersimpan di volume arsip P3. Tidak ada fallback HTTP diam-diam,
inference atau pengiriman order. One-off container memakai `--rm` dan sudah selesai.
Ini bukan klaim semua situs atau seluruh artikel sudah tercakup.

WIP mengganti Ethereum ke **`https://blog.ethereum.org/en`**, tetapi URL baru
**belum diuji dengan collector di VPS**. Penelusuran web membaca homepage dan
artikel resmi; homepage menunjuk artikel dengan path seperti
`/2026/10/22/glamsterdam-hoodi-delay` (empat segmen). Spider masih `--depth 2`;
ini kemungkinan membatasi penemuan artikel lengkap dan harus diperiksa.
Solana belum diperbaiki; jangan mengabaikan robots atau mengganti engine hanya
agar status terlihat sukses. WIP memisahkan gap sumber per aset agar sumber
sehat tetap dapat dianalisis, tanpa mengizinkan entry aset yang buktinya kurang.

## Peta kode WIP

| Area/file | Yang sudah ditulis; belum rilis |
|---|---|
| `internal/exchange/binance/quote.go` | Public Spot book ticker: bid/ask, kuantitas, validasi simbol/harga dan timestamp konservatif sebelum request. Qty nol tetap diterima untuk observasi; entry/fill ditahan menurut depth |
| `internal/catalyst/market.go`, `market_test.go` | Reuse client Binance dan indikator ATR14, candle 15m tertutup/berurutan, penolakan gap/stale/OHLC salah; cache metadata/volume 5 menit, ATR per batas 15m; book selalu baru. Reuse exclusion `bStocks`/fallback lama |
| `internal/catalyst/events.go`, `events_test.go` | Kandidat jadwal UTC eksplisit pada dokumen primer dengan satu mapping aset; cue dan penolakan ambigu/cancel/TBA. Registry arsip mempertahankan alias dokumen/koreksi waktu dan menyamakan artikel dengan aset/waktu sama; konflik menahan verifikasi |
| `internal/catalyst/budget.go` | 12 slot O_EXCL per hari WIB, sebelum I/O, fsync, lintas proses/restart pada arsip yang sama. Timeout/error tetap memakai slot; bukan retry/failover otomatis |
| `internal/catalyst/analyst.go` | Prompt **catalyst-v5**, `AnalyzeMarket`, konteks quote/posisi/event/gap/consumed events; cek aksi sesuai posisi, kandidat primer dan event terpakai. Marker tidak berubah hanya karena harga bergerak atau tesis diparafrasekan |
| `internal/catalyst/archive.go` | Kuota default **2 GiB**, flock lintas writer, cek headroom filesystem dan ukuran; idempotent Put tetap bisa membaca record yang sudah ada. Tidak menghapus bukti otomatis; kuota penuh menahan riset. Memakai syscall Unix/Linux, bukan dukungan Windows baru |
| `internal/catalyst/collector.go` | `AvailableEvidence` menampilkan sumber sehat + gap; checksum/I/O rusak tetap fatal. `LatestEvidence` mempertahankan kontrak strict lama. Raw pertama dipertahankan bila konten sama |
| `internal/catalyst/spider.go` | Identity konten memakai teks/dokumen hasil ekstraksi, bukan header HTTP/HTML inert yang berubah. Extraction tag `documents-v1/content-v2/spider-2.53.9-p3.1`; cache lama bisa memerlukan interval polling baru |
| `internal/domain/domain.go` | `BookQuote` sebagai observasi harga, bukan candle sintetis |
| `internal/broker/sim.go`, `book_test.go` | `MatchBook`: buy di ask/sell di bid, fee/slippage, maksimal 10% best-level depth bersama, lot rounding, partial fill dan conditional yang tetap terpicu sesudah partial/restart. Jalur candle lama tetap ada |
| `internal/managed/runtime.go` | `RecordBook` dengan guard harga/umur, timestamp mark aktual dan settlement Sim per event; reuse ledger/jurnal/checkpoint. Perubahan ini menyentuh shared code: wajib regression sebelum rilis, jangan deploy ulang P2 |
| `internal/catalyst/pilot.go` | Draft runner: loop riset, feed book dan timer terpisah; lease, snapshot ledger, event consumed, fresh quote setelah AI, preflight risiko, input atomik, expiry/proteksi managed, health file dan audit outcome. HOLD mempertahankan tipe/waktu event serta tesis/invalidation asli |
| `internal/catalyst/http.go` | Reuse dashboard/control managed; `/research`, `/research.json` dengan auth, `/healthz` membedakan runtime/feed dari kesiapan riset. Belum QA browser |
| `cmd/catalyst/main.go`, `pilot.go`, `pilot_test.go` | Draft aksi `run` paper-only, `market`, `health`; env P3, role `catalyst` + DB prefix `tradingbot_catalyst_`, migrasi/lease/managed.Init/NewPaper, password UI minimal 16 karakter, SIGTERM, sender Telegram tanpa poller |
| `internal/notify/telegram/telegram.go`, `telegram_test.go` | `RunSender` tanpa polling/update menu; test baru ditulis, belum dieksekusi pada putaran terakhir |
| `internal/catalyst/pilot_test.go`, `pilot_integration_test.go` | Fixture pipeline source→market→model mock→paper→HOLD→partial exit→restore; varian PostgreSQL/lease, auth/health |
| `internal/catalyst/spider_binary_test.go` | Native binary test tambahan memastikan proxy yang menolak tidak dibypass; lulus pada container network-none |
| `configs/catalyst.policy.json` | Versi lokal `catalyst-paper-v1`: Ethereum homepage, Anza blog (pengganti solana.com), Dogecoin, masing-masing dengan `article_pattern`. Tidak memperbarui policy dalam image VPS lama |
| `deploy/compose.catalyst.pilot.yml` | Overlay **belum divalidasi/dijalankan**: DB/volume sendiri, DoT CoreDNS sendiri, subnet kandidat `172.31.252.0/29`, UI loopback `18081`, runtime 0,5 CPU/512 MiB, DB 0,5 CPU/1 GiB, DNS 0,25 CPU/128 MiB |
| `Dockerfile.catalyst` | COPY migrations untuk bootstrap PG; **image WIP belum dibangun** |
| `.github/workflows/ci.yml` | Draft PG lifecycle step + buildx/cache khusus katalis. **Belum dipush/run**; regex native test masih hanya `^TestSpiderBinaryProtocol$`, belum mencakup test denial baru |
| `.env.catalyst.example` | Tambahan placeholder DB/dashboard/Telegram. Tidak diisi credential asli |

## Bukti pengujian dan batasnya

Lihat [laporan lokal pilot](testing/catalyst-pilot-local.md). Beberapa checkpoint
unit/race/lint/PG berhasil, tetapi **tidak ada putaran lengkap untuk seluruh
checkout final** setelah edit terakhir. Jangan mengatakan seluruh WIP lulus,
CI sudah menerima worker atau integrasi source/model nyata sudah selesai.

Container PostgreSQL lokal sementara `catalyst-verify-20261028` dipakai hanya
untuk fixture, DB `tradingbot_catalyst_test`, loopback port sementara `37209`.
Tes PG lulus. Container dihentikan saat handover dengan `podman stop`; karena
dibuat `--rm`, data/volume test ephemeral tidak dipertahankan. **Ini bukan DB
pilot atau data P2.** Tidak ada proses go/podman/gh tugas yang masih berjalan
pada inspeksi pause. Tidak menjalankan test/build/server baru untuk handover.

## Temuan/pekerjaan berikutnya yang belum ditutup

1. **Review source contract dan cakupan artikel.** Ethereum homepage belum
   dicrawl dengan perubahan baru; depth 2 mungkin tidak mencapai artikel
   bertanggal. Pertimbangan terakhir adalah pemilihan link artikel + scrape
   bounded melalui Spider, tetapi **belum ada kode/flag ArticlePattern atau
   implementasi crawl per artikel**. Jangan mengira ide itu sudah diterapkan.
2. **Verifier event masih konservatif dan sempit.** Parser UTC belum mendukung
   semua format nyata (misalnya long-month dengan detik); perlu fixture dari
   sumber nyata untuk konteks negasi, mainnet/testnet, jadwal lama dibatalkan,
   alias lintas artikel/koreksi dan waktu publikasi versus event. Cue teks dan
   mapping publisher bukan bukti dampak ekonomi. Jangan memberi Verified dari
   `review.Valid`, memperluas grammar hanya agar trade terjadi, atau mengklaim
   event identity sudah universal. Artikel hilang bukan pembatalan otomatis.
3. **Verifikasi akhir kode/runner.** Jalankan tes terkait/race/vet/lint/build
   dan PG terisolasi setelah review. Tambahkan bukti model lambat/mati tidak
   menghalangi feed/timer, storage/lease loss, expiry dan shutdown worker nyata.
   Test helper memulihkan runtime/ledger, belum membuktikan restart proses
   worker penuh dengan posisi aktif. Audit price freshness, risiko/sleeve,
   pending/cancel/partial/dust, dan tidak menjual inventori binding lain.
4. **Bootstrap/UI/Telegram.** Saat ini worker dan HTTP listener dimulai
   bersamaan; kegagalan bind bisa terjadi sesudah worker mulai. Pertimbangkan
   bind sebelum start. Uji `/research`, auth, health degraded/stale, form scope,
   escaping dan sender-only. Tidak ada inference nyata v5/pesan P3 yang dikirim.
5. **Operasi/Compose.** Validasi overlay dengan env placeholder tanpa mencetak
   secret; periksa subnet dan port VPS kosong, quote Binance/DoT nyata,
   ownership volume dan batas aktual. Provision env P3 secara aman; jangan
   menyalin seluruh env P2 atau mencetak key. Backup/restore DB/arsip P3 dan
   observasi/alert belum dipasang. Off-host backup tetap mengikuti keputusan lama.
6. **Rilis baru setelah siap dan user melanjutkan.** Commit per scope, push,
   seluruh CI/GHCR/digest; jangan menyalin aplikasi/image laptop ke VPS. Tetapkan
   baseline project lain, jalankan bounded source/market/model nyata dengan
   kuota 12, lalu start paper terpisah hanya sesudah penerimaan. Catat modal,
   hash policy, source/digest, DB/wallet, T0 aktual, recovery dan review hari
   1/3/7. Target selesai malam ini bukan kelulusan otomatis atau izin mainnet.

## Lanjutan 28 Oktober malam

- **Depth Spider dihitung dari segmen path, bukan hop link.** Uji nyata lokal:
  `--depth 2` dari `https://blog.ethereum.org/en` hanya mengembalikan halaman
  kategori/locale; artikel `/YYYY/MM/DD/slug` baru muncul pada depth ≥4 dan
  urutan crawl konkuren tidak menjamin artikel terbaru. `--blacklist-url`
  (tanpa fitur regex) dan `--budget` per path tidak menyelesaikannya.
- **`article_pattern` per sumber** (`Source.ArticlePattern`, divalidasi sebagai
  regexp ≤200 karakter). Bila diisi, collector men-scrape seed `--limit 1`,
  memilih link HTTPS same-host yang path-nya cocok dalam urutan halaman seed
  (index terkonfigurasi menaruh posting terbaru di atas), lalu men-scrape
  maksimal 4 artikel satu per satu dengan jeda 1 detik. Batas output 8 MiB
  berlaku untuk seluruh run satu fetch. Tanpa pattern, perilaku crawl lama tetap.
  Seed tanpa link cocok gagal sebagai coverage tidak lengkap.
- **SOL: `solana.com/news` diganti `https://www.anza.xyz/blog`** atas pilihan
  user. Penyebab kegagalan lama bukan robots disallow: `solana.com/robots.txt`
  menjawab HTTP 429 `x-vercel-mitigated: challenge` (bot challenge Vercel) dan
  tidak di-bypass. `github.com/anza-xyz/agave/releases.atom` ditolak karena
  robots GitHub melarang `/*.atom$`. Source id baru `anza-blog`.
- **Policy lokal** kini memakai pattern untuk ketiga sumber; hash policy berubah
  dan versi tetap `catalyst-paper-v1` karena belum pernah dirilis.
- Uji collect nyata lokal (Spider p3.1 disalin dari image lokal, arsip
  sementara di scratchpad, tanpa model/order/DB): ketiga sumber sukses, masing
  masing seed + 4 artikel terbaru. Record arsip: Ethereum ~1,84 MB, Anza
  ~2,66 MB, Dogecoin ~0,28 MB (batas 8 MiB). Ini bukan uji dari jaringan/DNS/
  proxy VPS. Posting Dogecoin terbaru bertanggal 2025-04-10: sumber DOGE nyaris
  tidak aktif, sehingga katalis DOGE kemungkinan jarang.
- `Document.published_text` kosong pada ketiga situs; waktu publikasi artikel
  belum diekstrak (relevan untuk temuan 2).
- `cmd/catalyst/pilot.go` kini `net.Listen` sebelum worker dimulai, sehingga port
  sibuk menggagalkan startup tanpa worker yatim (menutup sebagian temuan 4).
- Regex native CI menjadi `^TestSpiderBinary`, mencakup denial test proxy.
- Checkout akhir: `go build ./...`, `go vet ./...`, `go test -race -count=1 ./...`
  dan `golangci-lint run ./...` lulus (0 issues), `git diff --check` bersih.
  Test PG integration, native container, image, Compose dan CI tidak dijalankan
  ulang pada putaran ini.

Putaran berikutnya (28 Oktober malam, setelah user meminta lanjut sampai selesai):

- **Tanggal publikasi** kini diekstrak dari `meta itemprop` (Dogecoin), JSON-LD
  `datePublished` (Ethereum) dan `<time datetime>` pertama sebagai fallback
  (Anza). `ExtractionVersion` naik ke `documents-v2`; cache lama tidak dipakai.
- **Verifier event**: format `October 26, 2026, 17:42:48 UTC`, `... at HH:MM:SS UTC`
  dan `YYYY-MM-DD HH:MM:SS UTC` didukung; tanggal dekat `testnet/devnet/hoodi/
  sepolia/holesky` dilewati (bukan dampak Spot); negasi `will not/won't/no
  longer` membuat dokumen ambigu. Diuji dengan kalimat nyata Ethereum Foundation.
  Evidence nyata saat ini menghasilkan **0 kandidat** (Hoodi testnet/ditunda):
  pilot tidak akan entry sampai ada jadwal mainnet/produk eksplisit.
- **Worker penuh**: test `Pilot.Run` membuktikan model yang macet tidak menghentikan
  timer/feed (health tetap sehat) dan shutdown bersih; lease hilang menghentikan
  worker dengan error ownership. Fixture pilot dipisah ke `newPilotFixture`.
- **UI**: test escaping halaman `/research` (judul/URL/alasan tidak tepercaya).
  QA browser nyata belum dilakukan karena Chrome DevTools MCP gagal terhubung.
- **VPS read-only**: Docker Compose v5.6.1, subnet `172.31.252.0/29` dan port
  `127.0.0.1:18081` kosong, P2 resolver memakai `172.31.253.0/29`.
- Checkout akhir: `go vet -tags integration ./...`, `go test -race ./...`,
  `golangci-lint` 0 issues. PG integration lokal tidak diulang (CI menjalankannya).
- Kode di-commit lokal per scope `6e98705`..`2831c9e` (**belum di-push**). Commit
  docs dan push ditahan permission mode Claude Code; menunggu keputusan user.

Masih terbuka: push + CI/GHCR digest, provisioning `.env.catalyst` pilot di VPS,
deploy overlay, run bounded nyata, start paper, review hari 1/3/7.

## Referensi yang dibaca

- [Market data Binance](https://developers.binance.com/en/docs/catalog/core-trading-spot-trading/api/rest-api/market): book ticker/depth dan data publik; adapter memakai endpoint existing client.
- [Ethereum homepage](https://blog.ethereum.org/en) dan
  [pengumuman penundaan Hoodi](https://blog.ethereum.org/2026/10/22/glamsterdam-hoodi-delay): contoh nyata tanggal lama yang dibatalkan/testnet, bukan sinyal beli.
- [Solana news](https://solana.com/news): halaman terbaca di penelusuran web,
  tetapi collector VPS mendapat kegagalan robots. Keberhasilan web tool tidak
  membuktikan Spider/robots VPS berhasil.
- Lampiran user di Downloads sudah dibaca. Ketentuan lampiran lama exit H-1
  jam/hold 48 jam tetap digantikan arah Plan 3: hold sesuai tesis, stop monoton
  dan deadline review; jangan mengembalikan aturan lama itu.
