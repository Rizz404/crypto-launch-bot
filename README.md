# trading-bot

Bot trading crypto otomatis dalam Go (Binance dan Bybit; spot + futures).
Rencana aktif dan progres ada di [Plan 2](docs/plan-2.md): multi-instance dengan
modal terpisah dalam satu akun dan beberapa strategi per instance.
Implementasi runtime managed sudah tersedia secara lokal untuk Binance Spot;
belum dirilis ke VPS. [Panduan penggunaan](docs/managed-runtime.md) dan
[handover Claude](docs/HANDOVER-CLAUDE.md) mencatat fitur, hasil tes, dan batasnya.
[Plan 1](docs/plan.md) sudah selesai sebagai siklus pertama dan menjadi arsip;
pekerjaan tertunda dipindahkan ke Plan 2. [Indeks dokumentasi](docs/README.md).

Rilis dan deploy VPS mengikuti alur GitHub Actions → GHCR → image dengan digest
tetap. Panduan: [docs/deployment.md](docs/deployment.md). Konfigurasi VPS memakai
mode paper secara default. Observasi paper 24 jam Plan 1 sudah lulus;
uji multi-strategi testnet berikutnya dihentikan lebih awal pada 20 Oktober.
Belum ada trial baru atau transaksi mainnet yang disetujui.

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

Contoh konfigurasi ada di [configs/config.example.yaml](configs/config.example.yaml). Unit/contract test lokal dan integration paper dengan restart sudah berjalan. Bukti siklus pertama ada di [Plan 1](docs/plan.md#hasil-testing-2026-10-17);
pekerjaan lanjutan ada di [Plan 2](docs/plan-2.md). Pilihan kandidat pengujian
berikutnya tercatat di [keputusan strategi](docs/strategy-decision-2026-10-20.md).

Riset lima kandidat Spot (belum diaktifkan untuk trading) ada di
[ringkasan pengujian](docs/testing/candidates-2026-10-19/summary.txt).
Parameter yang dikunci sebelum pengujian tersedia di
[candidate-spec.json](docs/testing/candidates-2026-10-19/candidate-spec.json).
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
