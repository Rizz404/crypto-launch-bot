# Handoff: Bot Trading Crypto (Go)

> Dokumen ini adalah sumber kebenaran untuk perencanaan. Agent/developer yang melanjutkan: baca seluruh file ini sebelum menulis kode.

## Status saat ini (2026-10-17)

- **Fase 0 dan Fase 1 selesai** (2026-10-17). `bot run` merekam candle live Binance (spot/futures) ke TimescaleDB dengan backfill gap otomatis, dan `datafetch` mengunduh data historis. Engine trading (strategi, risk, order) belum ada.
- Seluruh keputusan di tabel "Keputusan yang sudah diambil" di bawah sudah disetujui user lewat tanya-jawab. Jangan ditanyakan ulang kecuali ada alasan teknis kuat.
- **Langkah berikutnya: Fase 2** (indikator streaming, interface Strategy + `emacross`, SimBroker, `cmd/backtest`). Data historis dibaca lewat `storage.Store.Candles`.
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
- [ ] Fase 2: Indikator + Strategy + Backtester
- [ ] Fase 3: Risk + OMS + Paper trading
- [ ] Fase 4: Binance LiveBroker (testnet/mainnet)
- [ ] Fase 5: Telegram + Prometheus/Grafana
- [ ] Fase 6: Web dashboard
- [ ] Fase 7: Bybit + OKX
- [ ] Fase 8: Strategi lanjutan (grid/DCA, arbitrase, market making)
- [ ] Fase 9: Testing (semua fitur)
- [ ] Fase 10: Fixing (semua temuan Fase 9)
- [ ] Fase 11: Live mini + VPS

## Backlog testing (diisi selama Fase 2–8, dikerjakan di Fase 9)

- Uji adapter Binance terhadap mainnet setelah DNS lokal diperbaiki (lihat catatan jaringan di bawah).
- Universe spot: putuskan dan uji pengecualian saham ter-tokenisasi (lihat pertanyaan terbuka).

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
