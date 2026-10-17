# Handoff: Bot Trading Crypto (Go)

> Dokumen ini adalah sumber kebenaran untuk perencanaan. Agent/developer yang melanjutkan: baca seluruh file ini sebelum menulis kode.

## Status saat ini (2026-10-17)

- **Fase 0–8 selesai** (2026-10-17). Fase 2–8 dibangun tanpa test; semua jalur trading belum pernah dijalankan. `bot run` merekam candle live Binance (spot/futures) ke TimescaleDB dengan backfill gap otomatis, dan `datafetch` mengunduh data historis. Engine trading (strategi, risk, order) belum ada.
- Seluruh keputusan di tabel "Keputusan yang sudah diambil" di bawah sudah disetujui user lewat tanya-jawab. Jangan ditanyakan ulang kecuali ada alasan teknis kuat.
- **Langkah berikutnya: Fase 9 (Testing)** — kerjakan seluruh "Backlog testing" di bawah. Setelah itu Fase 10 (Fixing). Fase 2–8 dibangun tanpa test (sesuai keputusan user).
- **Mode kerja sejak 2026-10-17 (keputusan user): bangun dulu, test belakangan.** Fase 2–8 fokus membangun fitur. Testing untuk semua fitur dilakukan di Fase 9, lalu semua temuannya diperbaiki di Fase 10. Lihat "Aturan fase build" di bawah.
- Cara menjalankan: lihat [README.md](../README.md).
- Tandai fase yang selesai di bagian "Progres" di bawah setiap kali satu fase beres.

## Aturan kerja dari user

- Mesin dev project ini adalah **CachyOS (bukan NixOS)**. Go 1.27.2 dipasang lewat mise (`.mise.toml`), container memakai **podman** + podman-compose. Tidak ada `nix`, jadi `flake.nix` tidak dipakai.
- Jangan menjalankan perintah build/rebuild NixOS yang mengaktifkan konfigurasi (aturan global user).
- Commit: Conventional Commits satu baris tanpa body, `type(scope): deskripsi` (contoh: `feat(binance): add futures kline stream`). Satu commit per scope, diurutkan sesuai alur pengerjaan. Tanpa `Co-Authored-By`. Aturan lengkap ada di `~/.claude/CLAUDE.md`.
- Jangan pernah menaruh API key/secret di repo. Secret hanya lewat env/secret file.
- Jangan menjalankan order ke mainnet sebelum user secara eksplisit menyetujui Fase 11 (live).

## Aturan fase build (Fase 2–8)

- Fokus menulis fitur. **Jangan** menulis unit test atau integration test baru, dan jangan melakukan uji end-to-end (paper run, order testnet, probe mainnet). Semua itu dikerjakan di Fase 9.
- Pemeriksaan yang tetap wajib sebelum commit (cepat, bukan testing): `go build ./...`, `go vet ./...`, dan `golangci-lint run` bersih. Test lama dari Fase 0–1 harus tetap ter-compile (`go vet` mencakup file test).
- Setiap hal yang perlu diuji, asumsi yang belum diverifikasi, atau perilaku API yang belum pasti dicatat di bagian **"Backlog testing"** di bawah, supaya tidak terlewat di Fase 9.
- Test yang sudah ada dari Fase 0–1 dipertahankan; tidak perlu dihapus.

## Progres

- [x] Fase 0: Fondasi (2026-10-17)
- [x] Fase 1: Domain + Binance market data (2026-10-17)
- [x] Fase 2: Indikator + Strategy + Backtester (2026-10-17, belum dites)
- [x] Fase 3: Risk + OMS + Paper trading (2026-10-17, belum dites)
- [x] Fase 4: Binance LiveBroker (testnet/mainnet) (2026-10-17, belum dites)
- [x] Fase 5: Telegram + Prometheus/Grafana (2026-10-17, belum dites)
- [x] Fase 6: Web dashboard (2026-10-17, belum dites)
- [x] Fase 7: Bybit + OKX (2026-10-17, belum dites)
- [x] Fase 8: Strategi lanjutan (grid/DCA, arbitrase, market making) (2026-10-17, belum dites)
- [ ] Fase 9: Testing (semua fitur)
- [ ] Fase 10: Fixing (semua temuan Fase 9)
- [ ] Fase 11: Live mini + VPS

## Backlog testing (diisi selama Fase 2–8, dikerjakan di Fase 9)

- Uji adapter Binance terhadap mainnet setelah DNS lokal diperbaiki (lihat catatan jaringan di bawah).
- Universe spot: putuskan dan uji pengecualian saham ter-tokenisasi (lihat pertanyaan terbuka).
- **Fase 2**
  - `indicator`: nilai EMA/SMA/RSI/ATR/MACD/Bollinger dibandingkan referensi (TA-Lib/TradingView); seed EMA dari SMA; RSI saat loss = 0.
  - `portfolio.Account`: average entry, partial close, flip posisi, fee pro-rata ke trade, funding (long bayar saat rate positif), `Snapshot` (equity, used margin).
  - `broker.Sim`: aturan fill (market di open bar berikutnya, gap di stop/TP, limit sebagai maker, urutan stop sebelum TP), order hanya cocok dengan candle yang open ≥ waktu order, slippage, fee.
  - `engine`: entry → SL/TP terpasang setelah fill; satu leg kena → leg lain dibatalkan; reversal (exit lalu entry); trailing stop bergeser hanya ke arah untung; stop yang sudah terlewati → exit market; stop ditolak → posisi ditutup; antrean event broker (tanpa deadlock saat broker memanggil handler secara sinkron); `CloseAll`; `SetPaused`.
  - `risk.Size`: sizing berbasis jarak stop, fallback `position_pct`, cap `max_position_pct` dan free margin, pembulatan lot, tolak di bawah minimum.
  - `backtest`: urutan event (TF eksekusi sebelum TF strategi pada close time yang sama), funding tiap 8 jam, metrik (CAGR, MDD, Sharpe/Sortino, profit factor ∞ → `null` di JSON), split in-sample/out-of-sample, CSV, `-save` ke DB.
  - Migrasi `00002` up/down; `config` men-decode `strategies[].params`, map `backtest.fees` di-merge dengan default.
  - Golden test `emacross` dan eksperimen parameter default.
