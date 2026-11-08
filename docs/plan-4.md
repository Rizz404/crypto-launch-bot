# Plan 4 — Website trading untuk bot

Rencana aktif sejak 1 November 2026. Plan 3 ditutup; semua runtime di VPS dihentikan
([review akhir](testing/final-review.md)). Detail teknis dan checklist: [agents/plan-4.md](agents/plan-4.md).

## Tujuan

Website yang bisa dipakai seperti website trading pada umumnya, dengan **bot sebagai inti**.
Website dipakai untuk memantau dan mengendalikan bot, serta mengintervensi posisinya.

## Keputusan user (1 November)

| Hal | Keputusan |
|---|---|
| Akun | Paper, testnet dan live **jalan bersamaan**, dipilih lewat switcher di header |
| Fitur v1 | Chart candlestick live, order manual, order book + trade, portofolio + riwayat |
| Data | Ikuti API Binance tanpa membuang limit: websocket di server, dibagikan ke browser |
| Order manual | Intervensi bot (binding `manual` per instance; modal/risiko tetap tercatat) |
| Pengguna | Hanya pemilik |
| Akses | Publik via domain + whitelist IP, login + TOTP |
| Tampilan | templ + HTMX + Alpine + lightweight-charts, komponen bergaya **Penguin UI** |
| Uang asli | Hanya wallet live yang diizinkan eksplisit di config, dengan izin terpisah dari user |

## Paper vs testnet

- **Paper**: harga Binance asli, order dan saldo disimulasikan bot. Untuk menilai strategi.
- **Testnet**: order dikirim ke server latihan Binance. Untuk menguji jalur eksekusi API;
  harga dan likuiditasnya tidak realistis.

## Status

Dirilis 1 November 23:24 WIB di `https://ana.nexlabs.cloud/trade`: akun **paper** 1.000 USDT virtual
(bot EMA ETH 1h + strategi manual BTC/ETH/SOL), login email + password (akun di database, dibuat lewat `/register`). Sejak 8 November ada juga akun **testnet** 1.000 USDT (bot sama, order ke Binance Spot Testnet).
Whitelist IP dipasang lewat gate.nexlabs.cloud. Live belum. Bybit dihapus.

## Tahap

1. P4-0 Akun paper/testnet/live dalam satu proses.
2. P4-1 Data pasar bersama (websocket → server → browser).
3. P4-2 Halaman trading: chart, order book, form order, posisi/order/riwayat.
4. P4-3 Halaman bot sebagai pusat: menu samping Trading bot, halaman awal setelah login.
5. P4-4 Order manual dan cancel/close per order/posisi.
6. P4-5 Login + TOTP + whitelist IP.
7. P4-6 Rilis bertahap: lokal → VPS paper+testnet → live hanya dengan izin.
