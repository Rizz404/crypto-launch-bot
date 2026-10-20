# Bukti pengujian

Bukti siklus pertama dirujuk dari [Plan 1 yang sudah ditutup](../plan.md).
Pengujian lanjutan dan pemindahan pekerjaan direncanakan di
[Plan 2](../plan-2.md#p2-7--testing-menyeluruh). File JSON berisi angka dan
asumsi yang dipakai; tidak ada kredensial. Bukti lama dipertahankan, bukan
hasil kelulusan otomatis fitur multi-instance baru.

| File | Isi |
|---|---|
| [p2-0-audit-2026-10-20.md](p2-0-audit-2026-10-20.md) | Audit OCO, dust DOGE, ekuitas transient dan kontrak akhir eksperimen; dua fix lokal dengan regression. |
| [testnet-stop-2026-10-20/analysis.md](testnet-stop-2026-10-20/analysis.md) | Penghentian dini testnet, pembatalan tujuh order, hasil lima strategi dan backlog aplikasi. |
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

Ekspor detail `*-equity.csv` dan `*-trades.csv` dari backtest tidak disimpan di
Git karena dapat dibuat ulang dengan script riset. Ringkasan, parameter,
manifest sumber/checksum, hasil agregat, dan bukti penutupan testnet tetap
diversikan. Arsip harga, binary, log antrean, database, serta kredensial runtime
berada di direktori lokal yang diabaikan Git.

## Plan 2 — implementasi lokal 20 Oktober 2026

[Laporan testing/fixing](plan2-local-2026-10-20.md) dan
[smoke test modal bersama](plan2-shared-capital-smoke-2026-10-20.json).
Suite baru mencakup ledger, ownership, reservasi, crash/replay, lease,
checkpoint, HTTP/control dan CLI. [QA browser dashboard fixture](plan2-browser-qa-2026-10-20.md)
selesai dengan empat perbaikan UI. Bukti operasi VPS masih terbuka; ini bukan
laporan trial prospektif atau hasil mainnet.