- **Fase 3**
  - `risk.CheckOrder`: `max_open_positions` (hanya untuk entry simbol baru), `max_exposure_pct`; kill switch `Observe`: rugi harian sejak 00:00 UTC, drawdown dari puncak, trip sekali, `Reset`/`Halt`; `close_on_kill` menutup semua posisi dan membatalkan order entry/strategi.
  - Engine: candle basi (close lebih dari 1 bar yang lalu) tetap masuk ke strategi tapi intent-nya dibuang; `AdoptOrders` membangun ulang proteksi dari order SL/TP; `Unprotected`.
  - Recorder: urutan tulis (order sebelum fill-nya), antrean penuh, flush saat shutdown; `UpsertOrder` memindahkan `run_id` order yang diadopsi ke sesi aktif.
  - Paper: `bot run` mode paper dengan strategi aktif. Cek warmup dari DB (300 bar), fill dari tick harga live, resume sesi (`paper.resume`: saldo, posisi, order terbuka), snapshot equity tiap menit, `FinishRun` saat shutdown. Uji restart di tengah posisi.
  - Migrasi `00003` up/down.
- **Fase 4** (semua di testnet dulu; banyak perilaku API yang hanya diketahui dari dokumentasi/connector, belum pernah dicoba)
  - Signature HMAC REST (query string) dan WS API (param diurutkan alfabetis) — cek terhadap contoh di dokumentasi (`openssl dgst`).
  - Spot user data: `userDataStream.subscribe.signature` di `wss://ws-api.binance.com:443/ws-api/v3` (testnet `ws-api.testnet.binance.vision`); respons subscribe; event `executionReport` dengan semua key kembar; resubscribe setelah reconnect.
  - Futures user data: `listenKey` + `wss://fstream.binance.com/private/stream?streams=<key>`. **URL private testnet belum pasti** (`fstream.binancefuture.com/stream`?). Keepalive 30 menit, `listenKeyExpired` → `Kick`.
  - Futures SL/TP lewat **Algo Order** (`POST /fapi/v1/algoOrder`, `algoType=CONDITIONAL`, `triggerPrice`, `workingType=MARK_PRICE`, `priceProtect=true`). Belum pasti: format `ai` (string/angka), apakah order hasil trigger membawa `clientOrderId` sendiri, urutan `ALGO_UPDATE` vs `ORDER_TRADE_UPDATE` (fill ditahan ≤10 detik menunggu mapping), dan status `FINISHED` tanpa eksekusi.
  - Spot OCO `POST /api/v3/orderList/oco` (above TAKE_PROFIT / below STOP_LOSS untuk SELL); cancel satu leg membatalkan list (cancel leg kedua → -2011, diabaikan).
  - `SetLeverage` (isolated, -4046/-4048), `EnsureOneWay` (-4059), sinkronisasi jam dan -1021.
  - Reconciliation: posisi futures dari `positionRisk` v3, saldo dari `/fapi/v3/balance` (`balance` = wallet), spot dari saldo base asset; koreksi hanya setelah 2 kali selisih berturut-turut; order `tb*` diadopsi dengan tag dari DB; `SyncOrders` memulihkan fill yang terlewat (fee 0).
  - Fee spot dalam base asset (qty posisi < qty fill) dan fee BNB (tercatat 0) → dikoreksi lewat reconciliation; pastikan tidak ada drift.
  - Guard: mode `live` ditolak tanpa `app.allow_live: true`; mode `testnet` ditolak jika exchange tidak `testnet: true`.
- **Fase 5**
  - Telegram: long polling, whitelist `chat_ids` (chat lain diabaikan), escape HTML, pesan > 4096 karakter dipotong, antrean penuh, flush pesan "Bot stopped" saat shutdown, token tidak pernah muncul di log/error.
  - Perintah: `/status`, `/positions`, `/balance`, `/pnl`, `/orders`, `/strategies`, `/pause`, `/resume` (ditolak saat kill switch aktif), `/killswitch confirm`, `/reset confirm`, `/stop confirm`, `/help`.
  - Notifikasi: entry terisi, trade ditutup (PnL), notice engine ≥ `notify_level`, ringkasan harian 00:00 UTC.
  - Metrics trading (`tradingbot_*`) terisi dengan benar; dashboard Grafana termuat (panel Prometheus + tabel TimescaleDB); alert Prometheus menyala sesuai kondisi (promtool hanya mengecek sintaks).
- **Fase 6**
  - Dashboard di browser (desktop + HP, light/dark): overview, partial auto-refresh, tombol pause/resume/kill/reset (konfirmasi), chart candle dengan marker trade (posisi marker di bar yang benar), chart equity, daftar dan detail backtest.
  - Keamanan: basic auth saat `TB_WEB__PASSWORD` di-set; validasi menolak listen non-localhost tanpa password; aksi `POST` tanpa header `HX-Request` → 403; header CSP/X-Frame-Options; CSP tidak memblokir htmx/lightweight-charts.
  - Mode tanpa sesi (hanya market data): halaman tetap jalan, tanpa panel live.
