# Bukti pengujian

Ringkasan bukti yang dirujuk dari [plan](../plan.md). File JSON berisi angka dan
asumsi yang dipakai; tidak ada kredensial.

| File | Isi |
|---|---|
| [browser-verification.json](browser-verification.json) | Uji dashboard di browser (desktop light, mobile dark, CSP, chart). |
| [monitoring-verification.json](monitoring-verification.json) | Grafana, datasource, dan alert Prometheus lokal. |
| [historical-backtest-2025q1.json](historical-backtest-2025q1.json) | Backtest awal emacross BTCUSDT 1h Q1 2025 dengan split. |
| [emacross-grid-2023-2026.json](emacross-grid-2023-2026.json) | Grid emacross 3 tahun × 5 pair × 1h/4h, walk-forward: tidak ada edge. |
| [emacross-trend-2023-2026.json](emacross-trend-2023-2026.json) | Emacross dengan filter tren `trend_ema`: drawdown turun, return sekitar 0–1%. |
| [dca-grid-2023-2026.json](dca-grid-2023-2026.json) | DCA (split) dan grid kuartalan, dibandingkan dengan buy-and-hold. |
| [vps-handoff-2026-10-17.json](vps-handoff-2026-10-17.json) | Checkpoint read-only VPS saat handoff dari Codex. |
| [vps-ops-2026-10-17.json](vps-ops-2026-10-17.json) | Deploy, resolver DNS, uji restore, dan timer di VPS. |

Rekaman payload Binance Spot Testnet untuk test offline ada di
`internal/exchange/binance/testdata/testnet/` (diputar ulang oleh
`TestRecordedSpotTestnetPayloads`).
