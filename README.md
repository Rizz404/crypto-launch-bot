# trading-bot

Bot trading crypto otomatis dalam Go (Binance dan Bybit; spot + futures).
Status proyek dan rencana: [Plan 3](docs/plan-3.md). Hasil riset strategi: [riset strategi](docs/strategy-2.md).
Cara kerja, kontrol dan deploy: [panduan](docs/guide.md).

> **Dokumentasi:** yang boleh dibaca manusia hanya [docs/readme.md](docs/readme.md) dan tiga file yang
> ditautkan di atas. **Jangan baca `docs/agents/` dan `docs/testing/`**: itu catatan panjang untuk agent
> (handover, log, bukti JSON), tidak perlu dibaca.

Semua trading saat ini **paper/virtual** (bukan uang asli). Transaksi mainnet dengan uang asli belum diizinkan.
Rilis ke server mengikuti GitHub Actions → GHCR → image dengan digest tetap.

## Prasyarat

- Go 1.27 (lihat `.mise.toml`)
- podman + podman-compose
- golangci-lint (opsional)

## Menjalankan secara lokal

```sh
cp configs/config.example.yaml configs/config.yaml
cp .env.example .env            # isi API key di sini, jangan di YAML

make up                         # TimescaleDB, Prometheus, Grafana
make migrate                    # bot migrate up
make run                        # bot run: rekam candle live (+ backfill gap)
```

Download data historis:

```sh
./bin/datafetch -market futures -tf 1h,4h -from 2025-01-01          # universe top-N dari config
./bin/datafetch -market spot -symbols BTCUSDT,ETHUSDT -tf 1m -from 2026-09-01
```

> **Jaringan:** kalau DNS lokal memblokir domain exchange (semua host Binance/Bybit
> me-resolve ke IP yang sama), bot tidak bisa terhubung ke mainnet. Perbaiki DNS sistem
> (misalnya DNS-over-TLS di systemd-resolved). Testnet Binance tidak terkena blokir ini.

- Metrics: http://127.0.0.1:9100/metrics, health: http://127.0.0.1:9100/healthz
- Prometheus: http://127.0.0.1:9090
- Grafana: http://127.0.0.1:3000 (admin/admin, ganti lewat `GRAFANA_ADMIN_PASSWORD`)

Menjalankan bot di dalam container:

```sh
podman compose -f deploy/compose.yml --profile bot up -d --build
```

## Strategi bawaan

| Nama | Ide |
|---|---|
| `emacross` | EMA cross + filter RSI, SL/TP berbasis ATR |
| `grid` | order limit bertingkat (long grid) |
| `dca` | beli berkala, jual saat untung dari harga rata-rata |
| `funding` | long spot + short perp saat funding tinggi |
| `xspread` | selisih harga perp antar-exchange |
| `mm` | market making sederhana (quote post-only) |
| `trendspot` | eksperimen Spot: breakout, momentum, atau pullback dalam tren naik |

Contoh konfigurasi ada di [configs/config.example.yaml](configs/config.example.yaml). Unit/contract test lokal dan integration paper dengan restart sudah berjalan.

Riset lima kandidat Spot (belum diaktifkan untuk trading); hasilnya ada di `docs/testing/candidates/` (untuk agent).
Reproduksi memakai arsip harga lokal: `go run ./scripts/backtest-year -suite candidates`.
Saat pertama kali menjalankan eksperimen, buat spesifikasi dengan
`go run ./scripts/backtest-year -suite candidates -declare` terlebih dahulu.

## Perintah

```
bot [-config configs/config.yaml] [-env-file .env] run
bot migrate [up|down|status]
bot version
```

Semua key config bisa di-override lewat env dengan prefix `TB_` dan `__` sebagai pemisah, misalnya `TB_APP__MODE=testnet` atau `TB_DATABASE__URL=...`.

## Pengembangan

```sh
make test
make test-integration   # butuh `make up`
make test-monitoring    # promtool dalam container sementara, tanpa jaringan
make lint
```