- **Fase 7** (format publik sudah dilihat 2026-10-17; semua jalur private belum pernah dijalankan)
  - Contract test bersama untuk ketiga adapter (`exchange.MarketData` + `broker.Live`) dengan fixture terekam.
  - Bybit: kline REST dibalik ke oldest-first, `confirm` di WS, subscribe maksimal 10 arg per request (spot), ping 20 detik; sign REST `ts+key+recvWindow+query|body`; auth WS `GET/realtime`; `triggerDirection` stop vs TP; spot `orderFilter=StopOrder` (cancel juga butuh `orderFilter`); `marketUnit=baseCoin`; fee spot buy dalam base; status `Deactivated`/`PartiallyFilledCanceled`; set margin mode isolated untuk UTA (account-wide!); kode error 110043/110025/110001/170213.
  - OKX: **konversi kontrak swap** (qty, minSz, lotSz, posisi, fill, volume) di setiap jalur; paginasi `history-candles` dengan `before`/`after`; bar UTC (`6Hutc`, `1Dutc`, …); `volCcy24h` swap × harga; demo trading (`x-simulated-trading: 1`, `wspap.okx.com`); login WS menunggu event `login`; pemetaan `orders-algo.ordId` → `algoClOrdId`; fee negatif = biaya; `tgtCcy=base_ccy` untuk market spot; error per item (`sCode`); posisi net mode; akun harus mode "spot and futures".
  - ID order engine sekarang alfanumerik (`tb` + hash 7 karakter + sequence) — pastikan diterima ketiga exchange dan tidak bentrok.
  - Wallet unified (Bybit/OKX): sesi menolak dua market di exchange yang sama.
- **Fase 8**
  - `grid`: level aritmatika/geometris, buy awal di bawah harga, order balasan setelah fill penuh (pemetaan harga ke level setelah pembulatan tick), restart tidak menggandakan order (cek `OpenOrders`), `stop_below` membatalkan semua dan exit.
  - `dca`: jadwal beli (`interval`, `dip_pct`, `max_buys`), TP/SL dari harga rata-rata, siklus baru setelah exit, timer berjalan di backtest (`eng.Tick`).
  - `funding`: data funding (Binance `premiumIndex`, Bybit tickers; OKX belum ada sumber) di-poll tiap 5 menit; buka/tutup dua kaki; kaki timpang dibongkar setelah grace; tidak bisa di-backtest (tidak ada histori funding).
  - `xspread`: arah posisi benar untuk spread positif/negatif, cooldown saat order dalam perjalanan, butuh dua broker live di mode testnet/live.
  - `mm`: requote (CancelAll + ref baru) hanya saat harga bergerak/fill/tanpa order, skew inventori, batas inventori, spot tidak menjual tanpa inventori. Backtest market making memakai fill limit `Sim` yang terlalu optimistis (tanpa antrean order book).
  - Legs: routing candle per instrumen di engine, akun dan subscription untuk semua leg, cek wallet unified ikut menghitung leg, backtest multi-akun (equity total, `InitialBalance` per akun).
  - Risk `max_exposure_pct` menghitung kedua kaki strategi hedged sebagai exposure; sesuaikan batas atau ubah perhitungan.

## Pertanyaan terbuka (belum diputuskan)

- Timeframe dan parameter default strategi `emacross` (tentukan lewat eksperimen backtest di Fase 9).
- Saham ter-tokenisasi di Binance spot (base berakhiran `B`: `NVDAB`, `TSLAB`, `SPYB`, `SNDKB`, `SPCXB`, dan lain-lain) lolos ke universe spot karena tidak punya penanda di `exchangeInfo`. Perlu keputusan user: dikecualikan (lewat `universe.exclude_bases` atau aturan baru) atau dibiarkan. Universe futures tidak terpengaruh, karena kontrak `TRADIFI_PERPETUAL` sudah disaring.
- Batas risk konkret untuk live (max drawdown %, rugi harian %, leverage maksimum). Usulkan nilai konservatif, lalu minta persetujuan user.

## Catatan implementasi (penyimpangan dari plan rinci di bawah)

Bagian ini lebih baru daripada plan rinci. Kalau ada konflik, ikuti bagian ini.

- Module Go: `github.com/rizz404/trading-bot`.
- `flake.nix` diganti `.mise.toml` (lihat aturan kerja di atas).
- Migrasi memakai **goose sebagai library**. File SQL di [migrations/](../migrations/) di-embed ke binary, lalu dijalankan dengan `bot migrate up|down|status`. CLI goose tidak diperlukan. `bot run` menolak start kalau masih ada migrasi pending.
- **sqlc ditunda** sampai Fase 1, ketika query pertama dibutuhkan. Pertimbangkan pgx biasa kalau query-nya sedikit.
- Config: defaults → YAML (`configs/config.yaml`, di-gitignore) → `.env` → environment proses. Key env: prefix `TB_`, pemisah nesting `__` (contoh: `TB_EXCHANGES__BINANCE__API_KEY`). `api_key`/`api_secret`/`passphrase` **ditolak** kalau ada di YAML. Mode `testnet` mewajibkan `testnet: true` di setiap exchange yang aktif, dan mode `live` mewajibkan `testnet: false`. Lihat [internal/config/config.go](../internal/config/config.go).
- Tipe `config.Secret` tidak pernah tercetak (`***`), baik lewat fmt maupun slog. Pakai `.Reveal()` hanya ketika nilai aslinya benar-benar dibutuhkan.
- Metrics: `/metrics` dan `/healthz` (ping DB) di `127.0.0.1:9100`. Prometheus di compose men-scrape `bot:9100` (bot di container, `--profile bot`) dan `host.containers.internal:9100` (bot di host). Di rootless podman, target host hanya `up` kalau bot listen `0.0.0.0` (`TB_METRICS__LISTEN=0.0.0.0:9100`), karena `host.containers.internal` tidak mengarah ke loopback host.
- Versi image yang di-pin: `timescale/timescaledb:2.30.2-pg17`, `prom/prometheus:v3.15.0`, `grafana/grafana:13.2.4`, build `golang:1.27.2-alpine`, runtime `distroless/static-debian12:nonroot`.
- **Jaringan dev (2026-10-17):** DNS lokal membelokkan semua domain Binance/Bybit/OKX ke `43.173.57.48` (diblokir). Koneksi lewat IP asli tembus, dan testnet Binance tidak diblokir. User memilih memperbaiki DNS sistem sendiri, jadi bot **tidak** punya resolver DoH bawaan. Sebelum DNS diperbaiki, uji mainnet hanya bisa lewat `curl --resolve` atau `binance.Options.HTTPClient` dengan dialer khusus.
- Grafana memprovisikan datasource `Prometheus` (uid `prometheus`) dan `TimescaleDB` (uid `timescaledb`). Dashboard JSON diletakkan di `deploy/grafana/dashboards/`.

