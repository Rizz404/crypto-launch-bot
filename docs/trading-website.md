# Website trading untuk bot

Diimpor dari repo `Rizz404/trading-bot` (di sana bernama Plan 4) melalui merge 10 November 2026; di repo ini roadmap utama tetap [Plan 4 presale](plan-4.md). Dimulai 1 November 2026. Plan 3 ditutup; semua runtime di VPS dihentikan
([review akhir](testing/final-review.md)). Detail teknis dan checklist: [agents/trading-website.md](agents/trading-website.md).

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
(bot EMA ETH 1h + strategi manual BTC/ETH/SOL), login email + password (akun di database, dibuat lewat `/register`). Sejak 8 November mulai fresh: akun **paper** dan **testnet** masing-masing 1.000 USDT dengan 4 strategi
(EMA 1h, pullback 4h, DCA 1h, grid) pada BTC/ETH/SOL/BNB/XRP. Data lama disimpan, tidak dipakai.
Whitelist IP dipasang lewat gate.nexlabs.cloud. Live belum. Bybit dihapus.

## Tahap

1. TW-0 Akun paper/testnet/live dalam satu proses.
2. TW-1 Data pasar bersama (websocket → server → browser).
3. TW-2 Halaman trading: chart, order book, form order, posisi/order/riwayat.
4. TW-3 Halaman bot sebagai pusat: menu samping Trading bot, halaman awal setelah login.
5. TW-4 Order manual dan cancel/close per order/posisi.
6. TW-5 Login + TOTP + whitelist IP.
7. TW-6 Rilis bertahap: lokal → VPS paper+testnet → live hanya dengan izin.
