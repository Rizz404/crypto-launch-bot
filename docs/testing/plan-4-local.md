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

## Analyze nyata pertama dan pilihan model — 14 November 2026

`.env.launch` diisi user (key memakai nama `TB_CATALYST_API_KEY`; `loadAnalyst`
memakai itu sebagai cadangan `TB_LAUNCH_API_KEY`). User meminta model selain
`cx/gpt-6-luna-low`, tier terendah. Arsip: `%LOCALAPPDATA%\crypto-launch-bot\lv\research`.

Smoke `json_schema` strict (1 request per model, di luar kuota aplikasi):
`cx/gpt-6-luna-low`, `cx/gpt-6-luna-medium`, `cx/gpt-6.1-sol-low`,
`cx/gpt-6.1-sol-medium`, `cc/claude-sonnet-5-5-medium`,
`claude/claude-sonnet-5-5-medium`, `openrouter/anthropic/claude-sonnet-5.5-medium`
dan `openrouter/google/gemini-3.1-pro-preview` lulus (finish `stop`, JSON
sesuai schema, 2–6 detik). `cxa/gpt-6-sol-medium` HTTP 503. Endpoint admin
`/api/*` menjawab 403 untuk key ini; hanya `/v1/models` dan `/v1/combos` terbaca.

Lalu `collect -source curve -limit 10` dan `analyze` satu kandidat curve
(`held`: pajak 2%/3%, 1 holder) dengan tiga model, memakai 3 dari 12 slot hari itu:

| Model | Hasil | Waktu | Token in/out |
|---|---|---:|---:|
| `cx/gpt-6-luna-low` | **Ditolak validator**: `asset_key` ditulis `eip155:56/0x…` (garis miring), bukan `eip155:56:0x…` | 11,7 s | 13.945 / 526 |
| `cx/gpt-6.1-sol-medium` | Valid, `WATCH` | 22,2 s | 13.945 / 642 |
| `cc/claude-sonnet-5-5-medium` | Valid, `WATCH` | 8,9 s | 16.798 / 929 |

Validator bekerja seperti dirancang: identitas yang salah satu karakter pun
ditolak dan slot tetap terpakai. Sol menandai bahwa `curve_round_trip_cost_fraction`
0,023 tampak di bawah batas 0,05 padahal alasan `curve_round_trip_cost_above_policy`.
Selisih itu berasal dari pajak yang ditambahkan policy, yang tidak dijelaskan
di packet. Model melaporkannya sebagai inkonsistensi, tidak mengabaikannya.