### Fase 1 (market data)

- Interface dipecah: `exchange.MarketData` (publik: instruments, tickers, klines, stream) sekarang, dan interface order/akun terpisah di Fase 4. Lihat [internal/exchange/exchange.go](../internal/exchange/exchange.go).
- Endpoint Binance ada di [internal/exchange/binance/client.go](../internal/exchange/binance/client.go). **WS futures mainnet = `wss://fstream.binance.com/market/stream`**: URL lama `/stream` masih menerima koneksi, tapi tidak mengirim data. Testnet futures: REST `testnet.binancefuture.com`, WS `fstream.binancefuture.com/stream`.
- **Jebakan JSON:** `encoding/json` mencocokkan key tanpa membedakan huruf besar-kecil, sedangkan Binance memakai key kembar (`l`/`L`, `v`/`V`, `t`/`T`, `q`/`Q`, `e`/`E`). Setiap struct untuk pesan Binance wajib mendeklarasikan field untuk key huruf besar juga, supaya key itu tidak menimpa field huruf kecil secara diam-diam. Contohnya `klineMsg` di [stream.go](../internal/exchange/binance/stream.go).
- Data testnet disimpan dengan nama exchange `binance-testnet` (dari `MarketData.Name()`), jadi tidak tercampur dengan mainnet. `bot run` memakai data testnet hanya di mode `testnet`. Mode `paper` memakai data mainnet.
- `exchange.WSStream`: reconnect dengan backoff dan jitter, reconnect saat idle 60 detik (koneksi diam), dan reconnect proaktif sebelum 24 jam. Setelah setiap connect, stream mengirim `EventConnected`, lalu `marketdata.Feed` mem-backfill gap lewat REST sebelum memproses event berikutnya. `Feed.OnCandle` menerima setiap candle closed tepat sekali per seri, urut waktu. Ini titik sambung untuk engine strategi di Fase 3.
- Rate limit memakai `exchange.WeightLimiter` per market (budget 4800/menit spot, 1900/menit futures, di bawah limit resmi). Respons 429/418 men-pause limiter sesuai `Retry-After`.
- Universe (`marketdata.SelectUniverse`): quote cocok, status TRADING, base tidak ada di `exclude_bases`, volume quote 24 jam ≥ minimum, spread ≤ maksimum, diurutkan berdasarkan volume, lalu diambil top-N. Futures hanya `PERPETUAL`.
- Tidak ada agregasi timeframe di Go: setiap timeframe di-subscribe atau di-download langsung dari exchange. Itu lebih sederhana dan hasilnya identik dengan bar exchange. Agregasi 1m → timeframe besar untuk fill intrabar di backtest bisa ditambahkan di Fase 2 kalau dibutuhkan.
- sqlc tidak dipakai. Query cukup sedikit, jadi ditulis dengan pgx langsung di [internal/storage/marketdata.go](../internal/storage/marketdata.go), dengan decimal ↔ numeric lewat `pgx-shopspring-decimal`.
- Integration test DB: `make test-integration` (butuh `make up`), file `*_integration_test.go` dengan build tag `integration`.
- Batasan yang diketahui: kalau backfill satu seri gagal lalu candle live seri itu masuk, gap sebelumnya tidak diisi ulang otomatis. Isi lewat `datafetch` (aman diulang).

### Fase 2 (strategi dan backtest)

- **OMS ada di package `engine`**, bukan package `oms` terpisah ([internal/engine/orders.go](../internal/engine/orders.go)). Engine meneruskan candle ke strategi, men-sizing intent lewat `risk`, memasang SL/TP setelah entry fill, membatalkan leg lain saat satu leg kena, menggeser trailing stop saat candle close, dan menangani reversal.
- Event broker diantrekan, lalu diproses oleh pemegang lock engine (`unlockAndDrain`). Ini perlu karena broker (misalnya `Sim.CancelOrder`) boleh memanggil handler secara sinkron saat engine memegang lock.
- Strategi mengembalikan `strategy.Intent`: `Enter`/`Exit` untuk strategi arah, dan `Place`/`Cancel`/`CancelAll` untuk order mentah (grid, market making). Hook opsional: `FillHandler`, `PriceHandler` (candle yang belum close), `TimerHandler`. Strategi didaftarkan lewat `strategy.Register` dan diimpor oleh `internal/strategy/all`.
- `portfolio.Account` = satu wallet (exchange + market), karena wallet spot dan futures Binance terpisah. `portfolio.Book` menggabungkan semua akun.
- `broker.Broker` adalah antarmuka order. `BracketPlacer` dipakai untuk broker yang butuh SL+TP terhubung (OCO di Binance spot), dan `LeverageSetter` untuk futures. `broker.Sim` dipakai backtest dan nanti paper trading.
- Backtest tidak punya periode warmup terpisah: indikator mulai siap di dalam rentang `-from`. Funding memakai rate konstan (`backtest.funding_rate`), karena histori funding belum disimpan.
- Hasil `backtest -save` masuk ke `strategy_runs` (mode `backtest`, `summary` = metrik JSON), `trades`, dan `equity_snapshots` (sampel per jam).

