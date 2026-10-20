# Penggunaan multi-instance Spot

Implementasi lokal Plan 2, 20 Oktober 2026. Belum dirilis ke VPS. Lihat
[handover](HANDOVER-CLAUDE.md) dan [hasil tes](testing/plan2-local-2026-10-20.md).

## Konfigurasi

`configs/managed.example.yaml` menunjukkan dua instance yang membagi **satu**
modal menjadi 70 USDT, 25 USDT, dan sisanya cadangan. Split ini demonstrasi,
bukan keputusan mengalokasikan uang ke kedua kandidat.
`configs/managed.eth-paper.yaml` mempertahankan profil ETH-only yang dibekukan:
110,79104808331486 USDT total, risiko 3%, posisi 75%, daily kill 8%, drawdown 30%,
leverage 1. Referensi: [keputusan strategi](strategy-decision-2026-10-20.md).

Struktur `wallets[] → instances[] → strategies[]` menggantikan daftar strategi
legacy untuk proses managed. Jangan mengaktifkan keduanya dalam konfigurasi
yang sama. Versi ini satu Binance Spot USDT wallet per akun/mode. Kredensial
tetap melalui environment, bukan YAML, state atau laporan. Paper tidak perlu key.

Modal awal ditetapkan eksplisit pada wallet. `allocation` instance diambil dari
modal tersebut; sisa menjadi reserve. Strategi berbagi tunai instance, dengan
`cap` opsional. Cap mencakup nilai inventori dan reservasi buy; equity strategi
tidak boleh dijumlahkan seolah-olah masing-masing mempunyai saldo instance.
Risiko yang dihilangkan memakai default; blok nonzero perlu konfigurasi valid
lengkap dengan leverage 1. Parent account/instance tetap membatasi strategi.

Konfigurasi bootstrap diberi hash; perubahan file setelah state dibuat ditolak
agar allocation tidak terulang. Ubah parameter melalui perintah yang dijurnal,
atau gunakan wallet/DB eksperimen baru setelah review. Startup tetap memakai
file bootstrap asli, lalu memuat revisi tersimpan. Run mencatat konfigurasi aktif.

## Kontrol

Dashboard menyediakan buat/clone, rename, preferensi notifikasi, tambah binding,
risiko/cap, transfer dan kontrol per scope. `account` berarti semua instance bot
terdaftar; scope lain adalah ID instance atau `instance/binding`.

| Aksi | Perilaku |
|---|---|
| Pause | Tahan entry baru; exit/proteksi tetap berjalan |
| Resume | Lanjut jika tidak halted/attention dan engine sesuai konfigurasi |
| Cancel | Batalkan entry/raw pending; bukan menjual posisi |
| Flatten | Cancel proteksi lalu jual inventori milik scope; tunggu konfirmasi |
| Finish retain | Cancel seluruh order scope, simpan koin; selesai setelah ack |
| Finish flatten | Tutup inventori scope; selesai setelah ack dan flat |
| Kill/reset | Halt sticky; reset eksplisit tidak menghapus selisih saldo |
| Archive | Posisi/order kosong termasuk dust; tunai instance pindah ke reserve dahulu |
| Stop proses | Persist state untuk resume; order/koin di exchange tetap ada |

Aksi dana memerlukan pilihan konfirmasi dan finish memerlukan retain/flatten.
Status closing tidak berarti order sudah selesai. Buat/clone dimulai paused
tanpa modal/riwayat; transfer saldo bebas terlebih dahulu. Binding baru atau
parameter edit perlu restart agar feed/indikator siap. Batas risiko bisa diubah
langsung; arus modal internal tidak dianggap profit. Dust tidak dihapus otomatis.

Web memakai Basic Auth bila password diaktifkan, CSRF per server, origin check,
CSP dan template escaping. Tetap gunakan bind loopback dan proxy HTTPS sesuai
runbook. JSON `POST /control?wallet=ID` memakai `X-CSRF-Token`, schema `Command`
di `internal/managed/control.go`, dan idempotency `id` unik. GET `/api/state`
mengekspor ledger/run tanpa credentials. Jangan memublikasikannya tanpa auth.

Telegram memakai satu long-poller, whitelist lama, notifikasi info/warn/error/off
per instance dan dedup satu menit. Format: `/pause wallet instance`,
`/flatten wallet instance/binding confirm`,
`/finish wallet instance retain confirm`, `/stop confirm` untuk stop proses.
Daily summary tetap 00.00 UTC/07.00 WIB; bukti penerimaan VPS masih diperlukan.

