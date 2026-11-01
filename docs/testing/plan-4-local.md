# Plan 4 — uji lokal 1 November 2026

Source belum di-commit (working tree di atas `7d56251`). Tidak ada perubahan VPS.

## Otomatis

- `go test -race ./...` lulus; `golangci-lint run` 0 issue; `make generate` bersih.
- Test baru: mode wallet + credential terpisah + gate live/TOTP (`internal/config`), urutan snapshot+diff
  order book Binance, gap/resync, history sekali REST (`internal/marketview`), parser depth/aggTrade/ticker
  termasuk kunci beda kapital (`internal/exchange/binance`), order manual market/limit/sell/cancel/close,
  replay idempoten, leg proteksi tidak bisa dibatalkan, binding bot menolak order manual
  (`internal/managed/manual_test.go`), vektor RFC 6238, login + replay kode + lockout, aksi live butuh kode,
  candle API dan SSE (`internal/managed/trade_test.go`).

## Manual (paper, Binance mainnet publik)

Postgres lokal (podman `deploy/compose.yml` service `db`), DB `tradingbot_p4qa`, config
`configs/managed.trading.example.yaml`, web loopback tanpa password (alur login dicakup unit test).

- Market stream 27 stream terhubung; order book tersinkron 14 level/sisi, ticker dan tape live di browser
  (Chromium headless via CDP, desktop 1600 px terang dan 500 px gelap).
- Form order: market buy 0,05 ETH dengan SL 2100/TP 2500 terisi di candle 1m berikutnya; SL/TP terpasang;
  limit buy 0,01 @ 2000 terbuka; order manual ke binding bot ditolak dengan pesan.
- Restart bot: posisi, limit dan SL/TP pulih, tanpa attention. Cancel limit lewat form → terminal;
  tutup posisi → proteksi dibatalkan dan exit market dipasang.
- Layout mobile tidak meluber; tabel scroll di dalam kartu.

## Batas

- Testnet nyata belum diuji (butuh API key testnet). Live tidak diuji.
- Peringatan CSP inline style dari `lightweight-charts.js` sudah ada sebelumnya; chart tetap tampil.
- QA browser hanya Chromium headless; tidak ada pengukuran beban/bobot API berjam-jam.