### Fase 3 (risk, persistensi, paper)

- `bot run` sekarang punya **sesi trading** kalau ada strategi dengan `enabled: true` ([cmd/bot/session.go](../cmd/bot/session.go)). Satu sesi = satu baris `strategy_runs` dengan `instance_id = 'session'`, dan semua order/fill/trade/equity sesi ditulis ke run itu lewat `internal/recorder` (asinkron). Kolom `strategy` di tabel order dan posisi mencatat strategi pemiliknya.
- Feed mencakup gabungan simbol dan timeframe dari `marketdata` dan dari strategi ([cmd/bot/feeds.go](../cmd/bot/feeds.go)). Nama exchange mengikuti mode: di mode `testnet` data, akun, dan order memakai nama `binance-testnet` (`app.ExchangeName`).
- Paper trading memakai `broker.Sim` dengan jam dinding. Setiap update candle yang belum close diperlakukan sebagai tick harga, dan candle close juga dicocokkan supaya lonjakan di antara tick tidak terlewat. Fee dan slippage diambil dari `backtest`.
- Kill switch hanya disimpan di memori: setelah restart, puncak equity dan equity awal hari dihitung ulang dari equity saat itu.
- Mode `testnet`/`live` dengan strategi aktif masih ditolak sampai Fase 4.

### Fase 4 (Binance live/testnet)

- API Binance yang dipakai diverifikasi ke dokumentasi resmi (2026-10-17): **listenKey spot sudah dihapus 2026-02-20** → user data spot lewat WebSocket API; **order kondisional futures lewat Algo Order API** (`/fapi/v1/algoOrder`, event `ALGO_UPDATE`); user data futures di `/private/stream`. Sumber: `binance-spot-api-docs` (GitHub) dan `binance-connector-js` (di-generate dari OpenAPI resmi). Situs developers.binance.com dilindungi AWS WAF (tidak bisa diambil dengan curl).
- `broker.Live` adalah antarmuka broker exchange sungguhan ([internal/broker/live.go](../internal/broker/live.go)); `binance.Trader` mengimplementasikannya ([internal/exchange/binance/](../internal/exchange/binance/)). Bybit/OKX di Fase 7 mengikuti antarmuka yang sama.
- `bot run` mode `testnet`/`live`: satu broker per exchange (semua market di exchange itu), user stream jalan, reconciliation saat start (setelah warmup), tiap menit, dan setiap stream reconnect ([cmd/bot/reconcile.go](../cmd/bot/reconcile.go)). Exchange adalah sumber kebenaran.
- Order mainnet butuh **dua** saklar: `app.mode: live` dan `app.allow_live: true`.

### Fase 5 (Telegram dan observability)

- Telegram memakai HTTP Bot API langsung ([internal/notify/telegram](../internal/notify/telegram/)). Token hanya dari `TB_TELEGRAM__TOKEN`; `chat_ids` di YAML. Perintah berbahaya butuh argumen `confirm`. Perintah didaftarkan di [cmd/bot/commands.go](../cmd/bot/commands.go).
- Metrics trading ada di [internal/metrics/trading.go](../internal/metrics/trading.go); sesi memperbarui gauge setiap 10 detik. Engine punya hook `OnPlaced` untuk latensi broker.
- Dashboard `deploy/grafana/dashboards/trading-bot.json` (uid `tradingbot`) dan alert `deploy/prometheus/alerts.yml`. Belum ada Alertmanager: alert terlihat di UI Prometheus, sedangkan notifikasi real-time dikirim bot lewat Telegram.

### Fase 6 (web dashboard)

- `internal/web`: server `net/http` + templ ([internal/web/views](../internal/web/views/), hasil generate `*_templ.go` ikut di-commit; regenerate dengan `make generate`, tool templ dipasang sebagai `go tool`). htmx 2.0.11 dan lightweight-charts 5.2.1 di-embed (`internal/web/static`), tanpa CDN saat runtime.
- Dashboard jalan di dalam `bot run` (`web.listen`, default `127.0.0.1:8080`). Sesi diakses lewat antarmuka `web.Live` ([cmd/bot/web.go](../cmd/bot/web.go)).
- Rute: `/` (overview), `/partials/*` (HTMX), `POST /actions/{pause,resume,kill,reset}`, `/chart?series=exchange:market:symbol:tf[&run=id]`, `/api/candles`, `/api/equity`, `/backtests`, `/backtests/{id}`.

### Fase 7 (Bybit dan OKX)

