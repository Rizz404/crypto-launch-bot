# Plan 4 — Website trading (detail)

Ringkasan untuk user: [../plan-4.md](../plan-4.md). Dimulai 1 November 2026 setelah
[review akhir Plan 3](../testing/final-review.md); runtime VPS berhenti
([handover VPS](handover-vps.md)). Perbarui status checklist di file ini.

## Kode yang dipakai ulang

- UI bersama `internal/ui` (Penguin UI style, `components.templ`, `static/app.js`,
  `static/lightweight-charts.js`), build `make generate` (templ + Tailwind standalone, tanpa Node).
- Dashboard managed `internal/managed/http.go`, `dashboard.templ` (tab, `/api/state`, `/control`,
  CSRF per proses, cek Origin, CSP `script-src 'self'`).
- Chart candle + marker dari dashboard lama `internal/web/server.go` (`/api/candles`) dan
  `internal/ui/static/app.js` (`baseChart`, `candles`).
- Eksekusi `managed/runtime.go` `scopedBroker.PlaceOrder` (reservasi, harga segar, filter, journal),
  `broker.Live`, `broker.NewSim`, Binance `orders.go`/`market.go`/`stream.go`/`userstream.go`,
  `exchange.NewWeightLimiter`.

## Checklist

### P4-0 Akun multi-mode
- [x] `wallets[].mode` (`paper|testnet|live`, kosong = `app.mode`) dan `wallets[].allow_live`.
- [x] Credential testnet terpisah: `exchanges.<ex>.testnet_api_key/testnet_api_secret`;
      `api_key/api_secret` untuk live (atau testnet pada config lama `testnet: true`).
- [x] Lease/duplikat per `exchange:mode`; broker dan sumber data per mode wallet.
- [x] Gate live: `app.allow_live` **dan** `wallets[].allow_live`.

### P4-1 Data pasar bersama
- [x] Hub WS: kline, `@depth@100ms` + snapshot REST, `@aggTrade`, `@ticker` (24 jam); resync hanya saat gap.
- [x] SSE `/stream` ke browser; tidak ada REST per browser.

### P4-2 Halaman trading
- [x] Header switcher akun + badge mode (live merah), pair selector.
- [x] Chart, order book, trade tape, form order, tab posisi/open order/riwayat/saldo.
- [x] Komponen baru di `internal/ui` dengan pola Penguin UI.

### P4-3 Bot
- [x] Sidebar bot-sentris (`view.Sidebar`, `ui.NavGroup` collapsible `<details>`): Trading bot (4 tab) terbuka, Trading manual
      nonaktif "Segera" (route `/trade` tetap ada, tidak ditautkan), Akun trading, Ganti password, Keluar. Login → `/`.
- [~] Halaman bot: status/PnL/kontrol/buat dari template memakai tab Portofolio yang ada (+ template `manual`). **Belum:** pembanding buy-and-hold dan halaman bot terpisah.

### P4-4 Intervensi manual
- [x] Strategi no-op `manual`, binding `<instance>/manual`.
- [x] Command `place_order`, `cancel_order`, `close_position`.

### P4-5 Akses
- [x] Login sesi email + password; akun tunggal di tabel `web_owner` (migrasi 6, PBKDF2-SHA256 600k),
      `/register` sekali dengan kode setup dari log, `/account` ganti password, konfirmasi password untuk wallet live,
      rate limit login. TOTP dihapus 7 November 2026. `TB_WEB__PASSWORD` hanya untuk dashboard legacy/catalyst.
- [x] Whitelist IP di vhost nginx `ana.nexlabs.cloud`: dipasang user lewat `gate.nexlabs.cloud` (7 November 2026).

### P4-6 Rilis
- [~] VPS paper + testnet berjalan di DB `tradingbot_plan4_lab` sejak 8 November 2026 (4 strategi × 5 pair per wallet).
      Order testnet pertama terisi (DCA, 5 fill). Belum: bukti SL/TP dan jual terisi, restart dengan saldo cocok. Live hanya dengan izin. Bybit dihapus dari kode (tidak tersedia di Indonesia).

## Status (1 November 2026)

Implementasi P4-0, P4-1, P4-2, P4-4, P4-5 (bagian aplikasi) dirilis paper di VPS 1 November 23:24 WIB
(source `048ae24`, [handover VPS](handover-vps.md)).
Bukti: [uji lokal](../testing/plan-4-local.md). Whitelist IP nginx dipasang user lewat `gate.nexlabs.cloud`
(7 November 2026). Belum: uji testnet nyata (butuh API key testnet), buy-and-hold di halaman bot;
observer tanpa `research` aktif lagi sejak 8 November 2026.

Catatan desain:
- Hub `internal/marketview`: satu combined stream per jaringan (`@depth@100ms`, `@aggTrade`, `@ticker`,
  `@kline_<tf>` untuk 1m/5m/15m/1h/4h/1d); snapshot REST depth (bobot 50) hanya saat (re)connect atau gap;
  history chart 1 REST klines per simbol/timeframe lalu diikuti stream. Simbol = `marketdata.symbols` +
  simbol binding di config (binding yang dibuat lewat UI belum masuk hub sampai restart dengan config baru).
- `Engine.Submit` memakai sizing/risk/proteksi yang sama dengan intent strategi; `ErrNoOrder` bila tidak
  ada order dibuat. `Engine.Cancel` menolak leg proteksi (runtime akan flatten posisi tanpa proteksi).
- Mode per wallet: `wallets[].mode`; `app.mode` hanya default. Metrik `mode` dan pesan stop Telegram masih
  memakai `app.mode`.