Satu kasus per model bukan evaluasi. Penerimaan pilot tetap butuh minimal
sepuluh kasus berlabel per jalur ([metode](../presale-analysis.md#metode-simulasi-dan-evaluasi-v1)).

Atas permintaan user, agent membuat combo OmniRoute **`launch-research`**
(strategi `priority`, `maxRetries` 1, urutan manual) memakai management key
yang user isikan di `.env.launch`:

1. `cc/claude-sonnet-5-5-medium`
2. `cx/gpt-6.1-sol-medium`
3. `openrouter/google/gemini-3.1-pro-preview`
4. `openrouter/anthropic/claude-sonnet-5.5-medium`

Luna-low (ditolak validator), `cxa/gpt-6-sol-medium` (HTTP 503) dan
`claude/claude-sonnet-5-5` (parent-nya `cc/claude-sonnet-5-5`, jadi tidak
menambah jalur) tidak dimasukkan. Dua smoke `json_schema` dan satu `analyze`
lewat nama combo lulus, dilayani target pertama: `valid`, `REJECT`, 9,5 s,
10.108/957 token. Fallback ke target 2–4 belum terbukti jalan; target 3 dan 4
baru lulus smoke, belum analyze nyata. Fallback hanya untuk error layanan,
bukan untuk jawaban yang ditolak validator.

## Policy exit v3 dan data curve dari chain — 14 November 2026

User menyetujui batas exit usulan agent dan memilih data holder curve dari
chain (opsi 2), karena GoPlus mengosongkan pajak/holder token four.meme yang
belum listing di DEX. Policy menjadi `launch-paper-v3`; aturannya di
[kontrak P4-3](../presale-analysis.md#policy-yang-dibekukan). Alasan
`holder_liquidity_and_exit_policy_pending` kini hanya untuk presale.

Temuan yang membentuk implementasi:

- 29 dari 29 token four.meme terbaru adalah TaxToken9 (creator type 9) dengan
  pajak 2–3% per sisi. Pajak dibaca dari `feeRateBuy()`/`feeRateSell()` token
  (persen), atau `feeRate()` untuk tipe 5 (basis point). Tipe 0 terbukti tanpa
  pajak dari transaksi beli nyata: `msg.value` = cost + fee protokol. Tipe lain
  memberi gap `curve_creator_type_unrecognised`.
- Estimasi jual Helper3 untuk token pajak 2–3% hanya 0,09% di bawah token
  tanpa pajak. Jadi pajak ditambahkan terpisah ke biaya curve, tidak dianggap
  sudah termasuk.
- Supply awal dipegang TokenManager2 sebagai inventori kurva, sehingga holder
  dihitung dari replay log `Transfer` sejak blok pembuatan tanpa manager.
- Gas swap dari receipt nyata: buy ±210 ribu, sell ±193 ribu, harga 0,05 gwei.
  Bolak-balik dihitung 460 ribu gas (termasuk approve) × `eth_gasPrice` × ask
  BNBUSDT Binance, sekitar 0,014 USDT.
- `cannot_sell_all` GoPlus kosong sebelum listing DEX. Untuk curve,
  estimasi jual Helper3 atas jumlah yang dibeli dipakai sebagai cek exit.
- DEX: konsentrasi dari daftar top-10 GoPlus, tanpa pool, dead dan holder
  terkunci. Slot yang dibuang diisi share terkecil yang terlihat, supaya holder
  ke-11 yang tersembunyi tidak membuat hasil tampak lebih aman.

| Pemeriksaan | Hasil |
|---|---|
| `go vet`, `go test` paket `launch` dan `cmd/launch`, `golangci-lint` | Lulus; test baru `TestExitPolicyHoldsUnknownsAndLimits`, `TestCurveTaxByCreatorType`, `TestGoPlusHoldersSkipPoolLockedAndDeadButKeepHiddenTail` |
| `collect -source curve -limit 10` lalu `dex -limit 10` di Windows (arsip sementara) | Exit 0 keduanya. Curve: semua `held`; token pajak tertahan `curve_round_trip_cost_above_policy` (2,3% kurva + 4–6% pajak), token tanpa pajak hanya oleh `holder_count_below_policy`. DEX: semua `held`, umumnya pajak/likuiditas unknown dan holder terkonsentrasi |

Belum: quote curve sebagai `PaperRouteQuote`. Journal paper sengaja hanya
menerima quote berawalan `fixture:`. Menyambungkan quote nyata adalah langkah
paper prospektif (P4-5) yang mengubah batas keamanan itu, jadi menunggu
keputusan. `holder_count` GoPlus untuk pool DEX yang baru bisa 0 walau ada
likuiditas; hasilnya ditahan, tidak lolos.

## Dukungan Windows — 14 November 2026

Atas permintaan user, test, CLI dan skrip pendukung dibuat lintas platform. Penyebab
gagal sebelumnya: `syscall.Flock`/`Statfs` tidak ada di Windows. Kini
`internal/catalyst/fs_unix.go` dan `fs_windows.go` (`golang.org/x/sys/windows`,
dependency yang sudah ada) menyediakan `LockFile`, `UnlockFile`, `SyncDir` dan
ruang disk. `SyncDir` di Windows hanya memeriksa direktori karena handle
direktori tidak bisa di-fsync; isi file tetap di-fsync. Perilaku Linux sama.

Perubahan lintas platform lain pada hari yang sama:

- Test Spider tidak lagi memakai skrip `#!/bin/sh`. Binary palsu adalah salinan
  test binary itu sendiri (`TestMain` di `spider_test.go`), jadi keempat test
  subprocess jalan di semua OS tanpa skip.
- `cmd/portfolio-backtest` memakai `os.DevNull`, bukan `/dev/null`.
- Default cache `/tmp/crypto-launch-bot-year` pada skrip backtest Go/Python
  kini `<temp OS>/crypto-launch-bot-year`. Di Linux hasilnya tetap `/tmp/...`.
- `scripts/backtest-queue`: `fcntl.flock` diganti `lock_exclusive` (msvcrt di
  Windows). `launch` tetap memakai `systemd-run`, jadi khusus Linux.
- `deploy/vps-observe.py`: `os.getloadavg` opsional. Test backup
  `catalyst-backup.sh` di-skip di Windows karena script itu khusus VPS Linux.
- `Makefile`: asset Tailwind dipilih per OS/arch dengan checksum dari
  `sha256sums.txt` rilis 4.3.3, dan binary memakai suffix `$(go env GOEXE)`.
  Di Windows butuh GNU make dan shell POSIX (Git Bash/MSYS2).
- `.gitattributes` `eol=lf`: script bash dan `gofmt` di CI butuh LF. Checkout
  Windows dengan `core.autocrlf=true` sebelumnya menghasilkan CRLF.
- CI: job `windows` (`go vet`, `go test`, test Python).

| Pemeriksaan | Hasil |
|---|---|
| Windows: `go build ./...`, `go vet ./...`, `go test ./...` | Lulus semua paket, test Spider subprocess tidak di-skip |
| Windows: unittest `deploy` dan `scripts/backtest-queue` | Lulus (1 skip: backup bash) |
| Windows: lock `lock_exclusive` kedua pada file yang sama | Ditolak, sesuai harapan |
| Cross: `GOOS=linux go vet ./...`, `GOOS=darwin go vet ./...` | Lulus |
| Linux: test binary `catalyst`, `launch`, `cmd/launch`, `cmd/portfolio-backtest` (dikompilasi `GOOS=linux`, dijalankan di WSL Fedora 45) | Lulus |
| Linux: unittest `scripts/backtest-queue` | Lulus |

Belum diverifikasi: `Makefile` (tidak ada `make` di Windows maupun WSL lokal),
job CI Windows (belum di-push), race test, test integrasi DB, dan test backup
bash di Linux (WSL Fedora 45 lokal tidak punya `awk`; runner Ubuntu CI punya).

## Pass ketiga, analyze dan policy v2 — 13 November 2026

Dijalankan sesudah user berkata "lanjut terus, pilih yang best practice, tanya
hanya jika benar-benar crucial". Tanpa wallet, signer atau transaksi.

| Pemeriksaan | Hasil |
|---|---|
| `collect` dex, cex, curve dengan `-limit 10` (batas flag 1–10), lalu presale | Exit 0 semua. DEX 168 detik, CEX 42 detik, curve 97 detik |
| `report` sesudahnya | DEX 20 `held`, CEX 20 `rejected`, curve 19 `held`, presale 7 `rejected`; tidak ada `accepted_for_research`; arsip 18 MB |
| `analyze` pada 2 kandidat (fair launch `0xdbcc…4fd9`, pool DEX `0x7555…b509`) dengan `-env-file .env.catalyst` | Gagal keduanya: `inference transport failed`, 0 token, 2 dari 12 slot hari itu terpakai |
| `go build ./...`, `go vet`, `go test ./...`, `go test -race` paket launch sesudah policy v2 | Lulus |
| `replay-paper` fixture dengan policy v2 | Exit 0, 18 langkah, kas 1008.961, fees 0.539 (sama seperti v1) |

Penyebab `analyze` gagal: `TB_CATALYST_BASE_URL` di `.env.catalyst` perangkat
kantor masih host lama `route-gateway.fbe.co.id`, yang sertifikat TLS-nya tidak
cocok. Host baru `route-gateway.nexlabs.cloud` menjawab 401 tanpa key, jadi
hidup. Agent tidak mengubah env privat, dan percobaan menimpa host lewat
variabel proses ditolak pengaman izin Claude Code. Yang harus dilakukan user:
ganti host di `.env.catalyst` atau buat `.env.launch`. Dukungan schema/usage
gateway dan kualitas model tetap belum teruji.

Perubahan kode yang diuji di atas:

- `launch-paper-v2`: `curve_fraction` 0,01; angka lain sama dengan v1.
- `launch-analyst-v2`: `PROPOSE_ENTRY` boleh untuk token curve berstatus
  `bonding_curve`; prompt menyebut curve dan fair launch.
- Paper: kind `curve` (venue `four-meme`) memakai jalur quote/broadcast/receipt
  yang sama dengan DEX; entry ditolak bila token sudah `graduated`.
- Verifikasi: alasan `curve_exit_and_paper_adapter_pending` dihapus. Curve
  tetap `held` oleh `holder_liquidity_and_exit_policy_pending`, sama seperti DEX.

Test baru: `TestCurveTradesLikeAnOnChainSwapOnlyWhileOnTheCurve`,
`TestCurveEntryProposalNeedsBondingCurveState`; `curve` masuk ke
`TestPaperSizeIncludesCostsAndNeverEnablesEntry`.

Sesudah itu discovery curve ditambah estimasi bolak-balik Helper3 (`tryBuy`
0,01 BNB lalu `trySell`; selector `0xe21b103a`/`0xc6f43e8c` diturunkan dari
[interface resmi](https://github.com/four-meme-community/fourmeme-docs) lewat
`web3_sha3` dan dicoba pada token nyata). Test launch lulus lagi, lalu
`collect -source curve -limit 10`: exit 0, kesepuluh token terbaru memberi
`curve_round_trip_cost_fraction` 0,0235 (fee 1% per sisi ditambah selisih
kurva, belum termasuk gas), tanpa gap quote. Arsip uji mencatatnya sebagai
fakta; quote ini belum disambungkan ke adapter paper.

Belum ada: quote kurva sebagai `PaperRouteQuote` (butuh kurs BNB/USDT dan gas),
langkah curve di fixture contoh, dan policy holder/likuiditas/exit yang membuat DEX,
presale dan curve bisa keluar dari `held`. `liquidity_usd` dari GeckoTerminal
kosong untuk pool yang baru dibuat, dan 0 dari DexScreener.

## Pass hari kedua — 13 November 2026

Dijalankan atas permintaan user ("lanjutkan yang belum") pada 01:08 UTC ke arsip
yang sama (`~/.local/state/crypto-launch-bot/lv/research`), sesudah `sale_type`
ditambahkan sebagai fakta kandidat (`go vet` dan `go test -race` paket
`internal/launch`/`cmd/launch` lulus). Tanpa inference, wallet atau transaksi.

| Jalur | Hasil |
|---|---|
| Presale | Exit 0; 7 pool yang sama, semua `sale_closed`; `sale_type` terisi (6 `presale`, 1 `fairlaunch`). Tidak ada sale upcoming/live |
| DEX | Exit 0; cursor maju, 5 pool baru (total 10, semua `held`); 2 dari 5 terisi GeckoTerminal; muncul `factory_backlog_pending` |
| CEX | Exit 0; rotasi maju ke 5 simbol berikutnya (total 10, semua ditolak `listing_older_than_30_days`) |
| Curve | Exit 0; 5 token baru (total 10, semua `held`) |
| `report` | Exit 0; first-seen pass pertama dipertahankan; `entry_allowed` false untuk semua; arsip 12 MB, 9 batch |

Yang terbukti: cursor DEX, rotasi CEX dan first-seen bertahan antar-pass pada
arsip yang sama. Yang belum: tiga pass dalam ±10 jam bukan coverage beberapa
hari. Dua kanal presale tidak berubah isinya antara dua hari itu, jadi jalur
presale belum pernah diuji dengan sale yang masih terbuka. `factory_backlog_pending`
menunjukkan `-limit 5` lebih lambat daripada laju pool baru PancakeSwap; laju
dan limit yang memadai belum diukur. Rotasi CEX lima simbol per pass butuh
ratusan pass untuk satu putaran ±1.300 simbol USDT.

### Pass kedua dan implementasi fair launch (12 November, malam)

Dijalankan atas permintaan user ("lanjutkan berdasarkan docs") di perangkat
`fujiyama-office-windows`, WSL Fedora. `/tmp/lv` hilang setelah WSL restart, jadi
binary dan arsip uji kini di `~/.local/state/crypto-launch-bot/lv` (di luar
repo, bertahan antar-restart) supaya pass berulang menumpuk di satu arsip.

| Pemeriksaan | Hasil |
|---|---|
| `collect` presale, dex, cex, curve (`-limit 5`) lalu `report` | Exit 0 semua; `entry_allowed` false untuk seluruh review; arsip 7 MB setelah 5 batch |
| Presale | 7 pool yang sama dengan pass sebelumnya, semua `sale_closed`; tidak ada sale upcoming/live |
| DEX | 5 pool baru, semua `held`; 1 terisi GeckoTerminal, 4 `market_metadata_unavailable` |
| CEX | 5 simbol yang sama, ditolak `listing_older_than_30_days` (rotasi simbol tidak maju antar-arsip baru) |
| Curve | 5 token four.meme terbaru, semua `held` |
| `go vet`, `go test -race ./internal/launch ./cmd/launch` sesudah perubahan | Lulus |

Pool `0xdbcc27573b47afcb616a4fa1457da2a492344fd9` selesai diselidiki lewat
`bsc-dataseed` (baca saja):

- Kodenya clone EIP-1167 dari `0x07f6351491b694f3a04c5f1e47d151d4ad927d45`,
  `factory()` mengembalikan factory PinkSale yang sama, slot proxy EIP-1967 kosong.
- Halaman PinkSale pool itu bertipe fair launch (soft cap saja, tanpa hard cap
  dan rate). Pool kedua di kanal, `0x006c2baa…f4ab`, juga fair launch dan memakai
  implementasi yang sama. Sembilan pool BSC di arsip terbagi 6/1/2 pada tiga
  implementasi, semuanya dengan factory yang sama.
- Implementasi ketiga berbagi 36 dari 43 selector dengan dua lainnya (dua
  implementasi presale berbagi 43). Ketiganya tidak terverifikasi di Sourcify.
- Keputusan agent: `0x07f6…7d45` ditambahkan ke daftar implementasi dikenal
  dengan standar bukti yang sama seperti dua sebelumnya (pengamatan chain atas
  pool yang disajikan pinksale.finance). Pass ulang: ketujuh kandidat
  `known_pinksale_clone`.

Cara mengulang pemeriksaan kontrak sale (JSON-RPC ke
`https://bsc-dataseed.binance.org`, jeda ±1 detik antar-request):
`eth_getCode` pool harus berbentuk
`0x363d3d373d3d3d363d73<implementasi>5af43d82803e903d91602b57fd5bf3`, dan
`eth_call` dengan data `0xc45a0155` (`factory()`) harus mengembalikan
`0x1ee7736987f2ebd0f519ec5858b2b3bc6fd0c6a0`. Perintah pass ada di
[panduan CLI](../presale-runtime.md#pass-berulang).

Gap baru: tipe sale baru menjadi fakta `sale_type` pada 13 November dan belum
dipakai analisis/paper, padahal fair launch tidak punya harga tetap; `factory()` dilaporkan sendiri oleh
kontrak, dan bukti bahwa factory yang men-deploy pool (transaksi pembuatan)
belum diambil. Coverage tetap dua pass pada hari yang sama, bukan beberapa hari.

### Sumber tambahan (12 November, lanjutan)

Atas keputusan user sesudah riset sumber: verifikasi clone PinkSale, kanal
tidak resmi sebagai petunjuk kedua (ditandai), GeckoTerminal sebagai metadata
pool, dan four.meme sebagai venue keempat. Build, vet dan race test paket
`catalyst`/`launch`/`cmd/launch` lulus, lalu satu pass nyata per jalur.

| Perubahan | Sumber kebenaran | Hasil pass nyata |
|---|---|---|
| Kontrak sale harus clone EIP-1167 dari implementasi PinkSale yang dikenal dan `factory()` cocok (`inspectSale`) | Dibaca dari 6 pool BSC; PinkSale tidak mempublikasikan alamat kontrak | 6 dari 7 pool `known_pinksale_clone`; 1 ditandai `sale_contract_not_known_pinksale_clone` |
| `t.me/s/PinkSaleTracking` sebagai petunjuk kedua, fakta `lead_official: 0` | Tidak tercantum di tautan resmi PinkSale | 7 kandidat presale (3 kanal resmi, 4 kanal tidak resmi), semua `sale_closed` |
| GeckoTerminal `networks/bsc/pools/<pool>` bila DexScreener kosong | API publik tanpa key; batas rate dan syarat pakai belum diverifikasi | 1 pool terisi DexScreener, 4 tetap `market_metadata_unavailable` (GeckoTerminal belum mengindeks) |
| Kind `curve`, venue `four-meme`: log `TokenCreate` TokenManager2 lalu konfirmasi `getTokenInfo` Helper3 | [Panduan integrasi dan ABI resmi](https://github.com/four-meme-community/fourmeme-docs) | 5 token terbaru terbaca (versi 2, manager cocok, dana kurva), semua `held` |

Catatan dan gap:

- `bsc-dataseed` menolak `eth_getLogs`, jadi log four.meme diambil dari
  `bsc-rpc.publicnode.com` (host baru di allowlist) sebagai petunjuk; tiap token
  dikonfirmasi lewat Helper3 di `bsc-dataseed`. Jendela 2.000 blok, hanya
  `-limit` token terbaru yang diperiksa (`bounded_curve_tail_not_complete`).
- `curve` pada 12 November baru sebatas discovery; sejak 13 November policy,
  analisis dan paper mengenalnya (lihat bagian policy v2 di atas).
- Pool yang ditandai bukan clone dikenal (`0xdbcc…4fd9`) ternyata fair launch
  dengan implementasi ketiga; lihat bagian pass kedua di atas.
- Enumerasi pool PinkSale lewat event factory tidak berhasil: tidak ada log dari
  alamat `factory()` pada 360 ribu blok terakhir.
- Kedua kanal belum memunculkan sale yang masih upcoming/live pada pass ini.

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