- `internal/exchange/bybit` dan `internal/exchange/okx` mengikuti pola Binance: `Client` (market data, `exchange.MarketData`) dan `Trader` (`broker.Live`). Sumber detail API: dokumentasi resmi Bybit (repo `bybit-exchange/docs`) dan OKX (`/docs-v5`), diambil 2026-10-17.
- Simbol memakai format asli exchange (OKX: `BTC-USDT`, `BTC-USDT-SWAP`). Qty di seluruh bot selalu dalam satuan base; adapter OKX mengonversi kontrak ↔ base memakai `ctVal`.
- SL/TP: Bybit memakai order kondisional (`triggerPrice`, `StopOrder` untuk spot); OKX memakai algo `conditional`. Keduanya tidak mengunci saldo sebelum trigger, jadi SL dan TP dipasang sebagai dua order terpisah (tanpa OCO bawaan; engine membatalkan leg lainnya).
- `exchange.TriggerBook` memetakan order hasil trigger ke ID order kondisional (dipakai OKX; Binance punya versi sendiri di `binance.Trader`).
- `broker.UnifiedWallet`: Bybit (UTA) dan OKX memakai satu wallet untuk spot dan futures; bot hanya memperdagangkan satu market per exchange tersebut.
- ID order engine diubah jadi alfanumerik (OKX menolak `-`).

### Fase 8 (strategi lanjutan)

- Strategi baru: `grid`, `dca`, `funding`, `xspread`, `mm` ([internal/strategy/](../internal/strategy/)); helper dua kaki di `internal/strategy/hedge`. Semua didaftarkan lewat `internal/strategy/all`.
- `strategy.Config.Legs` (YAML `strategies[].legs`) menambah instrumen di market/exchange lain; engine meneruskan candle semua instrumen ke strategi, dan sesi membuat akun + subscription untuk tiap leg. `strategy.Context` sekarang punya `LastPrice` dan `Funding`.
- Funding rate dari `exchange.FundingSource` (Binance, Bybit) di-poll sesi tiap 5 menit ke engine (`SetFunding`).
- Backtest sekarang multi-akun (satu `Sim` per exchange, satu akun per exchange+market, saldo awal per akun) dan menjalankan timer strategi.

---

# Plan Rinci: Bot Trading Crypto (Go)

## Context

Folder `/home/rizz404/kodingan/trading-bot` masih kosong. Tujuannya membangun bot trading crypto **otomatis penuh** di **Binance, Bybit, dan OKX**, untuk **spot dan futures**, ditulis dalam **Go**. Belum ada strategi yang terbukti, jadi bot harus menjadi *platform eksperimen*. Strategi dimasukkan sebagai plug-in, dan kode strategi yang sama bisa diuji lewat backtest → paper → testnet → live dengan modal kecil (< $500).

Jangka panjang, bot mendukung indikator teknikal, grid/DCA, arbitrase, dan market making dengan timeframe campuran. **MVP fokus pada strategi indikator teknikal di Binance** untuk membuktikan seluruh pipeline: data → sinyal → risk → order → monitoring.

Keputusan yang sudah diambil:
| Aspek | Pilihan |
|---|---|
| Bahasa | Go |
| Exchange | Binance (MVP), lalu Bybit dan OKX. Adapter ditulis sendiri di belakang satu interface |
| Pasar | Spot + futures (USDT-M perpetual), margin **isolated, one-way** sebagai default |
| Pair | Top 10–20 berdasarkan volume, lewat universe selector dengan filter likuiditas |
| Strategi | Kode Go yang mengimplementasikan interface, dengan parameter di YAML |
| Risk | SL/TP + trailing, batas rugi harian / max drawdown (kill switch), position sizing (% / ATR), batas leverage & exposure |
| Data | PostgreSQL + TimescaleDB |
| Kontrol | Telegram, web dashboard (templ + HTMX), CLI, Prometheus + Grafana |
| Testing | Backtest → paper → testnet → live mini |
| Deploy | Lokal dulu (CachyOS + podman), nanti VPS di Tokyo/Singapore |

## Arsitektur

Engine berbasis event. Strategi tidak pernah memanggil exchange secara langsung. Strategi hanya menerima event market dan mengeluarkan **Signal/OrderIntent**. Intent itu melewati **Risk Manager**, lalu diteruskan ke **Broker**. Ada tiga implementasi Broker yang bisa ditukar:

- `SimBroker` untuk backtest: fill disimulasikan dari candle historis, lengkap dengan fee, slippage, dan funding.
- `PaperBroker` untuk paper trading: data live, fill disimulasikan.
- `LiveBroker`: order sungguhan ke exchange, baik testnet maupun mainnet.

Karena logika strategi dan risk **identik** di ketiga mode, hasil backtest sebanding dengan hasil live.

```
 Exchange WS/REST ──► MarketData Feed ──► Event Bus ──► Strategy (N instance)
                                              │               │ OrderIntent
                                              │               ▼
                                              │         Risk Manager ──(tolak)──► log/alert
                                              │               │ approved
                                              ▼               ▼
                                         Storage ◄──── Order Manager (OMS) ──► Broker (Sim/Paper/Live)
                                    (Timescale/PG)            │ fills, positions
                                              ▲               ▼
                          Telegram / Web / Prometheus ◄── Portfolio / PnL
```

### Struktur folder