## Backtest tanpa koneksi database/exchange

```sh
make build
mkdir -p var/managed-backtest
TB_DATABASE__URL=postgres://unused@localhost/unused ./bin/portfolio-backtest \
  -config configs/managed.example.yaml -data /path/data \
  -from 2024-01-01 -to 2024-01-03 -out var/managed-backtest/result.json
```

URL dummy hanya memenuhi validasi config; CLI tidak membuka DB, exchange, `.env`
atau feed live. **Jangan menjalankan `bot run` di laptop untuk mengujinya.**
Data: `SYMBOL-1m.csv` dengan timestamp UTC detik/milidetik/mikrodetik dan kolom
open_time,open,high,low,close,volume; header opsional. `instruments.json` berupa
array `domain.Instrument`, **bukan** respons mentah Binance `exchangeInfo`.
Gunakan aturan tick/lot/minimum yang diketahui pada periode relevan dan catat
bila snapshot instrumen saat ini dipakai untuk harga lama.

CSV digabung kronologis, candle strategi dibangun dari 1m lengkap. Data sebelum
`from` hanya warmup; tidak ada order historis. Gap/duplikat ditolak. Opsi
`-allow-gaps` membolehkan gap secara eksplisit dan membuang agregat tidak lengkap.
Biaya diatur `-fee 0.001 -slippage-bps 2`; ulangi stress, misalnya 0.002/10.
Laporan mencakup hash konfigurasi/data, modal, return, drawdown, turnover, fee,
estimasi biaya exit inventori, tunai, buy-and-hold, dan state per pemilik.
Buy-and-hold adalah pembanding equal-weight penuh, bukan alokasi identik strategi.
Ini model OHLC tanpa antrean order, latency atau market impact, bukan holdout baru.

## Migrasi, ownership dan recovery

Schema perlu `bot migrate up` (migrasi 4), **pada DB salinan dahulu**. Wallet baru
menyimpan row unik exchange/mode/Spot dan advisory lease dedicated connection;
pool perlu setidaknya jumlah wallet + 2 koneksi. Kehilangan lease menahan I/O.
Jangan paralelkan dengan binary lama karena binary tersebut tidak mengenal lock.

Tidak ada auto-import saldo legacy: `managed_legacy_imports` berstatus `review`.
Untuk trial baru gunakan DB baru dengan modal virtual eksplisit; history lama
tetap utuh. Adopsi inventori riil memerlukan mapping/review terpisah. Jangan
menjalankan wallet baru untuk “mencoba” pada akun lama yang masih punya order.

Jika DB/network putus, hentikan entry dan simpan bukti. Restart memulihkan
checkpoint, inbox, reservasi dan order, kemudian membandingkan wallet exchange.
Order unknown/cancel ambigu, transaksi manual, deposit/withdrawal atau selisih
aset menjadi attention. Jangan reset/mengubah saldo agar entry terbuka paksa.
Trade history yang hilang harus dipulihkan dengan ID/komisi aktual. Fee aset
ketiga tanpa harga menahan entry sampai valuasi tersedia.

Stop proses mempertahankan order. Untuk menutup eksperimen, gunakan finish
terlebih dahulu dan verifikasi status order exchange terminal serta saldo.
Snapshot DB/jurnal, hasil query exchange, konfigurasi/hash, waktu dan pilihan
retain/flatten menjadi bukti akhir. Bukti historis tidak boleh ditimpa.

Rollback deployment: stop worker baru, backup state/schema, pastikan tidak ada
order managed yang tertinggal sebelum mengaktifkan binary legacy. **Jangan down
migration 4 pada DB berisi dana/history managed**; tes down/up hanya di DB test
disposable. Binary lama tidak memahami ownership instance baru.

## Preview dashboard fixture

```sh
TB_DASHBOARD_PREVIEW=127.0.0.1:18082 go test ./internal/managed \
  -run '^TestDashboardPreview$' -v -timeout 10m
```

Hanya fixture memori, tanpa `Runtime.Start`, feed, DB, `.env` atau exchange.
Tes ini sengaja blocking untuk preview; hentikan setelah QA. Auth/CSRF tetap
diuji di suite biasa. Sesi Codex belum mempunyai browser untuk visual QA.
