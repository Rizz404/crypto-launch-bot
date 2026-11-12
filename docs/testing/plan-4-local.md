# Checkpoint lokal Plan 4

**29 Oktober 2026.** Riset P4-1, implementasi development P4-2 dan analisis/policy
P4-3 serta paper sintetis P4-4. Commit discovery **`108b3fa`**; perubahan
P4-3/P4-4 disertakan pada checkpoint commit berikutnya atas permintaan user
(lihat `git log` untuk hash terbaru). Tidak ada
uji fungsi, build, crawl CLI, koneksi RPC/API melalui aplikasi, inference,
Telegram, database, server, deploy atau transaksi yang dijalankan.

## Verifikasi 12 November 2026

Dijalankan atas permintaan user di WSL Fedora (`GOTOOLCHAIN=go1.27.2`; Go
bawaan WSL 1.26) setelah rename ke `crypto-launch-bot` dan cherry-pick
`680a704`/`85def1c` dari upstream. Arsip uji di `/tmp/lv`. Tanpa inference,
wallet, signer atau transaksi.

| Pemeriksaan | Hasil |
|---|---|
| `go build ./...`, `go vet ./...`, `go test ./...` | Lulus, 29 paket |
| Test integrasi seperti job CI (storage, siklus migrasi, sesi paper, managed, pilot catalyst; `-race`, DB sementara `cryptolaunchbot_ci_test`) | Lulus |
| `promtool test rules alerts.test.yml` | Lulus |
| Build Spider pinned dari `Dockerfile.spider` (podman lokal) | `spider_cli 2.53.9-p3.1` |
| `collect -source presale` lalu `report` | Exit 0; 3 kandidat presale terbaca, semua `rejected`, `entry_allowed` false |

Sumber seed presale diputuskan user mengikuti saran agent: kanal Telegram
sebagai petunjuk tautan, halaman detail PinkSale dan chain sebagai bukti.
Pemeriksaan tautan resmi mengubah pilihan kanalnya:

