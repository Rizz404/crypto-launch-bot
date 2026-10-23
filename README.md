# trading-bot

Bot trading crypto otomatis dalam Go (Binance dan Bybit; spot + futures).
Rencana aktif dan progres ada di [Plan 3](docs/plan-3.md): riset katalis/AI,
hold berbasis tesis, proteksi dan pekerjaan tersisa dari platform multi-instance.
Strategi AI belum diimplementasikan atau diaktifkan. Runtime managed Binance
Spot sudah dirilis; checkpoint 23 Oktober mencatat paper pembanding tahap dua
6 instance/18 binding BTC/ETH/SOL dengan T0 22 Oktober 19:58:45 WIB, modal awal
747,83957456237531 USDT virtual. Rilis notifikasi terbaru tidak mereset T0.
Status dan review hari 1/3/7: [handover VPS](docs/handover-vps.md).
[Panduan penggunaan](docs/managed-runtime.md) dan
[handover agent](docs/handover-agent.md) mencatat fitur, hasil tes, dan batasnya.
[Plan 1](docs/plan.md) dan [Plan 2](docs/plan-2.md) selesai sebagai siklus dan
menjadi arsip; seluruh pekerjaan terbuka diteruskan ke Plan 3.
[Indeks dokumentasi](docs/readme.md).

Rilis dan deploy VPS mengikuti alur GitHub Actions → GHCR → image dengan digest
tetap. Panduan: [docs/deployment.md](docs/deployment.md). Konfigurasi VPS memakai
mode paper secara default. Observasi paper 24 jam Plan 1 sudah lulus;
uji multi-strategi testnet berikutnya dihentikan lebih awal pada 20 Oktober.
Trial ETH berikutnya juga sudah ditutup; trial riset memakai DB baru.
Transaksi mainnet dengan uang asli belum diizinkan.

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
pekerjaan lanjutan ada di [Plan 3](docs/plan-3.md). Pembanding lama dan matriks
ekspansinya tetap tercatat di [riset strategi](docs/strategy-2.md).

Riset lima kandidat Spot (belum diaktifkan untuk trading) ada di
[ringkasan pengujian](docs/testing/candidates/summary.txt).
Parameter yang dikunci sebelum pengujian tersedia di
[candidate-spec.json](docs/testing/candidates/candidate-spec.json).
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