```
trading-bot/
  cmd/
    bot/            # main: jalankan engine (mode: paper|testnet|live)
    backtest/       # CLI backtest + laporan
    datafetch/      # download candle historis ke Timescale
  internal/
    domain/         # tipe inti: Symbol, Candle, Order, Fill, Position, Side, MarketType (pakai decimal)
    exchange/       # interface Exchange + tipe umum, rate limiter, ws reconnect helper
      binance/      # spot + usdm futures (REST + WS)
      bybit/        # (fase 7)
      okx/          # (fase 7)
    marketdata/     # feed candle/ticker, agregasi multi-timeframe, universe selector (top-N volume)
    indicator/      # EMA, SMA, RSI, MACD, ATR, Bollinger (streaming/incremental)
    strategy/       # interface Strategy + registry
      emacross/     # strategi contoh MVP
    risk/           # pre-trade checks, sizing, kill switch, exposure limits
    oms/            # order manager: lifecycle order, SL/TP/trailing, reconciliation
    broker/         # SimBroker, PaperBroker, LiveBroker
    portfolio/      # posisi, saldo, PnL realized/unrealized, equity curve
    backtest/       # runner, model fee/slippage/funding, metrik (Sharpe, MDD, win rate, PF)
    storage/        # pgx pool, migrasi goose (embed); query sqlc menyusul
    notify/telegram/
    web/            # templ + HTMX + lightweight-charts
    metrics/        # Prometheus collectors
    config/         # load YAML + env (secret dari env, bukan file)
  migrations/
  configs/
    config.example.yaml
    strategies/emacross.yaml
  deploy/
    docker-compose.yml   # timescaledb, prometheus, grafana
    grafana/dashboards/
  .mise.toml             # versi Go (pengganti flake.nix, lihat catatan implementasi)
```

### Interface inti (sketsa)

```go
// exchange
type Exchange interface {
    Name() string
    Instruments(ctx, MarketType) ([]Instrument, error) // tick size, lot size, min notional
    Klines(ctx, sym, tf, from, to) ([]Candle, error)
    Tickers24h(ctx, MarketType) ([]Ticker, error)      // untuk universe selector
    SubscribeKlines(ctx, []Sub) (<-chan Candle, error)
    SubscribeUserData(ctx) (<-chan UserEvent, error)   // fills, order update, balance
    PlaceOrder(ctx, OrderRequest) (Order, error)
    CancelOrder(ctx, ...) error
    OpenOrders(ctx, ...) ([]Order, error)
    Balances(ctx, MarketType) ([]Balance, error)
    Positions(ctx) ([]Position, error)                 // futures
    SetLeverage(ctx, sym, int) error
    SetMarginMode(ctx, sym, MarginMode) error
}

// strategy
type Strategy interface {
    Name() string
    Init(ctx StrategyContext, params map[string]any) error
    Timeframes() []Timeframe
    OnCandle(c Candle) []OrderIntent
    OnFill(f Fill)
}
```

### Prinsip teknis penting
- **Semua harga dan kuantitas memakai `shopspring/decimal`**, tidak pernah `float64`. Indikator boleh float.
- **Pembulatan** mengikuti tick size, lot size, dan min notional dari `Instruments()` sebelum order dikirim.
- **Rate limiter** per exchange (token bucket, mengikuti header weight Binance) dan **WS auto-reconnect** dengan resubscribe serta backfill gap candle lewat REST.
- **Sinkronisasi jam** dengan server time exchange untuk request yang di-sign.
- **Reconciliation saat startup dan berkala**: state order dan posisi di DB dicocokkan dengan exchange. Exchange adalah sumber kebenaran.
- **SL/TP dipasang di exchange** (stop order native), bukan hanya dipantau oleh bot. Posisi tetap terlindungi kalau bot mati.
- **Idempotensi**: `clientOrderId` deterministik supaya retry tidak membuat order ganda.
- **Keamanan**: API key dari env/secret file. Izin key: trade saja, **tanpa withdraw**, dengan IP whitelist. Setiap environment (testnet atau live) memakai key terpisah.
- **Kill switch**: kalau rugi harian atau drawdown melewati batas, bot membatalkan order, menutup posisi sesuai config, menghentikan entry baru, lalu mengirim alert Telegram.
- Logging memakai `log/slog` dalam format JSON.

## Library yang dipakai
- `github.com/shopspring/decimal`: aritmatika uang
- `github.com/jackc/pgx/v5`, `sqlc`, `pressly/goose`: database dan migrasi
- `github.com/coder/websocket` (atau `gorilla/websocket`): WebSocket
- `github.com/knadh/koanf`: config YAML + env
- `github.com/go-telegram/bot`: Telegram
- `github.com/prometheus/client_golang`: metrics
- `github.com/a-h/templ` + HTMX + TradingView `lightweight-charts`: dashboard
- `golang.org/x/time/rate`: rate limit
- `github.com/stretchr/testify`: test

## Fase pengerjaan

Fase 0–1 dikerjakan dengan test langsung. Mulai Fase 2, ikuti "Aturan fase build": bangun dulu, test di Fase 9, perbaiki di Fase 10.

**Fase 0 · Fondasi** (selesai)
- `go mod init`, `.mise.toml`, `docker-compose` (TimescaleDB, Prometheus, Grafana), config loader, slog, kerangka Makefile/justfile.
- Migrasi awal: `candles` (hypertable), `instruments`, `orders`, `fills`, `positions`, `equity_snapshots`, `strategy_runs`.

**Fase 1 · Domain + Binance market data** (selesai)
- Tipe domain, interface `Exchange`, adapter Binance (spot + USDⓈ-M futures): instruments, klines REST, tickers 24h, WS kline.
- `cmd/datafetch`: download candle historis ke Timescale.
- Universe selector: top-N berdasarkan quote volume, dengan filter min volume, max spread, dan exclude stablecoin/leveraged token.

**Fase 2 · Indikator + Strategy + Backtester**
- Indikator streaming: EMA, SMA, RSI, MACD, ATR, Bollinger.
- Interface strategi + registry. Strategi contoh `emacross` (EMA cross + filter RSI, SL/TP berbasis ATR).
- `SimBroker` dengan fee maker/taker, slippage (bps), dan funding rate untuk futures.
- `cmd/backtest`: metrik total return, CAGR, max drawdown, Sharpe/Sortino, win rate, profit factor, jumlah trade. Equity curve diekspor ke CSV/DB.
- Dukungan walk-forward / out-of-sample split sederhana untuk menghindari overfitting.