- `t.me/PinkSaleTracking` **tidak** tercantum di
  [Social Links](https://docs.pinksale.finance/important/social-links) PinkSale,
  jadi tidak boleh disebut resmi. Yang tercantum hanya grup `t.me/pinkecosystem`
  dan kanal pengumuman `t.me/pinkecosystem_ann`.
- Seed yang dipakai: `https://t.me/s/pinkecosystem_ann`. Spider membaca satu
  halaman kanal, mengambil maksimal lima tautan `/launchpad/bsc/0x…` terbaru,
  lalu men-scrape halaman detailnya (`catalyst.CollectDiscoveryLinked`). Robots
  `t.me` dan Cloudflare PinkSale meloloskan agent `ana-scout-research`.
- Hasil pass nyata: tiga pool unik, semuanya sale Oktober yang sudah `ended`
  atau `canceled`, sehingga ditolak `sale_closed`. Token address, jadwal,
  status, kode kontrak sale dan data GoPlus terbaca untuk ketiganya.

Gap yang tersisa pada jalur presale:

- Kanal pengumuman hanya memuat sale yang dipromosikan dan pada pass ini tidak
  memuat satu pun sale yang masih upcoming/live. Jalur terbukti berfungsi,
  tetapi coverage untuk sale baru belum memadai (gap batch
  `lead_channel_lists_promoted_sales_only`). Opsi lanjutan menunggu keputusan
  user: event factory PinkSale lewat RPC, atau `PinkSaleTracking` sebagai
  petunjuk tambahan yang tidak resmi.
- Binary Spider di atas hasil build lokal; image `crypto-launch-bot-spider`
  belum di-publish dan pin digest di `Dockerfile.catalyst` masih nama lama.

## Verifikasi 10 November 2026

Dijalankan atas permintaan user di WSL Fedora (go1.27.2) setelah merge
trading-bot (`c48b653`). Arsip uji di `/tmp/lv` WSL, di luar repo; `var/launch`
tidak disentuh. Tanpa inference berbayar, wallet, signer atau transaksi.

| Pemeriksaan | Hasil |
|---|---|
| `go build ./...`, `go vet` launch/catalyst | Lulus |
| `go test ./...` | 29 paket lulus, sebelum dan sesudah perbaikan |
| `go test -race ./internal/catalyst ./internal/launch ./cmd/launch` | Lulus |
| `replay-paper` fixture tiga jalur | Exit 0, 18 langkah, kas 1008.961 USDT; replay kedua identik (idempotent) |
| `collect -source cex -limit 5` (Binance publik) | Awalnya gagal total, lalu diperbaiki; 5 simbol ditolak `listing_older_than_30_days` |
| `collect -source dex -limit 5` (BSC RPC, GoPlus, DexScreener) | Awalnya 4/5 pool gagal baca token, lalu diperbaiki; 5 pool terbaca, semua held |
| `report` | Exit 0; `entry_allowed` false untuk semua review |

Bug ditemukan dan diperbaiki (`c2a0863`, dengan test regresi yang terbukti
gagal tanpa perbaikan):

- **exchangeInfo melebihi batas respons.** Daftar SPOT tanpa filter sekarang
  ±6,7 MB (3.702 simbol, 2.330 `BREAK`), di atas `maxBody` 4 MB, sehingga
  jalur CEX selalu `exchange_info_unavailable`. Kini memakai
  `symbolStatus=TRADING` (±2,5 MB, 1.372 simbol).
- **Simbol lama ditahan, bukan ditolak.** Bila trade ID 0 sudah tidak disimpan
  Binance, trade tertua yang masih ada dicatat sebagai fakta
  `oldest_retained_trade_at`. Jika lebih tua dari 30 hari, kandidat ditolak;
  waktu listing tetap tidak diklaim.
- **State BSC terpangkas di tengah pass.** Node publik `bsc-dataseed`
  membalas `missing trie node` untuk blok ±60–120 detik di belakang finalized,
  lebih singkat dari satu pass yang dibatasi rate 2,1 detik/request. Snapshot
  finalized kini diperbarui bila berumur lebih dari 15 detik sebelum tiap pool
  atau sale; bukti refresh ikut di `EvidenceIDs`.

Test regresi baru: `TestCEXOldRetainedTradeRejectsWithoutClaimingListingTime`,
`TestDEXRefreshesPrunedSnapshotBeforePoolReads`, dan pemeriksaan filter
`symbolStatus=TRADING` pada `TestCEXHistoryRequiresTradeZeroAndKeepsVenueIdentity`.

Data yang teramati pada pass nyata:

- CEX: `0GUSDT`, `1000CATUSDT`, `1000CHEEMSUSDT` punya trade ID 0
  (Desember 2024 sampai September 2025). Trade tertua yang tersimpan untuk
  `1000SATSUSDT` dan `1INCHUSDT` dari November 2024. Kelimanya ditolak.
- DEX: lima pool terbaru registry PancakeSwap V2 terbaca token0/token1 dan
  data GoPlus. Satu token punya beberapa field GoPlus yang tidak lolos
  (proxy, mintable, pausable, slippage modifiable, owner change balance,
  source tidak terverifikasi, honeypot/blacklist unknown).
  DexScreener belum mengenal pool-pool tersebut.

Cara mengulang (Go hanya tersedia di WSL; pakai arsip sementara):

```sh
export GOTOOLCHAIN=auto GOFLAGS=-buildvcs=false
go build -o /tmp/lv/launch ./cmd/launch
/tmp/lv/launch -action replay-paper -fixture configs/launch.paper.fixture.json -archive /tmp/lv/research -paper-archive /tmp/lv/paper
/tmp/lv/launch -action collect -source cex -limit 5 -archive /tmp/lv/research
/tmp/lv/launch -action collect -source dex -limit 5 -archive /tmp/lv/research
/tmp/lv/launch -action report -archive /tmp/lv/research
```

Temuan yang belum ditutup:

- **Discovery presale tidak bisa menghasilkan kandidat.** `/launchpads`
  PinkSale dirender JavaScript: HTML server tidak memuat satu pun tautan
  `/launchpad/bsc/0x…`, dan sitemap di robots.txt membalas 404. Halaman detail
  `/launchpad/bsc/<pool>` dirender server, dan parser yang ada cocok dengan
  token address, status serta `Start time`/`End time … (UTC)`. Robots
  mengizinkan agent `ana-scout-research`; user agent generik seperti curl
  diblokir Cloudflare (403). Atas keputusan user, sumber daftar/seed
  ditunda dan dicatat sebagai gap. Opsi yang ditemukan: kanal Telegram
  `t.me/s/PinkSaleTracking` (dirender server, 9 tautan BSC saat dicek; pada
  12 November terbukti tidak tercantum sebagai kanal resmi) atau
  `api.pinksale.finance/api/v1/pool/list` (berfungsi tetapi tidak
  terdokumentasi, bertentangan dengan kontrak integrasi).
- Binary Spider pinned belum di-build secara lokal, jadi crawl presale melalui
  aplikasi belum dijalankan. Tanpa seed yang benar hasilnya tetap 0 kandidat.
- DexScreener membalas `pairs:null` untuk kelima pool baru, sehingga semuanya
  held `market_metadata_unavailable`. Ini sesuai desain (unknown bukan aman),
  tetapi jalur DEX baru bisa diterima bila metadata pool yang matang ikut diuji.
- `analyze` (inference) tidak dijalankan.

## Yang tersedia (29 Oktober)

- Riset referensi resmi dan matriks pilihan:
  [integrasi awal](../presale-integrations.md).
- CLI discovery/report terpisah dengan reuse archive serta pipeline Spider.
- Kandidat presale/pool/pair terpisah, identitas chain/kontrak atau venue,
  receipt time/revisi, bukti asli dan gap sumber.
- Verifier menahan data basi/unknown/konflik dan tidak pernah mengizinkan entry.
- Test source ditulis untuk batas 30 hari, identitas, jadwal presale/testnet,
  source gap, book, arsip rusak, trade ID 0, cursor/backlog pool, RPC error dan
  batas trust scanner.
- P4-3: schema/prompt, validator identitas/kutipan/lifecycle, policy dan helper
  sizing sesuai profil pilihan user, kuota durable WIB dan CLI analyze.
- Test source P4-3 mencakup override policy/size, missing/duplicate fields,
  quote palsu, expiry, consumed ID, locked exit/claim, HOLD/thesis, biaya/halt,
  dedup, kuota/error dan transport dengan schema kustom. Belum dijalankan.

## Pemeriksaan yang dilakukan (29 Oktober)

P4-4 menambahkan journal/adapters dan fixture data 18 langkah tiga jalur.
Data contoh dibentuk dengan serialisasi JSON/hash statis; bukan hasil eksekusi
simulator. Test source mencakup cash/cost/fee conservation, partial depth,
restart/idempotency, alokasi/vesting/refund, quote/min-out, timeout/expiry,
receipt gas gagal, finality/reorg dan journal fork. Semuanya belum dijalankan.

Review statis akhir P4-4: format Go dirapikan, diff whitespace bersih,
fixture JSON berlabel sintetis terbaca, 12 dokumen Markdown berubah/baru
tanpa tautan lokal/anchor putus. Payload bukti pengujian lama tidak berubah.

Review statis alur/callers dan format Go dengan gofmt; bukan kompilasi atau
kelulusan tes. Checkpoint P4-2 sebelumnya mencatat sepuluh dokumen dengan
tautan lokal/anchor valid. Lanjutan P4-3: diff whitespace bersih, JSON policy
terbaca dan sebelas dokumen Markdown berubah/baru tanpa tautan lokal atau
anchor putus. Dokumen dan
payload hasil Plan 1–3 tidak digunakan ulang sebagai bukti
strategi baru. Tidak ada perubahan remote Git atau VPS pada fase ini.

## Belum diverifikasi (status 10 November)

- Sumber seed daftar presale dan crawl PinkSale melalui binary Spider pinned
  (lihat temuan di atas).
- Reorg/downtime/rate limit jangka panjang RPC, akses geografis Binance dari
  host sasaran, latensi/coverage/backfill dan kapasitas/retensi arsip pada
  pass berulang. Satu pass bounded per jalur bukan bukti coverage.
- Kualitas model untuk prompt baru, dukungan schema/usage gateway, kuota dan
  penolakan adversarial pada inference nyata.
- API/receipt sumber nyata untuk paper P4-4, bridge AI→paper serta
  kontrol/pilot Plan 4. Yang sudah dijalankan hanya fixture sintetis.
- Wrapper transport gateway/kuota yang dipakai ulang lulus unit test repo,
  tetapi belum diuji terhadap gateway nyata.

Acceptance runtime P4-2–P4-4 tetap terbuka sebagian: build/test, replay paper
dan pass nyata CEX/DEX sudah ada buktinya; presale, AI dan sumber nyata paper
belum. Keberadaan kode bukan kelulusan gate.
