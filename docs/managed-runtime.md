# Runtime managed

Panduan konfigurasi dan kontrol multi-instance Binance Spot. Status deployment
di [handover VPS](handover-vps.md), desain di [multi-instance](multi-instance.md).
Trial ETH sudah dihentikan; contoh perintah di sini bukan izin memulainya lagi.

## Konfigurasi dan modal

`wallets[] → instances[] → strategies[]` menggantikan daftar legacy; jangan
mengaktifkan keduanya bersamaan. Satu wallet Spot USDT per akun/mode. Secret
lewat environment, bukan YAML/state/laporan; paper tidak perlu key trading.

- `configs/managed.example.yaml`: demo satu modal dibagi 70/25 USDT dan cadangan,
  bukan keputusan alokasi strategi.
- `configs/managed.eth-paper.yaml`: profil historis ETH-only 110,79104808331486
  USDT total, risk 3%, posisi 75%, daily kill 8%, drawdown 30%, leverage 1.
  Parameter lengkap di [Strategi](strategy.md); arah terbaru di [Strategi 2](strategy-2.md).
- `allocation` instance berasal modal wallet. Binding berbagi cash; `cap`
  opsional mencakup inventori dan reserved buy, tanpa saldo baru.
- Default risk berlaku jika blok dihilangkan; blok nonzero harus valid lengkap.
  Batas parent tetap berlaku. Transfer/setoran bukan profit.
- Bootstrap config diberi hash. Jangan edit file lama lalu resume untuk mengganti
  eksperimen; gunakan kontrol terjurnal atau DB/wallet baru yang direview.

## Kontrol

Scope `account` berarti semua instance terdaftar; scope lain ID instance atau
`instance/binding`. Dashboard mendukung create/clone, rename, binding, cap/risk,
transfer, lifecycle dan preferensi notifikasi.

| Aksi | Perilaku |
|---|---|
| Pause | Tahan entry; exit/proteksi lanjut |
| Resume | Hanya jika state sehat dan engine cocok dengan konfigurasi |
| Cancel | Batalkan entry/raw pending, tanpa menjual posisi |
| Flatten | Cancel proteksi, jual inventori scope; tunggu konfirmasi |
| Finish retain / flatten | Selesaikan order; simpan / tutup inventori |
| Kill/reset | Halt sticky; reset tidak menghapus selisih saldo |
| Archive | Order/posisi termasuk dust kosong, cash sudah ke reserve |
| Stop proses | Persist untuk resume; order/koin tetap dapat ada |

Aksi dana/finish memerlukan konfirmasi; closing belum berarti selesai.
Create/clone mulai paused tanpa modal/riwayat. Binding baru/edit memerlukan
restart agar feed/indikator siap; risk/cap bisa berubah langsung. Transfer hanya
cash bebas. Tidak ada penghapusan dust atau koreksi saldo paksa.

Web memakai Basic Auth, CSRF, origin check, CSP dan escaping. Bind loopback;
gunakan HTTPS bila diproxy. JSON `POST /control?wallet=ID` memakai
`X-CSRF-Token`, schema `Command` di `internal/managed/control.go`, dan ID unik.
`GET /api/state` berisi ledger/run tanpa credentials, tetap harus dilindungi.

Telegram memakai satu long-poller, whitelist, notifikasi info/warn/error/off per
instance, dedup satu menit dan menu `/`. Contoh format:

```text
/pause wallet instance
/flatten wallet instance/binding confirm
/finish wallet instance retain confirm
/resolve wallet order confirm
/stop confirm
```

Digest dijadwalkan 00.00 UTC/07.00 WIB; jadwal bukan bukti pesan diterima.
Mark ledger candle berjalan dibatasi sekali/10 detik/simbol; candle tutup,
fill/order/kontrol langsung dipersist. Engine memakai harga terbaru di memori.

## Backtest dan preview

Jalankan hanya saat pekerjaan pengujian telah diminta. CLI `portfolio-backtest`
tidak membuka DB/exchange/`.env`; URL dummy hanya memenuhi validasi config:

```sh
TB_DATABASE__URL=postgres://unused@localhost/unused ./bin/portfolio-backtest -config configs/managed.example.yaml -data /path/data -from 2024-01-01 -to 2024-01-03 -out var/managed-backtest/result.json
```

Data `SYMBOL-1m.csv`: timestamp UTC detik/ms/µs, OHLCV, header opsional.
`instruments.json` adalah array `domain.Instrument`, bukan raw `exchangeInfo`.
CSV digabung kronologis; data sebelum `from` hanya warmup. Gap/duplikat ditolak;
`-allow-gaps` membuang agregat tidak lengkap. Catat jika rules instrumen sekarang
menggantikan snapshot historis.

Baseline `-fee 0.001 -slippage-bps 2`; stress misalnya 0.002/10. Laporan berisi
hash data/config, satu modal total, return/drawdown, turnover/fee, estimasi exit,
benchmark tunai/buy-and-hold dan ownership. Buy-and-hold equal-weight penuh
berbeda exposure dari strategi. Model OHLC tanpa queue/latensi/market impact
bukan holdout baru atau bukti profit live.

Preview `TB_DASHBOARD_PREVIEW=127.0.0.1:18082 go test ./internal/managed -run '^TestDashboardPreview$' -v -timeout 10m`
hanya fixture memori, tanpa Runtime/feed/DB/`.env`; hentikan setelah QA.
Jangan memakai `bot run` di laptop. [Hasil QA historis](testing/plan-2-browser-qa.md).

## Migrasi dan recovery

Migrasi 4/5 diuji pada DB salinan dahulu. Wallet memakai row unik dan advisory
lease dedicated connection; pool minimal jumlah wallet + 2. Kehilangan lease
menahan I/O. Binary legacy lama tidak mengenal lock: jangan paralelkan worker.
Legacy import tetap `review`, tanpa adopsi saldo/inventori otomatis.

Putus DB/network atau mismatch aset menahan entry. Restart memulihkan checkpoint,
inbox dan reservasi, lalu reconciliation. Jangan reset attention untuk memaksa
entry; pulihkan trade ID/komisi asli. Fee aset ketiga menahan entry sampai harga
valuasinya tersedia. Transaksi manual/deposit/withdrawal harus ditelusuri.

**Order tak pasti:** setelah restart dan minimal lima menit sejak order dibuat,
pilih `resolve_order` dengan konfirmasi/ID unik. Exchange `-2013` melepas
reservasi dan memberi tahu engine; order open dilacak kembali; status terminal
memulihkan fill dari `myTrades`. Query gagal tetap unknown. Attention dihitung
ulang dari reconciliation, bukan dihapus langsung.

**Kompaksi:** tiap jam, order terminal/eksekusi/audit lebih dari tujuh hari
berpindah ke `managed_archive` secara atomik. Order belum selesai, saudara OCO
terbuka, inbox dan fee pending tetap di state. Agregat finansial dipertahankan;
unique journal request ID menolak replay lama. Dashboard/API hanya tujuh hari
terakhir; history lengkap di archive/journal. Grafik ringkas maksimal 720 jam.

**Penutupan:** finish dengan pilihan retain/flatten, verifikasi terminal dan
saldo, simpan DB/jurnal/config hash/waktu serta bukti exchange, baru tutup proses.
Stop proses tidak membatalkan order. Bukti historis jangan ditimpa.

**Rollback:** stop worker, backup schema/state, selesaikan order managed dan
periksa kompatibilitas binary. Jangan down migrasi 4/5 pada DB berisi dana/
history; down/up hanya DB test disposable.