**Fase 3 · Risk + OMS + Paper trading**
- Risk manager: sizing (% equity atau berbasis ATR), max leverage, max exposure per aset/exchange/total, max posisi terbuka, batas rugi harian dan drawdown (kill switch).
- OMS: lifecycle order, SL/TP/trailing, persist ke DB.
- `PaperBroker` dan `bot run` mode `paper` yang menyambungkan `marketdata.Feed.OnCandle` ke strategi.

**Fase 4 · Binance LiveBroker**
- Signed REST (HMAC), order endpoint (spot + futures), user data stream (listenKey + keepalive), set leverage/margin isolated, stop order native.
- Reconciliation saat startup dan berkala, `clientOrderId` idempoten, penanganan error/reject.
- Mode `testnet` dan `live` di `bot run` (live tetap dijaga validasi config; order mainnet baru dijalankan di Fase 11).

**Fase 5 · Telegram + Prometheus/Grafana**
- Notifikasi: entry/exit, SL/TP kena, error, kill switch, ringkasan harian.
- Perintah: `/status`, `/positions`, `/balance`, `/pnl`, `/pause`, `/resume`, `/stop`, `/killswitch`. Hanya `chat_id` di whitelist yang bisa memakainya.
- Metrics: equity, PnL, latensi order, WS reconnect, error rate, rate-limit usage. Dashboard Grafana dan alert rules.

**Fase 6 · Web dashboard**
- templ + HTMX: overview equity, posisi terbuka, riwayat trade, chart candle dengan marker entry/exit (lightweight-charts), daftar strategi dan statusnya, hasil backtest.
- Auth sederhana (password/token). Default bind ke localhost.

**Fase 7 · Bybit + OKX**
- Adapter untuk interface yang sama (spot + linear perp), termasuk endpoint testnet/demo masing-masing.

**Fase 8 · Strategi lanjutan**
- Grid / DCA (manajemen banyak order terbuka).
- Arbitrase: cross-exchange, dan spot-perp funding rate (basis/cash-and-carry).
- Market making: butuh WS order book L2, manajemen inventori, dan latensi rendah.

**Fase 9 · Testing (semua fitur)**
- Kerjakan seluruh "Backlog testing" dan semua butir di bagian "Verifikasi" di bawah.
- Unit test: indikator (dibandingkan nilai referensi), sizing/risk, kill switch, OMS, SimBroker (fee/slippage/funding), metrik backtest, strategi.
- Contract test bersama untuk semua adapter exchange (Binance, Bybit, OKX) dengan fixture respons REST/WS yang direkam, ditambah integration test opsional ke testnet dengan build tag `integration`.
- Golden test backtest `emacross`, lalu eksperimen backtest untuk menentukan parameter default.
- Uji end-to-end: paper run 24 jam, order sungguhan di testnet tiap exchange, reconciliation, Telegram, dan dashboard.
- Hasil: daftar temuan (bug, perilaku salah, celah keamanan) dicatat di plan, diurutkan berdasarkan tingkat keparahan.

**Fase 10 · Fixing**
- Perbaiki semua temuan Fase 9. Prioritas: keamanan dana (order, risk, kill switch, secret) → integritas data → sisanya.
- Setiap perbaikan disertai test yang membuktikan bug-nya hilang. Ulangi test terkait sampai semuanya lolos.

**Fase 11 · Live mini + VPS**
- Hanya setelah Fase 10 bersih **dan** user menyetujui secara eksplisit.
- Live di Binance dengan modal sangat kecil dan batas risk ketat. Strategi baru naik ke tahap ini setelah lolos backtest dan minimal 1–2 minggu paper/testnet.
- Deploy ke VPS di Tokyo/Singapore (systemd atau Docker), backup DB, dan alert kalau bot mati (heartbeat).

## Catatan realistis
- **Modal < $500 + 10–20 pair + futures**: min notional per order (sekitar 5–100 USDT tergantung pair dan exchange) membuat modal cepat terbagi tipis. Saat live awal, sebaiknya batasi ke 2–5 pair aktif sekaligus. Universe 20 pair tetap dipakai untuk backtest dan screening.
- **Market making dan arbitrase cross-exchange** praktis tidak menguntungkan dengan modal < $500 karena fee tier dan saldo harus terpecah di beberapa exchange. Fitur ini tetap ada di roadmap, tapi diprioritaskan terakhir.
- Pastikan akses dan legalitas exchange dari Indonesia sudah kamu cek sendiri. Ini di luar cakupan kode.

## Verifikasi (dikerjakan di Fase 9)
- `go test ./...` untuk unit test indikator (dibandingkan nilai referensi), sizing/risk, pembulatan tick/lot, dan agregasi timeframe.
- Contract test adapter exchange dengan fixture respons REST/WS yang direkam. Integration test opsional ke testnet, dengan build tag `integration`.
- Golden test backtest: `emacross` di BTCUSDT pada rentang tetap harus menghasilkan metrik yang deterministik.
- Paper run 24 jam: tanpa panic, WS reconnect teruji (putuskan jaringan secara manual), tidak ada gap candle, dan state DB konsisten.
- Testnet: kirim order spot dan futures sungguhan. Cek SL/TP native terpasang di exchange, restart bot di tengah posisi, lalu pastikan reconciliation memulihkan state. Picu kill switch dengan batas rugi buatan.
- Telegram, `/metrics` Prometheus, dan dashboard web dicek manual.
- `golangci-lint run` bersih.
