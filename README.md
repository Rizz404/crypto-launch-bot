# trading-bot

Bot trading crypto otomatis dalam Go (Binance, Bybit, OKX; spot + futures).
Plan, keputusan, dan progres ada di [docs/plan.md](docs/plan.md).

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

> **Jaringan:** kalau DNS lokal memblokir domain exchange (semua host Binance/Bybit/OKX
> me-resolve ke IP yang sama), bot tidak bisa terhubung ke mainnet. Perbaiki DNS sistem
> (misalnya DNS-over-TLS di systemd-resolved). Testnet Binance tidak terkena blokir ini.

- Metrics: http://127.0.0.1:9100/metrics, health: http://127.0.0.1:9100/healthz
- Prometheus: http://127.0.0.1:9090
- Grafana: http://127.0.0.1:3000 (admin/admin, ganti lewat `GRAFANA_ADMIN_PASSWORD`)

Menjalankan bot di dalam container:

```sh
podman compose -f deploy/compose.yml --profile bot up -d --build
```

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
make lint
```
